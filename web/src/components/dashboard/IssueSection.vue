<script setup lang="ts">
import { computed, ref } from 'vue'
import { NAlert, NButton, NGi, NGrid } from 'naive-ui'
import type { Issue } from '../../utils/overview'
import { formatRelative } from '../../utils/format'
import IssueCard from './IssueCard.vue'

const MAX_COLLAPSED = 4

const props = defineProps<{
  issues: Issue[]
  now: number
  /** 最近一次事件时间，用于“全部正常”横幅的补充说明 */
  lastEventAt?: string
}>()

const expanded = ref(false)
const visibleIssues = computed(() =>
  expanded.value ? props.issues : props.issues.slice(0, MAX_COLLAPSED),
)
const hiddenCount = computed(() => props.issues.length - MAX_COLLAPSED)
</script>

<template>
  <n-alert v-if="issues.length === 0" type="success" :show-icon="true">
    未发现需要处理的问题<template v-if="lastEventAt">
      · 最近一次事件 {{ formatRelative(lastEventAt, now) }}</template>
  </n-alert>

  <template v-else>
    <div class="section-title">需要处理</div>
    <n-grid cols="1 m:2" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
      <n-gi v-for="issue in visibleIssues" :key="issue.id">
        <IssueCard :issue="issue" :now="now" />
      </n-gi>
    </n-grid>
    <div v-if="hiddenCount > 0 || expanded" class="issue-more">
      <n-button text type="primary" @click="expanded = !expanded">
        {{ expanded ? '收起' : `查看全部 ${issues.length} 项` }}
      </n-button>
    </div>
  </template>
</template>

<style scoped>
.section-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 10px;
}
.issue-more {
  margin-top: 10px;
  text-align: center;
}
</style>
