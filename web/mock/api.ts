import type { Plugin } from 'vite'

/**
 * 本地演示用 mock 接口：`npm run dev:mock` 启用。
 * 覆盖仪表盘所需的全部 GET 接口；时间戳按请求时刻动态生成，
 * 相对时间展示（”已持续 N 分钟”）不会因数据固定而失真。
 *
 * 演示场景：
 * - 严重：飞牛 NAS 的 HTTP 检查失败（502，已持续 12 分钟）
 * - 注意：immich 容器已退出；PVE 节点内存 89% 资源风险
 * - 提醒：路由器 wan6 / lan4 接口 down（共 5 项，可演示”查看全部”展开）
 * - metrics 时序：proxmox/docker/openwrt 各对象 cpu_pct/mem_pct/temp_c 可过滤
 * - nav 关联设备：设备在线/离线角标演示
 */

const minutesAgo = (min: number) => new Date(Date.now() - min * 60_000).toISOString()
const secondsAgo = (sec: number) => new Date(Date.now() - sec * 1_000).toISOString()

const GiB = 1024 ** 3

function status() {
  return [
    {
      source: 'proxmox',
      status: 'online',
      collected_at: secondsAgo(6),
      data: {
        nodes: [
          {
            name: 'pve-01',
            status: 'online',
            cpu: 0.23,
            mem: Math.round(13.7 * GiB),
            maxmem: Math.round(15.4 * GiB),
            uptime: 86400 * 12 + 3600 * 5,
            vms: [
              { vmid: 100, name: 'homeassistant', status: 'running', cpu: 0.05, mem: 2 * GiB, maxmem: 4 * GiB, uptime: 86400 * 12 },
              { vmid: 101, name: 'fnos', status: 'running', cpu: 0.12, mem: 6 * GiB, maxmem: 8 * GiB, uptime: 86400 * 12 },
              { vmid: 102, name: 'openwrt', status: 'running', cpu: 0.03, mem: 0.5 * GiB, maxmem: 1 * GiB, uptime: 86400 * 12 },
              { vmid: 103, name: 'ubuntu-dev', status: 'stopped', cpu: 0, mem: 0, maxmem: 4 * GiB, uptime: 0 },
              { vmid: 104, name: 'win11', status: 'stopped', cpu: 0, mem: 0, maxmem: 8 * GiB, uptime: 0 },
              { vmid: 105, name: 'k3s-node', status: 'running', cpu: 0.08, mem: 3 * GiB, maxmem: 4 * GiB, uptime: 86400 * 3 },
            ],
          },
        ],
      },
    },
    {
      source: 'docker',
      status: 'online',
      collected_at: secondsAgo(8),
      data: {
        containers: [
          { id: 'a1', name: 'immich', image: 'ghcr.io/immich-app/immich-server', state: 'exited', status: 'Exited (1) 8 minutes ago' },
          { id: 'a2', name: 'gitea', image: 'gitea/gitea:latest', state: 'running', status: 'Up 5 days', cpu_pct: null, mem_used: 512 * 1024 * 1024, mem_limit: 2 * GiB },
          { id: 'a3', name: 'vaultwarden', image: 'vaultwarden/server', state: 'running', status: 'Up 5 days', cpu_pct: 2.3, mem_used: Math.round(180 * 1024 * 1024), mem_limit: 1 * GiB },
          { id: 'a4', name: 'jellyfin', image: 'jellyfin/jellyfin', state: 'running', status: 'Up 2 days', cpu_pct: 8.7, mem_used: Math.round(800 * 1024 * 1024), mem_limit: 2 * GiB },
          { id: 'a5', name: 'qbittorrent', image: 'linuxserver/qbittorrent', state: 'running', status: 'Up 12 hours', cpu_pct: 1.2, mem_used: Math.round(350 * 1024 * 1024), mem_limit: 1 * GiB },
          { id: 'a6', name: 'nginx-proxy', image: 'nginx:alpine', state: 'running', status: 'Up 5 days' },
        ],
      },
    },
    {
      source: 'openwrt',
      status: 'online',
      collected_at: secondsAgo(4),
      data: {
        hostname: 'ImmortalWrt',
        model: 'x86_64',
        release: 'ImmortalWrt 23.05.4',
        uptime_sec: 86400 * 45 + 3600 * 2,
        load: [0.35, 0.42, 0.38],
        memory: { total: 4 * GiB, free: 1.2 * GiB, available: 2.1 * GiB },
        interfaces: [
          { name: 'wan', up: true, device: 'eth0', ipv4: '100.64.12.34', rx_bytes: 12_345_678_901, tx_bytes: 2_345_678_901 },
          { name: 'lan', up: true, device: 'br-lan', ipv4: '192.168.31.1', rx_bytes: 9_876_543_210, tx_bytes: 8_765_432_109 },
          { name: 'wan6', up: false, device: 'eth0', ipv4: '', rx_bytes: 0, tx_bytes: 0 },
          { name: 'lan4', up: false, device: 'eth4', ipv4: '', rx_bytes: 0, tx_bytes: 0 },
        ],
      },
    },
  ]
}

