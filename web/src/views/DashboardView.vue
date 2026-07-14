<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NAlert, NButton, NGi, NGrid, NTag } from 'naive-ui'
import { useStatusStore } from '../stores/status'
import { useOverviewStore } from '../stores/overview'
import {
  buildIssues,
  buildRisks,
  resolveGlobalState,
  summarize,
} from '../utils/overview'
import { formatAbsolute, formatRelative } from '../utils/format'
import SummaryCards from '../components/dashboard/SummaryCards.vue'
import IssueSection from '../components/dashboard/IssueSection.vue'
import InfraSection from '../components/dashboard/InfraSection.vue'
import RiskList from '../components/dashboard/RiskList.vue'
import EventList from '../components/dashboard/EventList.vue'

const STATUS_POLL_MS = 10_000
const OVERVIEW_POLL_MS = 30_000

const router = useRouter()
const statusStore = useStatusStore()
const overview = useOverviewStore()

const now = ref(Date.now())
let nowTimer = 0

onMounted(() => {
  statusStore.startPolling(STATUS_POLL_MS)
  overview.startPolling(OVERVIEW_POLL_MS)
  nowTimer = window.setInterval(() => {
    now.value = Date.now()
  }, 10_000)
})

onUnmounted(() => {
  statusStore.stopPolling()
  overview.stopPolling()
  window.clearInterval(nowTimer)
})

const summary = computed(() =>
  summarize(statusStore.snapshots, overview.health.data, overview.devices.data),
)
const issues = computed(() =>
  buildIssues(
    statusStore.snapshots,
    overview.health.data,
    overview.devices.data,
    overview.events.data,
  ),
)
const risks = computed(() => buildRisks(statusStore.snapshots))
const globalState = computed(() => resolveGlobalState(summary.value, now.value, STATUS_POLL_MS))

const globalTag = computed(() => {
  switch (globalState.value) {
    case 'pending':
      return { label: '正在获取状态', type: 'default' as const }
    case 'issues':
      return { label: `${summary.value.issueCount} 项需处理`, type: 'error' as const }
    case 'stale':
      return { label: '数据可能已过期', type: 'warning' as const }
    default:
      return { label: '全部正常', type: 'success' as const }
  }
})

const updatedText = computed(() =>
  summary.value.updatedAt
    ? `更新于 ${formatRelative(summary.value.updatedAt, now.value)}`
    : '等待首次采集',
)

const lastEventAt = computed(() => overview.events.data[0]?.created_at)

/** 局部接口失败提示：各模块独立展示错误并可重试 */
const failures = computed(() => {
  const list: { key: string; label: string; error: string; retry: () => void }[] = []
  if (statusStore.error)
    list.push({ key: 'status', label: '数据源状态', error: statusStore.error, retry: () => void statusStore.fetchNow() })
  if (overview.health.error)
    list.push({ key: 'health', label: '健康检查', error: overview.health.error, retry: () => void overview.loadHealth() })
  if (overview.events.error)
    list.push({ key: 'events', label: '最近事件', error: overview.events.error, retry: () => void overview.loadEvents() })
  if (overview.devices.error)
    list.push({ key: 'devices', label: '设备台账', error: overview.devices.error, retry: () => void overview.loadDevices() })
  return list
})

const issueSectionEl = ref<HTMLElement | null>(null)
function focusIssues() {
  issueSectionEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const quickLinks = [
  { label: '设备中心', path: '/devices' },
  { label: '健康中心', path: '/health' },
  { label: '节点详情', path: '/nodes' },
  { label: '服务导航', path: '/nav' },
]
</script>

<template>
  <div class="dashboard">
    <header class="dash-head">
      <h2 class="dash-title">家庭运行概览</h2>
      <div class="dash-status">
        <n-tag :type="globalTag.type" size="medium" round>{{ globalTag.label }}</n-tag>
        <span class="dash-updated" :title="formatAbsolute(summary.updatedAt)">
          {{ updatedText }}
        </span>
      </div>
    </header>

    <n-alert
      v-for="f in failures"
      :key="f.key"
      type="error"
      class="dash-block"
      :title="`${f.label}加载失败`"
    >
      <div class="failure-body">
        <span>{{ f.error }}</span>
        <n-button size="small" @click="f.retry">重试</n-button>
      </div>
    </n-alert>

    <div class="dash-block">
      <SummaryCards
        :summary="summary"
        :health-ready="overview.health.loaded"
        :devices-ready="overview.devices.loaded"
        @focus-issues="focusIssues"
      />
    </div>

    <div ref="issueSectionEl" class="dash-block">
      <IssueSection :issues="issues" :now="now" :last-event-at="lastEventAt" />
    </div>

    <div class="dash-block">
      <InfraSection :snapshots="statusStore.snapshots" :now="now" />
    </div>

    <n-grid cols="1 m:2" :x-gap="12" :y-gap="12" responsive="screen" item-responsive class="dash-block">
      <n-gi>
        <RiskList :risks="risks" />
      </n-gi>
      <n-gi>
        <EventList :events="overview.events.data" :now="now" />
      </n-gi>
    </n-grid>

    <div class="dash-quick">
      <n-button
        v-for="link in quickLinks"
        :key="link.path"
        size="small"
        secondary
        @click="router.push(link.path)"
      >
        {{ link.label }}
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
  max-width: 1440px;
  margin: 0 auto;
}
.dash-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}
.dash-title {
  margin: 0;
  font-size: 20px;
}
.dash-status {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dash-updated {
  font-size: 12px;
  opacity: 0.6;
}
.dash-block {
  margin-bottom: 16px;
}
.dash-quick {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.failure-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
</style>
