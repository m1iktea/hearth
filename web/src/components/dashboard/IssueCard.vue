<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NCard, NTag } from 'naive-ui'
import type { Issue, IssueSeverity } from '../../utils/overview'
import { formatAbsolute, formatDurationText, formatRelative } from '../../utils/format'

const props = defineProps<{ issue: Issue; now: number }>()
const router = useRouter()

const SEVERITY_META: Record<IssueSeverity, { label: string; type: 'error' | 'warning' | 'default' }> = {
  critical: { label: '严重', type: 'error' },
  warning: { label: '注意', type: 'warning' },
  info: { label: '提醒', type: 'default' },
}

const severity = computed(() => SEVERITY_META[props.issue.severity])

const timeText = computed(() => {
  if (props.issue.since) return `已持续 ${formatDurationText(props.issue.since, props.now)}`
  if (props.issue.lastCheckedAt) return `最后采集于 ${formatRelative(props.issue.lastCheckedAt, props.now)}`
  return ''
})

const timeTitle = computed(() => formatAbsolute(props.issue.since ?? props.issue.lastCheckedAt))
</script>

<template>
  <n-card size="small" class="issue-card">
    <div class="issue-head">
      <n-tag :type="severity.type" size="small">{{ severity.label }}</n-tag>
      <span class="issue-title">{{ issue.title }}</span>
    </div>
    <div class="issue-message">{{ issue.message }}</div>
    <div v-if="timeText" class="issue-meta" :title="timeTitle">{{ timeText }}</div>
    <div v-if="issue.context" class="issue-meta">{{ issue.context }}</div>
    <div class="issue-actions">
      <n-button size="small" type="primary" secondary @click="router.push(issue.detailPath)">
        查看详情
      </n-button>
      <n-button
        v-if="issue.managementUrl"
        size="small"
        secondary
        tag="a"
        :href="issue.managementUrl"
        target="_blank"
        rel="noopener noreferrer"
      >
        打开管理入口
      </n-button>
    </div>
  </n-card>
</template>

<style scoped>
.issue-card {
  height: 100%;
}
.issue-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.issue-title {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.issue-message {
  margin-top: 6px;
  font-size: 13px;
}
.issue-meta {
  margin-top: 4px;
  font-size: 12px;
  opacity: 0.65;
}
.issue-actions {
  margin-top: 10px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
