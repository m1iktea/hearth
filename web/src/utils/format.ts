export function formatBytes(n: number): string {
  if (n <= 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return i === 0 ? `${v} B` : `${v.toFixed(1)} ${units[i]}`
}

export function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export function percent(used: number, total: number): number {
  if (total <= 0) return 0
  return Math.round((used / total) * 100)
}

/** 占用率映射进度条状态：>90% 红、>70% 黄，其余绿 */
export function usageStatus(pct: number): 'success' | 'warning' | 'error' {
  if (pct > 90) return 'error'
  if (pct > 70) return 'warning'
  return 'success'
}

/** 相对时间：<1 分钟“刚刚”、<1 小时“N 分钟前”、<1 天“N 小时前”、否则“N 天前” */
export function formatRelative(iso: string | undefined, now: number): string {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return '—'
  const diff = Math.max(0, now - t)
  if (diff < 60_000) return '刚刚'
  const min = Math.floor(diff / 60_000)
  if (min < 60) return `${min} 分钟前`
  const h = Math.floor(min / 60)
  if (h < 24) return `${h} 小时前`
  return `${Math.floor(h / 24)} 天前`
}

/** 持续时长文本，用于“已持续 N 分钟” */
export function formatDurationText(fromIso: string, now: number): string {
  const t = new Date(fromIso).getTime()
  if (Number.isNaN(t)) return '未知时长'
  const diff = Math.max(0, now - t)
  if (diff < 60_000) return '不足 1 分钟'
  const min = Math.floor(diff / 60_000)
  if (min < 60) return `${min} 分钟`
  const h = Math.floor(min / 60)
  if (h < 24) return `${h} 小时`
  return `${Math.floor(h / 24)} 天`
}

/** 浏览器本地时区的绝对时间，用于悬浮提示 */
export function formatAbsolute(iso: string | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString()
}
