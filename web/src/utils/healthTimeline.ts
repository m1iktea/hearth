import type { HealthTransition } from '../types'

export type SegmentStatus = 'online' | 'offline' | 'nodata'

export interface TimelineSegment {
  status: SegmentStatus
  startMs: number
  endMs: number
  /** 仅 offline 段携带失败原因 */
  reason?: string
}

/**
 * 将状态迁移事件还原为覆盖 [windowStartMs, windowEndMs] 的连续状态段。
 * - 首个迁移之前的时间标记为 'nodata'（灰，表示无采集数据）。
 * - 每个落在窗口内的迁移开启一个新段，直到下一个迁移或窗口结束。
 * - 窗口开始前的最后一次迁移决定窗口起点的初始状态。
 */
export function buildTimelineSegments(
  transitions: HealthTransition[],
  windowStartMs: number,
  windowEndMs: number,
): TimelineSegment[] {
  if (!(windowEndMs > windowStartMs)) return []

  const sorted = transitions
    .map((t) => ({ ms: Date.parse(t.created_at), status: t.status, reason: t.reason ?? '' }))
    .filter((t) => Number.isFinite(t.ms))
    .sort((a, b) => a.ms - b.ms)

  // 窗口起点的初始状态：最后一个发生在 windowStart 之前（含）的迁移
  let startStatus: SegmentStatus = 'nodata'
  let startReason = ''
  for (const t of sorted) {
    if (t.ms <= windowStartMs) {
      startStatus = t.status
      startReason = t.reason
    }
  }

  const points: { ms: number; status: SegmentStatus; reason: string }[] = [
    { ms: windowStartMs, status: startStatus, reason: startReason },
  ]
  for (const t of sorted) {
    if (t.ms > windowStartMs && t.ms < windowEndMs) {
      points.push({ ms: t.ms, status: t.status, reason: t.reason })
    }
  }

  const segments: TimelineSegment[] = []
  for (let i = 0; i < points.length; i++) {
    const startMs = points[i].ms
    const endMs = i + 1 < points.length ? points[i + 1].ms : windowEndMs
    if (endMs <= startMs) continue
    segments.push({
      status: points[i].status,
      startMs,
      endMs,
      reason: points[i].status === 'offline' && points[i].reason ? points[i].reason : undefined,
    })
  }
  return segments
}

/**
 * 时段可用率 = online 时长 / (online + offline 时长) × 100。
 * nodata 段不计入分母；无任何有效覆盖时返回 null。
 */
export function computeUptime(segments: TimelineSegment[]): number | null {
  let up = 0
  let down = 0
  for (const s of segments) {
    const d = s.endMs - s.startMs
    if (s.status === 'online') up += d
    else if (s.status === 'offline') down += d
  }
  const total = up + down
  if (total <= 0) return null
  return (up / total) * 100
}
