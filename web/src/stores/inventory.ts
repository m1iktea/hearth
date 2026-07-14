import { defineStore } from 'pinia'
import { apiDelete, apiGet, apiPost, apiPut } from '../api/client'
import type { Device, DeviceDetail, DiscoveryResult, Event, HealthCheck } from '../types'

export const useInventoryStore = defineStore('inventory', {
  state: () => ({ devices: [] as Device[], health: [] as (HealthCheck & { device_name: string; device_ip: string })[], events: [] as Event[], error: '' }),
  actions: {
    async loadDevices() { try { this.devices = await apiGet<Device[]>('/api/v1/devices'); this.error = '' } catch (e) { this.error = String(e) } },
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
