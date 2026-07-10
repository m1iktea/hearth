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
}

export interface NavCategory {
  id: number
  name: string
  sort_order: number
  items: NavItem[]
}
