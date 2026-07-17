import { describe, expect, it } from 'vitest'
import { buildEChartsSeries, type MetricSample } from '../api/metrics'

describe('buildEChartsSeries', () => {
  it('按 object 分组，时间升序，value 保留原值', () => {
    const samples: MetricSample[] = [
      { source: 'proxmox', object: 'pve-01', metric: 'cpu_pct', value: 20.5, created_at: '2026-07-17T10:00:00Z' },
      { source: 'proxmox', object: 'pve-02', metric: 'cpu_pct', value: 35.0, created_at: '2026-07-17T10:00:00Z' },
      { source: 'proxmox', object: 'pve-01', metric: 'cpu_pct', value: 22.0, created_at: '2026-07-17T10:01:00Z' },
    ]

    const series = buildEChartsSeries(samples)

    expect(series).toHaveLength(2)
    const pve01 = series.find((s) => s.name === 'pve-01')
    const pve02 = series.find((s) => s.name === 'pve-02')
    expect(pve01).toBeDefined()
    expect(pve01!.data).toHaveLength(2)
    expect(pve01!.data[0]).toEqual(['2026-07-17T10:00:00Z', 20.5])
    expect(pve01!.data[1]).toEqual(['2026-07-17T10:01:00Z', 22.0])
    expect(pve02).toBeDefined()
    expect(pve02!.data).toHaveLength(1)
  })

  it('空数组返回空序列', () => {
    expect(buildEChartsSeries([])).toEqual([])
  })
})
