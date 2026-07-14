import { describe, expect, it } from 'vitest'
import type { Device, Event, Snapshot } from '../types'
import {
  buildIssues,
  buildRisks,
  humanizeError,
  resolveGlobalState,
  summarize,
  type HealthCheckRow,
} from './overview'

const NOW = new Date('2026-07-14T12:00:00Z').getTime()

function snapshot(overrides: Partial<Snapshot> & Pick<Snapshot, 'source'>): Snapshot {
  return {
    status: 'online',
    collected_at: '2026-07-14T11:59:55Z',
    ...overrides,
  }
}

function check(overrides: Partial<HealthCheckRow>): HealthCheckRow {
  return {
    id: 1,
    device_id: 3,
    name: 'https',
    type: 'http',
    target: 'https://nas.local',
    port: 0,
    expected_status: 200,
    enabled: true,
    last_status: 'online',
    last_error: '',
    latency_ms: 12,
    checked_at: '2026-07-14T11:59:50Z',
    device_name: '飞牛 NAS',
    device_ip: '192.168.1.10',
    ...overrides,
  }
}

function device(overrides: Partial<Device>): Device {
  return {
    id: 3,
    name: '飞牛 NAS',
    kind: 'nas',
    hostname: 'nas',
    ip_address: '192.168.1.10',
    mac_address: '',
    location: '',
    notes: '',
    url: 'https://nas.example.local',
    enabled: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

const dockerSnapshot = snapshot({
  source: 'docker',
  data: {
    containers: [
      { id: 'a', name: 'immich', image: 'immich', state: 'exited', status: 'Exited (1) 8 minutes ago' },
      { id: 'b', name: 'gitea', image: 'gitea', state: 'running', status: 'Up 2 hours' },
    ],
  },
})

describe('buildIssues', () => {
  it('聚合数据源离线、检查失败与容器退出，并按严重程度排序', () => {
    const snapshots = [
      snapshot({ source: 'proxmox', status: 'offline', last_error: 'context deadline exceeded' }),
      dockerSnapshot,
    ]
    const health = [check({ last_status: 'offline', last_error: 'HTTP status 502' })]
    const events: Event[] = [
      {
        id: 9, device_id: 3, device_name: '飞牛 NAS', check_id: 1, type: 'offline',
        severity: 'warning', title: '设备离线', message: '', created_at: '2026-07-14T11:48:00Z',
      },
    ]
    const issues = buildIssues(snapshots, health, [device({})], events)

    expect(issues.map((i) => i.id)).toEqual(['check-1', 'source-proxmox', 'container-immich'])
    const checkIssue = issues[0]
    expect(checkIssue.severity).toBe('critical')
    expect(checkIssue.title).toBe('飞牛 NAS · HTTP 检查')
    expect(checkIssue.message).toBe('HTTP 返回 502')
    expect(checkIssue.since).toBe('2026-07-14T11:48:00Z')
    expect(checkIssue.detailPath).toBe('/devices/3')
    expect(checkIssue.managementUrl).toBe('https://nas.example.local')
    expect(issues[1].message).toBe('请求超时')
    expect(issues[2].severity).toBe('warning')
    expect(issues[2].detailPath).toBe('/nodes')
  })

  it('同级严重程度按持续时间降序（since 更早排前）', () => {
    const health = [
      check({ id: 1, last_status: 'offline' }),
      check({ id: 2, name: 'ping', type: 'ping', last_status: 'offline' }),
    ]
    const events: Event[] = [
      { id: 1, device_id: 3, device_name: '', check_id: 1, type: 'offline', severity: 'warning', title: '', message: '', created_at: '2026-07-14T11:50:00Z' },
      { id: 2, device_id: 3, device_name: '', check_id: 2, type: 'offline', severity: 'warning', title: '', message: '', created_at: '2026-07-14T11:30:00Z' },
    ]
    const issues = buildIssues([], health, [device({})], events)
    expect(issues.map((i) => i.id)).toEqual(['check-2', 'check-1'])
  })

  it('忽略已停用的检查，网口 down 为提醒级', () => {
    const wrt = snapshot({
      source: 'openwrt',
      data: {
        hostname: 'router', model: '', release: '', uptime_sec: 100,
        load: [0.1, 0.1, 0.1],
        memory: { total: 1000, free: 800, available: 800 },
        interfaces: [{ name: 'wan6', up: false, device: 'eth1', ipv4: '', rx_bytes: 0, tx_bytes: 0 }],
      },
    })
    const health = [check({ last_status: 'offline', enabled: false })]
    const issues = buildIssues([wrt], health, [], [])
    expect(issues).toHaveLength(1)
    expect(issues[0].id).toBe('iface-wan6')
    expect(issues[0].severity).toBe('info')
  })
})

describe('buildRisks', () => {
  it('按阈值识别 PVE CPU/内存与路由器可用内存风险，降序排列', () => {
    const snapshots = [
      snapshot({
        source: 'proxmox',
        data: {
          nodes: [
            { name: 'pve-01', status: 'online', cpu: 0.9, mem: 890, maxmem: 1000, uptime: 1, vms: [] },
            { name: 'pve-02', status: 'online', cpu: 0.2, mem: 100, maxmem: 1000, uptime: 1, vms: [] },
          ],
        },
      }),
      snapshot({
        source: 'openwrt',
        data: {
          hostname: 'router', model: '', release: '', uptime_sec: 1,
          load: [0, 0, 0],
          memory: { total: 1000, free: 100, available: 100 },
          interfaces: [],
        },
      }),
    ]
    const risks = buildRisks(snapshots)
    expect(risks.map((r) => r.id)).toEqual(['pve-pve-01-cpu', 'wrt-mem', 'pve-pve-01-mem'])
    expect(risks[0].value).toBe(90)
    expect(risks.find((r) => r.id === 'wrt-mem')?.value).toBe(90)
  })

  it('无风险时返回空数组', () => {
    expect(buildRisks([dockerSnapshot])).toEqual([])
  })
})

describe('summarize', () => {
  it('需处理 = 离线数据源 + 离线检查 + 已停止容器', () => {
    const snapshots = [
      snapshot({ source: 'proxmox', status: 'offline', collected_at: '2026-07-14T11:58:00Z' }),
      dockerSnapshot,
    ]
    const health = [
      check({ id: 1, last_status: 'offline' }),
      check({ id: 2, last_status: 'online' }),
      check({ id: 3, last_status: 'offline', enabled: false }),
    ]
    const devices = [device({ id: 1 }), device({ id: 2, enabled: false })]
    const s = summarize(snapshots, health, devices)
    expect(s.issueCount).toBe(3)
    expect(s.offlineSources).toBe(1)
    expect(s.totalSources).toBe(2)
    expect(s.offlineChecks).toBe(1)
    expect(s.enabledChecks).toBe(2)
    expect(s.managedDevices).toBe(1)
    expect(s.updatedAt).toBe('2026-07-14T11:59:55Z')
  })
})

describe('resolveGlobalState', () => {
  it('无数据为 pending，有异常为 issues，超过两个轮询周期为 stale，否则 ok', () => {
    const base = summarize([], [], [])
    expect(resolveGlobalState(base, NOW, 10_000)).toBe('pending')

    const fresh = summarize([snapshot({ source: 'proxmox' })], [], [])
    expect(resolveGlobalState(fresh, NOW, 10_000)).toBe('ok')

    const withIssue = summarize([snapshot({ source: 'proxmox', status: 'offline' })], [], [])
    expect(resolveGlobalState(withIssue, NOW, 10_000)).toBe('issues')

    const stale = summarize(
      [snapshot({ source: 'proxmox', collected_at: '2026-07-14T11:58:00Z' })],
      [], [],
    )
    expect(resolveGlobalState(stale, NOW, 10_000)).toBe('stale')
  })
})

describe('humanizeError', () => {
  it('压缩常见错误为可读摘要', () => {
    expect(humanizeError('dial tcp: connection refused')).toBe('连接被拒绝')
    expect(humanizeError('context deadline exceeded')).toBe('请求超时')
    expect(humanizeError('unexpected HTTP status 502')).toBe('HTTP 返回 502')
    expect(humanizeError('')).toBe('')
    expect(humanizeError('x'.repeat(100))).toHaveLength(61)
  })
})
