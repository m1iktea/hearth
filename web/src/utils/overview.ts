import type {
  Device,
  DockerData,
  Event,
  HealthCheck,
  OpenWrtData,
  ProxmoxData,
  Snapshot,
} from '../types'

export type HealthCheckRow = HealthCheck & { device_name: string; device_ip: string }

/** 严重程度：严重（红）/ 注意（橙）/ 提醒（灰） */
export type IssueSeverity = 'critical' | 'warning' | 'info'

export type GlobalState = 'ok' | 'issues' | 'pending' | 'stale'

export interface Issue {
  id: string
  severity: IssueSeverity
  /** 对象名称 + 对象类型，如“飞牛 NAS · HTTP 检查” */
  title: string
  /** 一句话原因摘要 */
  message: string
  /** 首次失败时间（可推断时），用于“已持续 N 分钟” */
  since?: string
  /** 最后一次采集/检查时间 */
  lastCheckedAt?: string
  detailPath: string
  managementUrl?: string
  /** 可选上下文：检查目标、端口、原始状态等 */
  context?: string
}

export interface ResourceRisk {
  id: string
  /** 对象描述，如“PVE / pve-01” */
  label: string
  metric: string
  /** 百分比 0–100 */
  value: number
  detailPath: string
}

export interface OverviewSummary {
  /** 需处理口径：offline 检查 + offline 数据源 + 已停止容器 */
  issueCount: number
  offlineSources: number
  totalSources: number
  offlineChecks: number
  enabledChecks: number
  managedDevices: number
  stoppedContainers: number
  /** 各数据源 collected_at 中最新者 */
  updatedAt?: string
}

export const CPU_RISK_PCT = 85
export const MEM_RISK_PCT = 85
export const WRT_AVAILABLE_MEM_MIN_PCT = 15

export const SOURCE_LABELS: Record<string, string> = {
  proxmox: 'Proxmox VE',
  docker: 'Docker',
  openwrt: 'ImmortalWrt',
}

const CHECK_TYPE_LABELS: Record<string, string> = {
  ping: 'Ping',
  tcp: 'TCP',
  http: 'HTTP',
}

const SEVERITY_RANK: Record<IssueSeverity, number> = { critical: 0, warning: 1, info: 2 }

/** 将 collector/检查的原始错误压缩为一句可读摘要；原文保留在 context */
export function humanizeError(raw: string | undefined): string {
  if (!raw) return ''
  const s = raw.toLowerCase()
  if (s.includes('timeout') || s.includes('deadline exceeded')) return '请求超时'
  if (s.includes('connection refused')) return '连接被拒绝'
  if (s.includes('no route to host') || s.includes('unreachable')) return '网络不可达'
  if (s.includes('401') || s.includes('403') || s.includes('unauthorized') || s.includes('authentication'))
    return '认证失败'
  if (s.includes('certificate') || s.includes('x509') || s.includes('tls')) return '证书或 TLS 错误'
  if (s.includes('no such host') || s.includes('dns')) return '域名解析失败'
  const httpStatus = raw.match(/(?:status|返回)\s*(?:code\s*)?(\d{3})/i)
  if (httpStatus) return `HTTP 返回 ${httpStatus[1]}`
  return raw.length > 60 ? `${raw.slice(0, 60)}…` : raw
}

function proxmoxData(s: Snapshot | undefined): ProxmoxData | undefined {
  return s?.source === 'proxmox' ? (s.data as ProxmoxData | undefined) : undefined
}

function dockerData(s: Snapshot | undefined): DockerData | undefined {
  return s?.source === 'docker' ? (s.data as DockerData | undefined) : undefined
}

function openwrtData(s: Snapshot | undefined): OpenWrtData | undefined {
  return s?.source === 'openwrt' ? (s.data as OpenWrtData | undefined) : undefined
}

/** 某个健康检查最近一次 offline 事件的时间，作为“已持续”的起点 */
function latestOfflineEventAt(events: Event[], checkId: number): string | undefined {
  const matched = events.filter((e) => e.check_id === checkId && e.type === 'offline')
  if (matched.length === 0) return undefined
  return matched.reduce((latest, e) => (e.created_at > latest ? e.created_at : latest), matched[0].created_at)
}

function sourceIssues(snapshots: Snapshot[]): Issue[] {
  return snapshots
    .filter((s) => s.status === 'offline')
    .map((s) => ({
      id: `source-${s.source}`,
      severity: 'critical' as const,
      title: `${SOURCE_LABELS[s.source] ?? s.source} · 数据源`,
      message: humanizeError(s.last_error) || '采集失败',
      lastCheckedAt: s.collected_at,
      detailPath: '/nodes',
      context: s.last_error && humanizeError(s.last_error) !== s.last_error ? s.last_error : undefined,
    }))
}

function checkIssues(health: HealthCheckRow[], devices: Device[], events: Event[]): Issue[] {
  const deviceById = new Map(devices.map((d) => [d.id, d]))
  return health
    .filter((c) => c.enabled && c.last_status === 'offline')
    .map((c) => {
      const device = deviceById.get(c.device_id)
      const target = c.port > 0 ? `${c.target}:${c.port}` : c.target
      return {
        id: `check-${c.id}`,
        severity: 'critical' as const,
        title: `${c.device_name || device?.name || c.name} · ${CHECK_TYPE_LABELS[c.type] ?? c.type} 检查`,
        message: humanizeError(c.last_error) || '检查失败',
        since: latestOfflineEventAt(events, c.id),
        lastCheckedAt: c.checked_at,
        detailPath: `/devices/${c.device_id}`,
        managementUrl: device?.url || undefined,
        context: `检查目标 ${target}`,
      }
    })
}

