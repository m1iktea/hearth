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

  it('同一 object 混入不同 metric 时仅按 object 分组（当前函数契约）', () => {
    // buildEChartsSeries 的契约：按 object 键分组，不区分 metric 字段。
    // 调用方应确保每次调用只传入同一 metric 的样本；若混入多 metric，
    // 数据点仍归到同一条序列，而非按 metric 拆分。
    const samples: MetricSample[] = [
      { source: 'proxmox', object: 'pve-01', metric: 'cpu_pct', value: 20.0, created_at: '2026-07-17T10:00:00Z' },
      { source: 'proxmox', object: 'pve-01', metric: 'mem_pct', value: 55.0, created_at: '2026-07-17T10:00:30Z' },
      { source: 'proxmox', object: 'pve-02', metric: 'cpu_pct', value: 10.0, created_at: '2026-07-17T10:00:00Z' },
    ]

    const series = buildEChartsSeries(samples)

    // 按 object 分组：pve-01（2 个数据点，来自两种 metric），pve-02（1 个数据点）
    expect(series).toHaveLength(2)
    const pve01 = series.find((s) => s.name === 'pve-01')
    expect(pve01).toBeDefined()
    expect(pve01!.data).toHaveLength(2)
    const pve02 = series.find((s) => s.name === 'pve-02')
    expect(pve02).toBeDefined()
    expect(pve02!.data).toHaveLength(1)
  })
})
