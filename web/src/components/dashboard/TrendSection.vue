<script setup lang="ts">
// TODO: echarts 使 Dashboard chunk >500KB，功能稳定后可用异步组件或 manualChunks 拆分
import { ref } from 'vue'
import { NCard, NGi, NGrid, NRadioButton, NRadioGroup, NSpace } from 'naive-ui'
import MetricChart from './MetricChart.vue'
import type { SeriesDef } from './MetricChart.vue'

type TimeRange = 1 | 6 | 24

const timeRange = ref<TimeRange>(6)

const timeRangeOptions: { label: string; value: TimeRange }[] = [
  { label: '1h', value: 1 },
  { label: '6h', value: 6 },
  { label: '24h', value: 24 },
]

// PVE 节点 CPU 趋势：source=proxmox，metric=cpu_pct；每节点一条序列（不过滤 object，由 API 返回所有节点）
const pveCpuDefs: SeriesDef[] = [
  { params: { source: 'proxmox', metric: 'cpu_pct' } },
]

// PVE 节点内存趋势
const pveMemDefs: SeriesDef[] = [
  { params: { source: 'proxmox', metric: 'mem_pct' } },
]

// PVE 宿主机温度趋势：source=proxmox，metric=temp_c；object 为 lm-sensors 芯片名，每芯片一条序列
const pveTempDefs: SeriesDef[] = [
  { params: { source: 'proxmox', metric: 'temp_c' } },
]

// Docker 容器 CPU 趋势：source=docker，metric=cpu_pct
const dockerCpuDefs: SeriesDef[] = [
  { params: { source: 'docker', metric: 'cpu_pct' } },
]

// Docker 容器内存趋势：source=docker，metric=mem_pct
const dockerMemDefs: SeriesDef[] = [
  { params: { source: 'docker', metric: 'mem_pct' } },
]

// OpenWrt 内存趋势：source=openwrt，metric=mem_used_pct；object 为路由器 hostname
const openwrtMemDefs: SeriesDef[] = [
  { params: { source: 'openwrt', metric: 'mem_used_pct' } },
]
</script>

<template>
  <n-card title="资源趋势" style="margin-top: 16px">
    <template #header-extra>
      <n-radio-group v-model:value="timeRange" size="small">
        <n-radio-button v-for="opt in timeRangeOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </n-radio-button>
      </n-radio-group>
    </template>

    <n-grid :cols="2" x-gap="16" y-gap="16" responsive="screen" item-responsive>
      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">PVE CPU %</span>
          <MetricChart title="PVE CPU" :series-defs="pveCpuDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">PVE 内存 %</span>
          <MetricChart title="PVE 内存" :series-defs="pveMemDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">PVE 温度 ℃</span>
          <MetricChart title="PVE 温度" :series-defs="pveTempDefs" :time-range-hours="timeRange" unit="℃" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">Docker 容器 CPU %</span>
          <MetricChart title="Docker CPU" :series-defs="dockerCpuDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">Docker 容器内存 %</span>
          <MetricChart title="Docker 容器内存" :series-defs="dockerMemDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">OpenWrt 内存 %</span>
          <MetricChart title="OpenWrt 内存" :series-defs="openwrtMemDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>
    </n-grid>
  </n-card>
</template>