function containerIssues(snapshots: Snapshot[]): Issue[] {
  const docker = snapshots.find((s) => s.source === 'docker')
  const data = dockerData(docker)
  if (!data) return []
  return data.containers
    .filter((c) => c.state !== 'running')
    .map((c) => ({
      id: `container-${c.name}`,
      severity: 'warning' as const,
      title: `${c.name} · Docker 容器`,
      message: c.state === 'exited' ? '容器已退出' : `容器状态：${c.state}`,
      lastCheckedAt: docker?.collected_at,
      detailPath: '/nodes',
      context: c.status || undefined,
    }))
}

function riskIssues(risks: ResourceRisk[]): Issue[] {
  return risks.map((r) => ({
    id: `risk-${r.id}`,
    severity: 'warning' as const,
    title: `${r.label} · 资源风险`,
    message: `${r.metric}占用 ${r.value}%`,
    detailPath: r.detailPath,
  }))
}

function interfaceIssues(snapshots: Snapshot[]): Issue[] {
  const wrt = snapshots.find((s) => s.source === 'openwrt')
  const data = openwrtData(wrt)
  if (!data) return []
  return data.interfaces
    .filter((i) => !i.up)
    .map((i) => ({
      id: `iface-${i.name}`,
      severity: 'info' as const,
      title: `${i.name} · 网络接口`,
      message: '接口已断开',
      lastCheckedAt: wrt?.collected_at,
      detailPath: '/nodes',
      context: i.device ? `设备 ${i.device}` : undefined,
    }))
}

/**
 * 汇总“需要处理”列表。
 * 排序：严重程度优先，同级按持续时间降序（since 越早越靠前，无 since 的排后）。
 */
export function buildIssues(
  snapshots: Snapshot[],
  health: HealthCheckRow[],
  devices: Device[],
  events: Event[],
): Issue[] {
  const risks = buildRisks(snapshots)
  const issues = [
    ...sourceIssues(snapshots),
    ...checkIssues(health, devices, events),
    ...containerIssues(snapshots),
    ...riskIssues(risks),
    ...interfaceIssues(snapshots),
  ]
  return issues.sort((a, b) => {
    const bySeverity = SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity]
    if (bySeverity !== 0) return bySeverity
    if (a.since && b.since) return a.since < b.since ? -1 : a.since > b.since ? 1 : 0
    if (a.since) return -1
    if (b.since) return 1
    return 0
  })
}

/** 资源风险：CPU ≥ 85%、内存 ≥ 85%、ImmortalWrt 可用内存 ≤ 15%；按风险值降序 */
export function buildRisks(snapshots: Snapshot[]): ResourceRisk[] {
  const risks: ResourceRisk[] = []
  const pve = proxmoxData(snapshots.find((s) => s.source === 'proxmox'))
  for (const node of pve?.nodes ?? []) {
    const cpuPct = Math.round(node.cpu * 100)
    const memPct = node.maxmem > 0 ? Math.round((node.mem / node.maxmem) * 100) : 0
    if (cpuPct >= CPU_RISK_PCT)
      risks.push({ id: `pve-${node.name}-cpu`, label: `PVE / ${node.name}`, metric: 'CPU', value: cpuPct, detailPath: '/nodes' })
    if (memPct >= MEM_RISK_PCT)
      risks.push({ id: `pve-${node.name}-mem`, label: `PVE / ${node.name}`, metric: '内存', value: memPct, detailPath: '/nodes' })
  }
  const wrt = openwrtData(snapshots.find((s) => s.source === 'openwrt'))
  if (wrt && wrt.memory.total > 0) {
    const availPct = Math.round((wrt.memory.available / wrt.memory.total) * 100)
    if (availPct <= WRT_AVAILABLE_MEM_MIN_PCT)
      risks.push({
        id: 'wrt-mem',
        label: `ImmortalWrt / ${wrt.hostname || '路由器'}`,
        metric: '内存',
        value: 100 - availPct,
        detailPath: '/nodes',
      })
  }
  return risks.sort((a, b) => b.value - a.value)
}

export function summarize(
  snapshots: Snapshot[],
  health: HealthCheckRow[],
  devices: Device[],
): OverviewSummary {
  const offlineSources = snapshots.filter((s) => s.status === 'offline').length
  const enabledChecksList = health.filter((c) => c.enabled)
  const offlineChecks = enabledChecksList.filter((c) => c.last_status === 'offline').length
  const docker = dockerData(snapshots.find((s) => s.source === 'docker'))
  const stoppedContainers = (docker?.containers ?? []).filter((c) => c.state !== 'running').length
  const updatedAt = snapshots.reduce<string | undefined>(
    (latest, s) => (!latest || s.collected_at > latest ? s.collected_at : latest),
    undefined,
  )
  return {
    issueCount: offlineSources + offlineChecks + stoppedContainers,
    offlineSources,
    totalSources: snapshots.length,
    offlineChecks,
    enabledChecks: enabledChecksList.length,
    managedDevices: devices.filter((d) => d.enabled).length,
    stoppedContainers,
    updatedAt,
  }
}

/**
 * 全局状态。数据超过两个轮询周期未更新视为过期；
 * 额外 5 秒余量吸收“后端采集 + 前端轮询”两级延迟的正常抖动。
 */
export function resolveGlobalState(
  summary: OverviewSummary,
  now: number,
  pollIntervalMs: number,
): GlobalState {
  if (!summary.updatedAt) return 'pending'
  if (summary.issueCount > 0) return 'issues'
  const age = now - new Date(summary.updatedAt).getTime()
  if (age > pollIntervalMs * 2 + 5_000) return 'stale'
  return 'ok'
}
