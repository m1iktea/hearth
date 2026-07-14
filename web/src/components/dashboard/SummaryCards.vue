<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NGi, NGrid } from 'naive-ui'
import type { OverviewSummary } from '../../utils/overview'

const props = defineProps<{
  summary: OverviewSummary
  /** health 数据是否已成功加载过（未加载时计数视为未知） */
  healthReady: boolean
  devicesReady: boolean
}>()

const emit = defineEmits<{ (e: 'focus-issues'): void }>()
const router = useRouter()

type Tone = 'danger' | 'success' | 'muted' | 'neutral'

interface SummaryItem {
  key: string
  label: string
  value: string
  hint: string
  tone: Tone
  onClick: () => void
}

const items = computed<SummaryItem[]>(() => {
  const s = props.summary
  const sourcesKnown = s.totalSources > 0
  return [
    {
      key: 'issues',
      label: '需处理',
      value: String(s.issueCount),
      hint: '离线数据源、失败检查与已停止容器',
      tone: s.issueCount > 0 ? 'danger' : 'success',
      onClick: () => emit('focus-issues'),
    },
    {
      key: 'sources',
      label: '数据源离线',
      value: sourcesKnown ? `${s.offlineSources} / ${s.totalSources}` : '未知',
      hint: '前往节点详情',
      tone: !sourcesKnown ? 'muted' : s.offlineSources > 0 ? 'danger' : 'success',
      onClick: () => router.push('/nodes'),
    },
    {
      key: 'checks',
      label: '健康检查异常',
      value: props.healthReady ? `${s.offlineChecks} / ${s.enabledChecks}` : '未知',
      hint: '前往健康中心',
      tone: !props.healthReady ? 'muted' : s.offlineChecks > 0 ? 'danger' : 'success',
      onClick: () => router.push({ path: '/health', query: { status: 'offline' } }),
    },
    {
      key: 'devices',
      label: '已纳管设备',
      value: props.devicesReady ? String(s.managedDevices) : '未知',
      hint: '前往设备中心',
      tone: 'neutral',
      onClick: () => router.push('/devices'),
    },
  ]
})
</script>

<template>
  <n-grid cols="2 m:4" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
    <n-gi v-for="item in items" :key="item.key">
      <n-card size="small" hoverable class="summary-card" @click="item.onClick">
        <div class="summary-label">{{ item.label }}</div>
        <div class="summary-value" :class="`tone-${item.tone}`">{{ item.value }}</div>
        <div class="summary-hint">{{ item.hint }}</div>
      </n-card>
    </n-gi>
  </n-grid>
</template>

<style scoped>
.summary-card {
  cursor: pointer;
  height: 100%;
}
.summary-label {
  font-size: 13px;
  opacity: 0.65;
}
.summary-value {
  font-size: 26px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.4;
}
.summary-hint {
  font-size: 12px;
  opacity: 0.5;
}
.tone-danger {
  color: #d03050;
}
.tone-success {
  color: #18a058;
}
.tone-muted {
  opacity: 0.5;
}
</style>
