import { describe, expect, it } from 'vitest'
import { buildTimelineSegments, computeUptime } from './healthTimeline'
import type { HealthTransition } from '../types'

const START = Date.parse('2026-01-01T00:00:00Z')
const HOUR = 3600_000
const END = START + HOUR

function tr(status: 'online' | 'offline', offsetMs: number, reason = ''): HealthTransition {
  return {
    id: 0, check_id: 1, device_id: 1, target: 't', check_type: 'ping',
    status, latency_ms: 0, reason,
    created_at: new Date(START + offsetMs).toISOString(),
  }
}

describe('buildTimelineSegments', () => {
  it('无迁移时整段标记为 nodata', () => {
    const segs = buildTimelineSegments([], START, END)
    expect(segs).toHaveLength(1)
    expect(segs[0]).toMatchObject({ status: 'nodata', startMs: START, endMs: END })
  })

  it('窗口起点即 online 时整段为 online', () => {
    const segs = buildTimelineSegments([tr('online', 0)], START, END)
    expect(segs).toHaveLength(1)
    expect(segs[0].status).toBe('online')
    expect(segs[0].endMs - segs[0].startMs).toBe(HOUR)
  })

  it('中途 online→offline 切分为两段并携带原因', () => {
    const segs = buildTimelineSegments(
      [tr('online', 0), tr('offline', HOUR / 2, 'connection refused')],
      START, END,
    )
    expect(segs).toHaveLength(2)
    expect(segs[0]).toMatchObject({ status: 'online', startMs: START, endMs: START + HOUR / 2 })
    expect(segs[1]).toMatchObject({ status: 'offline', startMs: START + HOUR / 2, endMs: END, reason: 'connection refused' })
  })

  it('窗口开始前的迁移决定初始状态', () => {
    const segs = buildTimelineSegments([tr('offline', -600_000, 'down')], START, END)
    expect(segs).toHaveLength(1)
    expect(segs[0].status).toBe('offline')
  })
})

describe('computeUptime', () => {
  it('全 online 为 100%', () => {
    const segs = buildTimelineSegments([tr('online', 0)], START, END)
    expect(computeUptime(segs)).toBe(100)
  })

  it('一半 online 一半 offline 为 50%', () => {
    const segs = buildTimelineSegments([tr('online', 0), tr('offline', HOUR / 2)], START, END)
    expect(computeUptime(segs)).toBe(50)
  })

  it('仅 nodata 时返回 null（无有效覆盖）', () => {
    const segs = buildTimelineSegments([], START, END)
    expect(computeUptime(segs)).toBeNull()
  })

  it('nodata 段不计入分母', () => {
    // 前半段 nodata（无迁移），后半段 online → 可用率 100%
    const segs = buildTimelineSegments([tr('online', HOUR / 2)], START, END)
    expect(computeUptime(segs)).toBe(100)
  })
})
