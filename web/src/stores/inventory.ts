import { defineStore } from 'pinia'
import { apiDelete, apiGet, apiPost, apiPut } from '../api/client'
import type { Device, DeviceDetail, DiscoveryResult, Event, HealthCheck } from '../types'

export const useInventoryStore = defineStore('inventory', {
  state: () => ({ devices: [] as Device[], health: [] as (HealthCheck & { device_name: string; device_ip: string })[], events: [] as Event[], error: '', arpDiscovery: false }),
  actions: {
    async loadDevices() { try { this.devices = await apiGet<Device[]>('/api/v1/devices'); this.error = '' } catch (e) { this.error = String(e) } },
    // 能力未知时按不可用处理，避免闪现一个点了必然报错的按钮
    async loadCapabilities() { try { const caps = await apiGet<{ arp_discovery: boolean }>('/api/v1/capabilities'); this.arpDiscovery = caps.arp_discovery } catch { this.arpDiscovery = false } },
    async loadHealth() { try { this.health = await apiGet<typeof this.health>('/api/v1/health'); this.error = '' } catch (e) { this.error = String(e) } },
    async loadEvents() { try { this.events = await apiGet<Event[]>('/api/v1/events'); this.error = '' } catch (e) { this.error = String(e) } },
    detail(id: number) { return apiGet<DeviceDetail>(`/api/v1/devices/${id}`) },
    createDevice(input: Omit<Device, 'id' | 'created_at' | 'updated_at'>) { return apiPost<Device>('/api/v1/devices', input) },
    updateDevice(id: number, input: Omit<Device, 'id' | 'created_at' | 'updated_at'>) { return apiPut<Device>(`/api/v1/devices/${id}`, input) },
    deleteDevice(id: number) { return apiDelete(`/api/v1/devices/${id}`) },
    createCheck(deviceId: number, input: Omit<HealthCheck, 'id' | 'device_id' | 'last_status' | 'last_error' | 'latency_ms' | 'checked_at'>) { return apiPost<HealthCheck>(`/api/v1/devices/${deviceId}/checks`, input) },
    deleteCheck(deviceId: number, id: number) { return apiDelete(`/api/v1/devices/${deviceId}/checks/${id}`) },
    async discoverARP() { try { const result = await apiPost<DiscoveryResult>('/api/v1/discovery/arp', {}); this.error = ''; return result } catch (e) { this.error = e instanceof Error ? e.message : String(e); throw e } },
  },
})
