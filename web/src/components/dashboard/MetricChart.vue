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
import { NEmpty, NSpin, useThemeVars } from 'naive-ui'
import { buildEChartsSeries, queryMetrics, type EChartsSeriesItem, type MetricQueryParams } from '../../api/metrics'
import { useThemeStore } from '../../stores/theme'

use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const themeStore = useThemeStore()
const isDark = computed(() => themeStore.mode === 'dark')
// 错误提示文字复用 Naive UI 主题变量，随明/暗主题切换
const themeVars = useThemeVars()

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

// 数据线色板：明/暗两套，随 isDark（主题 store 单一事实源）切换。
// 暗色用更亮/高饱和以在深底上清晰；亮色用更深以在白底上保证对比度。
const PALETTE_DARK = ['#5B8DEF', '#3CCB8E', '#F4B740', '#A78BFA', '#F0736B', '#3BC0D4', '#E687C4', '#94A3B8']
const PALETTE_LIGHT = ['#2F63D9', '#12946A', '#D98A1F', '#7C4DE0', '#D6453F', '#1B8F9E', '#C13E96', '#5B6B7F']
const palette = computed(() => (isDark.value ? PALETTE_DARK : PALETTE_LIGHT))

// 主题色令牌：随 isDark 响应式切换
const themeTokens = computed(() => isDark.value
  ? {
      axisLabelColor: 'rgba(255,255,255,0.55)',
      legendTextColor: 'rgba(255,255,255,0.65)',
      splitLineColor: 'rgba(255,255,255,0.08)',
      axisLineColor: 'rgba(255,255,255,0.2)',
      axisPointerColor: 'rgba(255,255,255,0.25)',
      tooltipBg: '#1f2430',
      tooltipBorder: 'rgba(255,255,255,0.1)',
      tooltipTextColor: 'rgba(255,255,255,0.85)',
      areaOpacitySuffix: '38', // ~22% alpha for dark
    }
  : {
      axisLabelColor: 'rgba(0,0,0,0.45)',
      legendTextColor: 'rgba(0,0,0,0.55)',
      splitLineColor: 'rgba(0,0,0,0.06)',
      axisLineColor: 'rgba(0,0,0,0.15)',
      axisPointerColor: 'rgba(0,0,0,0.2)',
      tooltipBg: '#fff',
      tooltipBorder: 'rgba(0,0,0,0.08)',
      tooltipTextColor: 'rgba(0,0,0,0.85)',
      areaOpacitySuffix: '2e', // ~18% alpha for light
    },
)

const chartOption = computed(() => {
  const t = themeTokens.value
  const colors = palette.value
  return {
    color: colors,
    animationDuration: 200,
    tooltip: {
      trigger: 'axis',
      backgroundColor: t.tooltipBg,
      borderColor: t.tooltipBorder,
      borderWidth: 1,
      textStyle: { color: t.tooltipTextColor },
      extraCssText: 'box-shadow:0 2px 8px rgba(0,0,0,0.15);',
      axisPointer: {
        type: 'line',
        lineStyle: { type: 'dashed', color: t.axisPointerColor, width: 1 },
      },
      formatter: (params: { seriesName: string; value: [string, number] }[]) =>
        params.map((p) => `${p.seriesName}: ${p.value[1]}${props.unit ?? '%'}`).join('<br/>'),
    },
    legend: {
      bottom: 0,
      icon: 'roundRect',
      itemWidth: 12,
      itemHeight: 3,
      textStyle: { fontSize: 11, color: t.legendTextColor },
    },
    grid: { left: 52, right: 16, top: 16, bottom: 40, containLabel: true },
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: t.axisLineColor } },
      axisTick: { show: false },
      axisLabel: {
        fontSize: 11,
        color: t.axisLabelColor,
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
        color: t.axisLabelColor,
        formatter: (v: number) => `${v}${props.unit ?? '%'}`,
      },
      splitLine: { lineStyle: { type: 'dashed', color: t.splitLineColor } },
    },
    series: series.value.map((s, idx) => {
      const baseColor = colors[idx % colors.length]
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
              { offset: 0, color: baseColor + t.areaOpacitySuffix },
              { offset: 1, color: baseColor + '00' },
            ],
          },
        },
      }
    }),
  }
})
</script>

<template>
  <div style="position: relative; min-height: 160px">
    <div v-if="loading" style="display: flex; justify-content: center; padding: 40px">
      <n-spin size="small" />
    </div>
    <template v-else-if="hasData">
      <v-chart :option="chartOption" style="height: 200px" autoresize />
      <div v-if="error" :style="{ fontSize: '12px', color: themeVars.errorColor, padding: '4px 0' }">{{ error }}</div>
    </template>
    <n-empty v-else :description="error || '暂无数据'" style="padding: 32px 0" />
  </div>
</template>
