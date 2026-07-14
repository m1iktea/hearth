import { defineStore } from 'pinia'
import { apiGet } from '../api/client'
import type { Device, Event } from '../types'
import type { HealthCheckRow } from '../utils/overview'

interface Slice<T> {
  data: T
  loading: boolean
  loaded: boolean
  error: string
}

function slice<T>(data: T): Slice<T> {
  return { data, loading: false, loaded: false, error: '' }
}

/**
 * 仅服务于仪表盘的 health/events/devices 数据。
 * 与 inventory store 分离：每个数据块独立记录错误，
 * 任一接口失败不影响其他模块展示（PRD 局部错误要求）。
 */
export const useOverviewStore = defineStore('overview', {
  state: () => ({
    health: slice<HealthCheckRow[]>([]),
    events: slice<Event[]>([]),
    devices: slice<Device[]>([]),
    timer: 0 as number,
  }),
  actions: {
    async loadHealth() {
      await this.loadSlice(this.health, '/api/v1/health')
    },
    async loadEvents() {
      await this.loadSlice(this.events, '/api/v1/events?limit=20')
    },
    async loadDevices() {
      await this.loadSlice(this.devices, '/api/v1/devices')
    },
    async loadSlice<T>(target: Slice<T>, path: string) {
      target.loading = true
      try {
        target.data = await apiGet<T>(path)
        target.error = ''
        target.loaded = true
      } catch (e) {
        target.error = e instanceof Error ? e.message : String(e)
      } finally {
        target.loading = false
      }
    },
    refreshAll() {
      void this.loadHealth()
      void this.loadEvents()
      void this.loadDevices()
    },
    startPolling(intervalMs = 30_000) {
      this.stopPolling()
      this.refreshAll()
      this.timer = window.setInterval(() => this.refreshAll(), intervalMs)
    },
    stopPolling() {
      if (this.timer) {
        window.clearInterval(this.timer)
        this.timer = 0
      }
    },
  },
})
