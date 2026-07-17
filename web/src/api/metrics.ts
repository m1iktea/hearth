import { apiGet } from './client'

export interface MetricSample {
  source: string
  object: string
  metric: string
  value: number
  created_at: string
}

export interface MetricQueryParams {
  source?: string
  object?: string
  metric?: string
  /** RFC3339 时间字符串 */
  since?: string
  limit?: number
}

export interface EChartsSeriesItem {
  name: string
  type: 'line'
  smooth: boolean
  data: [string, number][]
}

/**
 * 查询 metric_samples，返回时序数组。
 * GET /api/v1/metrics?source=&object=&metric=&since=&limit=
 */
export async function queryMetrics(params: MetricQueryParams): Promise<MetricSample[]> {
  const qs = new URLSearchParams()
  if (params.source) qs.set('source', params.source)
  if (params.object) qs.set('object', params.object)
  if (params.metric) qs.set('metric', params.metric)
  if (params.since) qs.set('since', params.since)
  if (params.limit != null) qs.set('limit', String(params.limit))
  const query = qs.toString()
  return apiGet<MetricSample[]>(`/api/v1/metrics${query ? `?${query}` : ''}`)
}

/**
 * 将平坦的 MetricSample 数组按 object 分组，转换为 ECharts series 格式。
 * 输入已按时间升序（后端保证）。
 */
export function buildEChartsSeries(samples: MetricSample[]): EChartsSeriesItem[] {
  const groups = new Map<string, [string, number][]>()
  for (const s of samples) {
    if (!groups.has(s.object)) {
      groups.set(s.object, [])
    }
    groups.get(s.object)!.push([s.created_at, s.value])
  }
  return Array.from(groups.entries()).map(([name, data]) => ({
    name,
    type: 'line',
    smooth: true,
    data,
  }))
}