const NAMED_DEVICES = [
  { id: 3, name: '飞牛 NAS', kind: 'nas', hostname: 'fnos', ip_address: '192.168.31.10', url: 'https://192.168.31.10:5666' },
  { id: 4, name: 'PVE 主机', kind: 'server', hostname: 'pve-01', ip_address: '192.168.31.2', url: 'https://192.168.31.2:8006' },
  { id: 5, name: '打印机', kind: 'printer', hostname: '', ip_address: '192.168.31.20', url: '' },
  { id: 6, name: 'ImmortalWrt 路由器', kind: 'router', hostname: 'ImmortalWrt', ip_address: '192.168.31.1', url: 'http://192.168.31.1' },
  { id: 7, name: '树莓派', kind: 'sbc', hostname: 'raspberrypi', ip_address: '192.168.31.30', url: '' },
]

function devices() {
  const named = NAMED_DEVICES.map((d) => ({
    mac_address: '', location: '书房', notes: '', enabled: true,
    created_at: minutesAgo(60 * 24 * 30), updated_at: minutesAgo(60), ...d,
  }))
  const filler = Array.from({ length: 19 }, (_, i) => ({
    id: 100 + i,
    name: `局域网设备-${String(i + 1).padStart(2, '0')}`,
    kind: 'discovered',
    hostname: '',
    ip_address: `192.168.31.${100 + i}`,
    mac_address: `aa:bb:cc:dd:ee:${(16 + i).toString(16)}`,
    location: '',
    notes: '',
    url: '',
    enabled: true,
    created_at: minutesAgo(60 * 24 * 7),
    updated_at: minutesAgo(60 * 24),
  }))
  const disabled = {
    id: 200, name: '旧笔记本', kind: 'other', hostname: '', ip_address: '192.168.31.200',
    mac_address: '', location: '', notes: '已闲置', url: '', enabled: false,
    created_at: minutesAgo(60 * 24 * 90), updated_at: minutesAgo(60 * 24 * 30),
  }
  return [...named, ...filler, disabled]
}

function health() {
  return [
    {
      id: 18, device_id: 3, name: 'HTTPS 面板', type: 'http', target: 'https://192.168.31.10:5666', port: 0,
      expected_status: 200, enabled: true, last_status: 'offline',
      last_error: 'unexpected HTTP status 502', latency_ms: 0, checked_at: secondsAgo(15),
      device_name: '飞牛 NAS', device_ip: '192.168.31.10',
    },
    {
      id: 19, device_id: 3, name: 'Ping', type: 'ping', target: '192.168.31.10', port: 0,
      expected_status: 0, enabled: true, last_status: 'online',
      last_error: '', latency_ms: 2, checked_at: secondsAgo(15),
      device_name: '飞牛 NAS', device_ip: '192.168.31.10',
    },
    {
      id: 20, device_id: 7, name: 'SSH', type: 'tcp', target: '192.168.31.30', port: 22,
      expected_status: 0, enabled: true, last_status: 'online',
      last_error: '', latency_ms: 4, checked_at: secondsAgo(20),
      device_name: '树莓派', device_ip: '192.168.31.30',
    },
    {
      id: 21, device_id: 5, name: 'Ping', type: 'ping', target: '192.168.31.20', port: 0,
      expected_status: 0, enabled: true, last_status: 'online',
      last_error: '', latency_ms: 8, checked_at: secondsAgo(25),
      device_name: '打印机', device_ip: '192.168.31.20',
    },
    {
      id: 22, device_id: 6, name: 'Web 面板', type: 'http', target: 'http://192.168.31.1', port: 0,
      expected_status: 200, enabled: false, last_status: 'offline',
      last_error: 'context deadline exceeded', latency_ms: 0, checked_at: minutesAgo(60 * 24),
      device_name: 'ImmortalWrt 路由器', device_ip: '192.168.31.1',
    },
  ]
}

