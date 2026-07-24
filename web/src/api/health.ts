import { apiGet } from './client'
import type { HealthTransition } from '../types'

export interface HealthTimelineParams {
  checkId: number
  /** RFC3339 时间字符串 */
  since?: string
  limit?: number
}

/**
 * 查询某健康检查的状态迁移事件，用于还原绿红可用率时间线。
 * GET /api/v1/health/timeline?check_id=&since=&limit=
 */
export async function queryHealthTimeline(params: HealthTimelineParams): Promise<HealthTransition[]> {
  const qs = new URLSearchParams()
  qs.set('check_id', String(params.checkId))
  if (params.since) qs.set('since', params.since)
  if (params.limit != null) qs.set('limit', String(params.limit))
  return apiGet<HealthTransition[]>(`/api/v1/health/timeline?${qs.toString()}`)
}
