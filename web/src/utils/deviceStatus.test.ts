import { describe, expect, it } from 'vitest'
import { buildDeviceStatusMap } from './deviceStatus'

describe('buildDeviceStatusMap', () => {
  it('设备有任一 check 为 online → online', () => {
    const health = [
      { device_id: 1, last_status: 'offline' },
      { device_id: 1, last_status: 'online' },
    ]
    const map = buildDeviceStatusMap(health)
    expect(map.get(1)).toBe('online')
  })

  it('设备有 check 且全部非 online → offline', () => {
    const health = [
      { device_id: 2, last_status: 'offline' },
      { device_id: 2, last_status: 'offline' },
    ]
    const map = buildDeviceStatusMap(health)
    expect(map.get(2)).toBe('offline')
  })

  it('设备无 check 记录 → unknown（不出现在 map 中）', () => {
    const map = buildDeviceStatusMap([])
    expect(map.has(3)).toBe(false)
    expect(map.get(3)).toBeUndefined()
  })

  it('多设备混合场景互不干扰', () => {
    const health = [
      { device_id: 1, last_status: 'online' },
      { device_id: 2, last_status: 'offline' },
      { device_id: 3, last_status: 'offline' },
      { device_id: 3, last_status: 'online' },
    ]
    const map = buildDeviceStatusMap(health)
    expect(map.get(1)).toBe('online')
    expect(map.get(2)).toBe('offline')
    expect(map.get(3)).toBe('online')
    expect(map.has(4)).toBe(false)
  })

  it('online 先出现、后续有 offline 时仍保持 online', () => {
    const health = [
      { device_id: 1, last_status: 'online' },
      { device_id: 1, last_status: 'offline' },
    ]
    const map = buildDeviceStatusMap(health)
    expect(map.get(1)).toBe('online')
  })
})
