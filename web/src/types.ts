export interface Snapshot {
  source: 'proxmox' | 'docker' | 'openwrt' | string
  status: 'online' | 'offline'
  collected_at: string
  last_error?: string
  data?: ProxmoxData | DockerData | OpenWrtData
}

export interface ProxmoxData {
  nodes: {
    name: string
    status: string
    cpu: number
    mem: number
    maxmem: number
    uptime: number
    vms: {
      vmid: number
      name: string
      status: string
      cpu: number
      mem: number
      maxmem: number
      uptime: number
    }[]
  }[]
}

export interface DockerData {
  containers: {
    id: string
    name: string
    image: string
    state: string
    status: string
    cpu_pct?: number | null
    mem_used?: number
    mem_limit?: number
  }[]
}

export interface OpenWrtData {
  hostname: string
  model: string
  release: string
  uptime_sec: number
  load: [number, number, number]
  memory: { total: number; free: number; available: number }
  interfaces: {
    name: string
    up: boolean
    device: string
    ipv4: string
    rx_bytes: number
    tx_bytes: number
  }[]
}

export interface NavItem {
  id: number
  category_id: number
  name: string
  url: string
  icon: string
  sort_order: number
  device_id?: number
}

export interface NavCategory {
  id: number
  name: string
  sort_order: number
  items: NavItem[]
}

export interface Device {
  id: number
  name: string
  kind: string
  hostname: string
  ip_address: string
  mac_address: string
  location: string
  notes: string
  url: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface HealthCheck {
  id: number
  device_id: number
  name: string
  type: 'ping' | 'tcp' | 'http'
  target: string
  port: number
  expected_status: number
  enabled: boolean
  last_status: 'unknown' | 'online' | 'offline'
  last_error: string
  latency_ms: number
  checked_at?: string
}
export interface DeviceDetail { device: Device; checks: HealthCheck[]; nav_item?: NavItem }
export interface Event { id: number; device_id: number; device_name: string; check_id: number; type: string; severity: 'info' | 'warning'; title: string; message: string; created_at: string }
export interface DiscoveryResult { devices: { device: Device; is_new: boolean; vendor: string }[]; new_count: number; updated_count: number }
