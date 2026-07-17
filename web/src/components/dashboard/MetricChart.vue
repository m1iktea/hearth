<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { use } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import {
  GridComponent,
  LegendComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { NEmpty, NSpin } from 'naive-ui'
import { buildEChartsSeries, queryMetrics, type EChartsSeriesItem, type MetricQueryParams } from '../../api/metrics'

use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

export interface SeriesDef {
  params: MetricQueryParams
}

const props = defineProps<{
  title: string
  /** 每条查询序列的定义；支持多个并发请求 */
  seriesDefs: SeriesDef[]
  /** 时间范围（小时），映射到 since 参数 */
  timeRangeHours: 1 | 6 | 24
  unit?: string
}>()

const loading = ref(false)
const error = ref('')

const series = ref<EChartsSeriesItem[]>([])

watchEffect(async (onCleanup) => {
  let cancelled = false
  onCleanup(() => { cancelled = true })

  loading.value = true
  error.value = ''
  const since = new Date(Date.now() - props.timeRangeHours * 3600 * 1000).toISOString()

  const results = await Promise.allSettled(
    props.seriesDefs.map((def) =>
      queryMetrics({ ...def.params, since, limit: 500 }),
    ),
  )

  if (cancelled) return

  const allSeries: EChartsSeriesItem[] = []
  let failedCount = 0
  for (const result of results) {
    if (result.status === 'fulfilled') {
      allSeries.push(...buildEChartsSeries(result.value))
    } else {
      failedCount++
    }
  }

  series.value = allSeries
  loading.value = false

  if (failedCount > 0) {
    error.value = `${failedCount} 条序列加载失败`
  }
})

const hasData = computed(() => series.value.some((s) => s.data.length > 0))

const PALETTE = ['#4E7CF6', '#22B07D', '#F2A93B', '#8B5CF6', '#E45B5B', '#2AA7B8', '#D96BB0', '#7E8B9E']

const chartOption = computed(() => ({
  color: PALETTE,
  animationDuration: 200,
  tooltip: {
    trigger: 'axis',
    backgroundColor: '#fff',
    borderColor: 'rgba(0,0,0,0.08)',
    borderWidth: 1,
    extraCssText: 'box-shadow:0 2px 8px rgba(0,0,0,0.08);',
    axisPointer: {
      type: 'line',
      lineStyle: { type: 'dashed', color: 'rgba(0,0,0,0.2)', width: 1 },
    },
    formatter: (params: { seriesName: string; value: [string, number] }[]) =>
      params.map((p) => `${p.seriesName}: ${p.value[1]}${props.unit ?? '%'}`).join('<br/>'),
  },
  legend: {
    bottom: 0,
    icon: 'roundRect',
    itemWidth: 12,
    itemHeight: 3,
    textStyle: { fontSize: 11, color: 'rgba(0,0,0,0.55)' },
  },
  grid: { left: 52, right: 16, top: 16, bottom: 40, containLabel: true },
  xAxis: {
    type: 'time',
    axisLine: { lineStyle: { color: 'rgba(0,0,0,0.15)' } },
    axisTick: { show: false },
    axisLabel: {
      fontSize: 11,
      color: 'rgba(0,0,0,0.45)',
      formatter: (v: number) => new Date(v).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
    },
    splitLine: { show: false },
  },
  yAxis: {
    type: 'value',
    min: 0,
    max: props.unit ? undefined : 100,
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: {
      fontSize: 11,
      color: 'rgba(0,0,0,0.45)',
      formatter: (v: number) => `${v}${props.unit ?? '%'}`,
    },
    splitLine: { lineStyle: { type: 'dashed', color: 'rgba(0,0,0,0.06)' } },
  },
  series: series.value.map((s, idx) => {
    const baseColor = PALETTE[idx % PALETTE.length]
    return {
      ...s,
      showSymbol: false,
      smooth: 0.3,
      lineStyle: { width: 1.8 },
      emphasis: { focus: 'series' },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: baseColor + '2e' },
            { offset: 1, color: baseColor + '00' },
          ],
        },
      },
    }
  }),
}))
</script>

<template>
  <div style="position: relative; min-height: 160px">
    <div v-if="loading" style="display: flex; justify-content: center; padding: 40px">
      <n-spin size="small" />
    </div>
    <template v-else-if="hasData">
      <v-chart :option="chartOption" style="height: 200px" autoresize />
      <div v-if="error" style="font-size: 12px; color: #e88080; padding: 4px 0">{{ error }}</div>
    </template>
    <n-empty v-else :description="error || '暂无数据'" style="padding: 32px 0" />
  </div>
</template>
