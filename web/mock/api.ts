import type { Plugin } from 'vite'

/**
 * 本地演示用 mock 接口：`npm run dev:mock` 启用。
 * 覆盖仪表盘所需的全部 GET 接口；时间戳按请求时刻动态生成，
 * 相对时间展示（“已持续 N 分钟”）不会因数据固定而失真。
 *
 * 演示场景：
 * - 严重：飞牛 NAS 的 HTTP 检查失败（502，已持续 12 分钟）
 * - 注意：immich 容器已退出；PVE 节点内存 89% 资源风险
 * - 提醒：路由器 wan6 / lan4 接口 down（共 5 项，可演示“查看全部”展开）
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
          { id: 'a2', name: 'gitea', image: 'gitea/gitea:latest', state: 'running', status: 'Up 5 days' },
          { id: 'a3', name: 'vaultwarden', image: 'vaultwarden/server', state: 'running', status: 'Up 5 days' },
          { id: 'a4', name: 'jellyfin', image: 'jellyfin/jellyfin', state: 'running', status: 'Up 2 days' },
          { id: 'a5', name: 'qbittorrent', image: 'linuxserver/qbittorrent', state: 'running', status: 'Up 12 hours' },
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

function nav() {
  return [
    {
      id: 1, name: '常用服务', sort_order: 1,
      items: [
        { id: 1, category_id: 1, name: 'Jellyfin', url: 'http://192.168.31.10:8096', icon: '', sort_order: 1 },
        { id: 2, category_id: 1, name: 'Gitea', url: 'http://192.168.31.10:3000', icon: '', sort_order: 2 },
      ],
    },
  ]
}

function deviceDetail(id: number) {
  const device = devices().find((d) => d.id === id)
  if (!device) return undefined
  const checks = health()
    .filter((c) => c.device_id === id)
    .map(({ device_name: _n, device_ip: _i, ...check }) => check)
  return { device, checks }
}

function resolveMock(path: string): unknown {
  if (path === '/api/v1/status') return status()
  if (path === '/api/v1/health') return health()
  if (path === '/api/v1/events') return events()
  if (path === '/api/v1/devices') return devices()
  if (path === '/api/v1/nav') return nav()
  const detail = path.match(/^\/api\/v1\/devices\/(\d+)$/)
  if (detail) return deviceDetail(Number(detail[1]))
  return undefined
}

export function mockApi(): Plugin {
  return {
    name: 'hearth-mock-api',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if ((req.method ?? 'GET') !== 'GET') return next()
        const path = new URL(req.url ?? '', 'http://localhost').pathname
        const data = resolveMock(path)
        if (data === undefined) return next()
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify({ success: true, data }))
      })
    },
  }
}
