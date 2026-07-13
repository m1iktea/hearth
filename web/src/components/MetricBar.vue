<script setup lang="ts">
import { NProgress } from 'naive-ui'
import { usageStatus } from '../utils/format'

defineProps<{
  label: string
  /** 右侧展示的数值文本，如 "6.1 GiB / 15.4 GiB" */
  value: string
  percentage: number
}>()
</script>

<template>
  <div class="metric">
    <div class="metric-head">
      <span class="metric-label">{{ label }}</span>
      <span class="metric-value">{{ value }}</span>
    </div>
    <n-progress
      type="line"
      :percentage="percentage"
      :show-indicator="false"
      :status="usageStatus(percentage)"
      :height="6"
    />
  </div>
</template>

<style scoped>
.metric {
  margin-top: 12px;
}
.metric-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
  font-size: 13px;
}
.metric-label {
  opacity: 0.65;
}
.metric-value {
  font-variant-numeric: tabular-nums;
}
</style>
