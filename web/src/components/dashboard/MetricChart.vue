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
import { buildEChartsSeries, queryMetrics, type MetricQueryParams } from '../../api/metrics'

use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

export interface SeriesDef {
  /** 图例名称，同时也是传给 API 的 object 过滤值（若设置） */
  label: string
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

interface SeriesData {
  name: string
  type: 'line'
  smooth: boolean
  data: [string, number][]
}

const series = ref<SeriesData[]>([])

watchEffect(async () => {
  loading.value = true
  error.value = ''
  const since = new Date(Date.now() - props.timeRangeHours * 3600 * 1000).toISOString()

  const results = await Promise.allSettled(
    props.seriesDefs.map((def) =>
      queryMetrics({ ...def.params, since, limit: 500 }),
    ),
  )

  const allSeries: SeriesData[] = []
  for (const result of results) {
    if (result.status === 'fulfilled') {
      allSeries.push(...buildEChartsSeries(result.value))
    }
  }
  series.value = allSeries
  loading.value = false
})

const hasData = computed(() => series.value.some((s) => s.data.length > 0))

const chartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    formatter: (params: { seriesName: string; value: [string, number] }[]) =>
      params.map((p) => `${p.seriesName}: ${p.value[1]}${props.unit ?? '%'}`).join('<br/>'),
  },
  legend: { bottom: 0 },
  grid: { left: 48, right: 16, top: 12, bottom: 36 },
  xAxis: {
    type: 'time',
    axisLabel: { formatter: (v: number) => new Date(v).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) },
  },
  yAxis: {
    type: 'value',
    min: 0,
    max: props.unit ? undefined : 100,
    axisLabel: { formatter: (v: number) => `${v}${props.unit ?? '%'}` },
  },
  series: series.value,
}))
</script>

<template>
  <div style="position: relative; min-height: 160px">
    <div v-if="loading" style="display: flex; justify-content: center; padding: 40px">
      <n-spin size="small" />
    </div>
    <template v-else-if="hasData">
      <v-chart :option="chartOption" style="height: 200px" autoresize />
    </template>
    <n-empty v-else description="暂无数据" style="padding: 32px 0" />
  </div>
</template>
