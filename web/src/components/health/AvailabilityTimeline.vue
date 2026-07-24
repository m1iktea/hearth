<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { NButton, NButtonGroup, NSpin, useThemeVars } from 'naive-ui'
import { queryHealthTimeline } from '../../api/health'
import { buildTimelineSegments, computeUptime, type SegmentStatus, type TimelineSegment } from '../../utils/healthTimeline'
import type { HealthTransition } from '../../types'

const props = defineProps<{ checkId: number }>()

// 状态色统一取 Naive UI 主题变量，随明/暗主题切换（无硬编码单主题色）
const themeVars = useThemeVars()
const colorMap = computed<Record<SegmentStatus, string>>(() => ({
  online: themeVars.value.successColor,
  offline: themeVars.value.errorColor,
  nodata: themeVars.value.dividerColor,
}))

const RANGES = [1, 6, 24] as const
const rangeHours = ref<1 | 6 | 24>(24)

const loading = ref(false)
const error = ref('')
const transitions = ref<HealthTransition[]>([])
const windowStartMs = ref(0)
const windowEndMs = ref(0)

// 竞态防护：沿用 MetricChart 的 cancelled 标志，范围切换/快速重渲染时丢弃过期响应
watchEffect(async (onCleanup) => {
  let cancelled = false
  onCleanup(() => { cancelled = true })

  loading.value = true
  error.value = ''
  const end = Date.now()
  const start = end - rangeHours.value * 3600_000
  try {
    const data = await queryHealthTimeline({
      checkId: props.checkId,
      since: new Date(start).toISOString(),
      limit: 2000,
    })
    if (cancelled) return
    transitions.value = data
    windowStartMs.value = start
    windowEndMs.value = end
  } catch (e) {
    if (cancelled) return
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    if (!cancelled) loading.value = false
  }
})

const segments = computed<TimelineSegment[]>(() =>
  buildTimelineSegments(transitions.value, windowStartMs.value, windowEndMs.value),
)
const uptime = computed(() => computeUptime(segments.value))
const uptimeText = computed(() =>
  uptime.value == null ? '—' : `${uptime.value.toFixed(uptime.value >= 99.95 ? 0 : 1)}%`,
)

function fmtTime(ms: number): string {
  return new Date(ms).toLocaleString('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  })
}
function segLabel(s: SegmentStatus): string {
  return s === 'online' ? '正常' : s === 'offline' ? '离线' : '无数据'
}
function segTitle(s: TimelineSegment): string {
  const head = `${segLabel(s.status)}｜${fmtTime(s.startMs)} – ${fmtTime(s.endMs)}`
  return s.reason ? `${head}\n原因：${s.reason}` : head
}
</script>

<template>
  <div class="timeline">
    <div class="timeline-head">
      <span class="uptime">可用率 <b>{{ uptimeText }}</b></span>
      <n-button-group size="tiny">
        <n-button
          v-for="h in RANGES"
          :key="h"
          :type="rangeHours === h ? 'primary' : 'default'"
          @click="rangeHours = h"
        >{{ h }}h</n-button>
      </n-button-group>
    </div>

    <div v-if="loading" class="timeline-loading"><n-spin size="small" /></div>
    <template v-else>
      <div v-if="error" class="timeline-error" :style="{ color: themeVars.errorColor }">{{ error }}</div>
      <div class="timeline-bar">
        <div
          v-for="(s, i) in segments"
          :key="i"
          class="timeline-seg"
          :style="{ background: colorMap[s.status], flexGrow: Math.max(1, s.endMs - s.startMs) }"
          :title="segTitle(s)"
        />
      </div>
      <div class="timeline-legend">
        <span><i :style="{ background: colorMap.online }" />正常</span>
        <span><i :style="{ background: colorMap.offline }" />离线</span>
        <span><i :style="{ background: colorMap.nodata }" />无数据</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.timeline {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.timeline-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.uptime {
  font-size: 12px;
  opacity: 0.75;
}
.uptime b {
  font-variant-numeric: tabular-nums;
}
.timeline-loading {
  display: flex;
  justify-content: center;
  padding: 8px 0;
}
.timeline-error {
  font-size: 12px;
}
.timeline-bar {
  display: flex;
  gap: 1px;
  height: 22px;
  border-radius: 4px;
  overflow: hidden;
}
.timeline-seg {
  min-width: 1px;
  transition: opacity 0.15s ease;
}
.timeline-seg:hover {
  opacity: 0.75;
}
.timeline-legend {
  display: flex;
  gap: 14px;
  font-size: 11px;
  opacity: 0.65;
}
.timeline-legend span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.timeline-legend i {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  display: inline-block;
}
</style>
