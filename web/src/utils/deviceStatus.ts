/**
 * 根据健康检查记录推导每台设备的在线状态。
 *
 * 规则：
 * - 该设备有任一 check last_status 为 'online' → online
 * - 该设备有 check 但全部非 'online' → offline
 * - 该设备无 check 记录 → unknown（不出现在 map 中）
 */
export function buildDeviceStatusMap(
  health: Array<{ device_id: number; last_status: string }>,
): Map<number, 'online' | 'offline' | 'unknown'> {
  const map = new Map<number, 'online' | 'offline' | 'unknown'>()
  for (const check of health) {
    const prev = map.get(check.device_id)
    if (prev === 'online') continue
    if (check.last_status === 'online') {
      map.set(check.device_id, 'online')
    } else if (prev === undefined) {
      map.set(check.device_id, 'offline')
    }
  }
  return map
}