function events() {
  return [
    {
      id: 45, device_id: 3, device_name: '飞牛 NAS', check_id: 18, type: 'offline', severity: 'warning',
      title: '设备离线', message: 'HTTP 检查失败：unexpected HTTP status 502', created_at: minutesAgo(12),
    },
    {
      id: -1, device_id: 0, device_name: 'pve-01', check_id: 0, type: 'reboot', severity: 'warning',
      title: '节点重启', message: 'pve-01 检测到重启：运行时长从 12d5h 回落到 3m', created_at: minutesAgo(35),
    },
    {
      id: 44, device_id: 7, device_name: '树莓派', check_id: 20, type: 'online', severity: 'info',
      title: '设备恢复', message: 'TCP 检查恢复正常', created_at: minutesAgo(65),
    },
    {
      id: 43, device_id: 7, device_name: '树莓派', check_id: 20, type: 'offline', severity: 'warning',
      title: '设备离线', message: 'dial tcp: connection refused', created_at: minutesAgo(78),
    },
    {
      id: 42, device_id: 5, device_name: '打印机', check_id: 21, type: 'online', severity: 'info',
      title: '设备恢复', message: 'Ping 恢复正常', created_at: minutesAgo(60 * 26),
    },
  ]
}

/**
 * nav items 内存状态，支持 PUT /api/v1/nav/items/{id} 演示绑定/解绑设备。
 * 使用数字 device_id：
 *   - Jellyfin → device 3（飞牛 NAS），health offline → 角标显示离线
 *   - Gitea    → device 7（树莓派），health online → 角标显示在线
 *   - Gitea 下新增 ImmortalWrt → device 6（路由器），health offline → 角标显示离线
 */
const navState: {
  categories: Array<{
    id: number
    name: string
    sort_order: number
    items: Array<{
      id: number
      category_id: number
      name: string
      url: string
      icon: string
      sort_order: number
      device_id: number | null
    }>
  }>
} = {
  categories: [
    {
      id: 1, name: '常用服务', sort_order: 1,
      items: [
        { id: 1, category_id: 1, name: 'Jellyfin', url: 'http://192.168.31.10:8096', icon: '', sort_order: 1, device_id: 3 },
        { id: 2, category_id: 1, name: 'Gitea', url: 'http://192.168.31.10:3000', icon: '', sort_order: 2, device_id: 7 },
        { id: 3, category_id: 1, name: 'ImmortalWrt', url: 'http://192.168.31.1', icon: '', sort_order: 3, device_id: 6 },
      ],
    },
  ],
}

function nav() {
  return navState.categories
}

function deviceDetail(id: number) {
  const device = devices().find((d) => d.id === id)
  if (!device) return undefined
  const checks = health()
    .filter((c) => c.device_id === id)
    .map(({ device_name: _n, device_ip: _i, ...check }) => check)
  // 若有 nav item 绑定了该设备，附加 nav_item 字段
  const navItem = navState.categories
    .flatMap((cat) => cat.items)
    .find((item) => item.device_id === id) ?? null
  return { device, checks, nav_item: navItem }
}

// ── metrics 时序生成 ─────────────────────────────────────────────────────────

interface MetricSample {
  source: string
  object: string
  metric: string
  value: number
  created_at: string
}

/** 平滑伪随机：base ± amplitude，叠加 sin 波形 + 微扰 */
function smoothValue(base: number, amplitude: number, tSec: number, seed: number): number {
  const slow = Math.sin(tSec / 600 + seed) * amplitude * 0.6
  const fast = Math.sin(tSec / 120 + seed * 2.3) * amplitude * 0.25
  const micro = Math.sin(tSec / 30 + seed * 7.1) * amplitude * 0.15
  return Math.max(0, Math.min(100, base + slow + fast + micro))
}

/** metric 定义：{ source, object, metric, base, amplitude, seed } */
const METRIC_DEFS = [
  // proxmox
  { source: 'proxmox', object: 'pve-01', metric: 'cpu_pct',  base: 35, amplitude: 20, seed: 1.1 },
  { source: 'proxmox', object: 'pve-01', metric: 'mem_pct',  base: 58, amplitude: 12, seed: 2.2 },
  { source: 'proxmox', object: 'pve-02', metric: 'cpu_pct',  base: 22, amplitude: 15, seed: 3.3 },
  { source: 'proxmox', object: 'pve-02', metric: 'mem_pct',  base: 65, amplitude: 10, seed: 4.4 },
  // proxmox 温度（℃，object 为 lm-sensors 芯片名）
  { source: 'proxmox', object: 'coretemp-isa-0000', metric: 'temp_c', base: 52, amplitude: 8, seed: 14.5 },
  { source: 'proxmox', object: 'nvme-pci-0100',      metric: 'temp_c', base: 44, amplitude: 5, seed: 15.6 },
  // docker
  { source: 'docker', object: 'hearth',       metric: 'cpu_pct',  base: 8,  amplitude: 8,  seed: 5.5 },
  { source: 'docker', object: 'hearth',       metric: 'mem_pct',  base: 18, amplitude: 8,  seed: 6.6 },
  { source: 'docker', object: 'gitea',        metric: 'cpu_pct',  base: 12, amplitude: 10, seed: 7.7 },
  { source: 'docker', object: 'gitea',        metric: 'mem_pct',  base: 28, amplitude: 10, seed: 8.8 },
  { source: 'docker', object: 'qbittorrent',  metric: 'cpu_pct',  base: 20, amplitude: 15, seed: 9.9 },
  { source: 'docker', object: 'qbittorrent',  metric: 'mem_pct',  base: 32, amplitude: 12, seed: 10.1 },
  { source: 'docker', object: 'frigate',      metric: 'cpu_pct',  base: 25, amplitude: 18, seed: 11.2 },
  { source: 'docker', object: 'frigate',      metric: 'mem_pct',  base: 38, amplitude: 10, seed: 12.3 },
  // openwrt
  { source: 'openwrt', object: 'immortalwrt', metric: 'mem_used_pct', base: 40, amplitude: 10, seed: 13.4 },
]

/** 生成 since → now 每 60s 一个点的时序，按参数过滤 */
function metrics(params: URLSearchParams): MetricSample[] {
  const pSource = params.get('source')
  const pObject = params.get('object')
  const pMetric = params.get('metric')
  const pSince  = params.get('since')
  const pLimit  = params.get('limit')

  const nowMs   = Date.now()
  const sinceMs = pSince ? new Date(pSince).getTime() : nowMs - 60 * 60 * 1000 // 默认 1h

  const stepMs  = 60_000 // 60s 一个点
  const samples: MetricSample[] = []

  const defs = METRIC_DEFS.filter((d) => {
    if (pSource && d.source !== pSource) return false
    if (pObject && d.object !== pObject) return false
    if (pMetric && d.metric !== pMetric) return false
    return true
  })

  for (const def of defs) {
    let t = sinceMs
    while (t <= nowMs) {
      const tSec = t / 1000
      const value = parseFloat(smoothValue(def.base, def.amplitude, tSec, def.seed).toFixed(1))
      samples.push({
        source: def.source,
        object: def.object,
        metric: def.metric,
        value,
        created_at: new Date(t).toISOString(),
      })
      t += stepMs
    }
  }

  // 按时间升序
  samples.sort((a, b) => a.created_at.localeCompare(b.created_at))

  if (pLimit) {
    const limit = parseInt(pLimit, 10)
    if (!isNaN(limit) && limit > 0) return samples.slice(-limit)
  }
  return samples
}

function resolveMockGet(path: string, searchParams: URLSearchParams): unknown {
  if (path === '/api/v1/capabilities') return { arp_discovery: true }
  if (path === '/api/v1/status') return status()
  if (path === '/api/v1/health') return health()
  if (path === '/api/v1/events') return events()
  if (path === '/api/v1/devices') return devices()
  if (path === '/api/v1/nav') return nav()
  if (path === '/api/v1/metrics') return metrics(searchParams)
  const detail = path.match(/^\/api\/v1\/devices\/(\d+)$/)
  if (detail) return deviceDetail(Number(detail[1]))
  return undefined
}

/** PUT /api/v1/nav/items/{id}：更新内存中的 device_id（null 解绑） */
function handleNavItemPut(path: string, body: unknown): { status: number; data: unknown } {
  const m = path.match(/^\/api\/v1\/nav\/items\/(\d+)$/)
  if (!m) return { status: 404, data: null }
  const id = Number(m[1])
  for (const cat of navState.categories) {
    const item = cat.items.find((it) => it.id === id)
    if (item) {
      const payload = body as Record<string, unknown>
      if ('device_id' in payload) {
        const raw = payload['device_id']
        item.device_id = raw == null ? null : Number(raw)
      }
      // 允许更新其他字段
      for (const key of ['name', 'url', 'icon', 'sort_order'] as const) {
        if (key in payload && payload[key] !== undefined) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          ;(item as any)[key] = payload[key]
        }
      }
      return { status: 200, data: item }
    }
  }
  return { status: 404, data: null }
}

export function mockApi(): Plugin {
  return {
    name: 'hearth-mock-api',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const method = req.method ?? 'GET'
        const url = new URL(req.url ?? '', 'http://localhost')
        const path = url.pathname

        // ── PUT 支持 ──────────────────────────────────────────────
        if (method === 'PUT' && path.startsWith('/api/v1/nav/items/')) {
          let raw = ''
          req.on('data', (chunk: Buffer) => { raw += chunk.toString() })
          req.on('end', () => {
            let body: unknown = {}
            try { body = JSON.parse(raw) } catch { /* empty body */ }
            const { status, data } = handleNavItemPut(path, body)
            res.setHeader('Content-Type', 'application/json')
            res.statusCode = status
            res.end(JSON.stringify({ success: status === 200, data }))
          })
          return
        }

        // ── GET 支持 ──────────────────────────────────────────────
        if (method !== 'GET') return next()
        const data = resolveMockGet(path, url.searchParams)
        if (data === undefined) return next()
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify({ success: true, data }))
      })
    },
  }
}
