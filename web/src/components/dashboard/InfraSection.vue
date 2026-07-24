<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NGi, NGrid, NTag, useThemeVars } from 'naive-ui'
import type { DockerData, OpenWrtData, ProxmoxData, Snapshot } from '../../types'
import { formatAbsolute, formatRelative, formatUptime, percent } from '../../utils/format'
import { humanizeError, SOURCE_LABELS } from '../../utils/overview'

const props = defineProps<{ snapshots: Snapshot[]; now: number }>()
const router = useRouter()

interface InfraRow {
  label: string
  value: string
  warn?: boolean
}

interface InfraCard {
  source: string
  title: string
  status: 'online' | 'offline' | 'unknown'
  collectedAt?: string
  errorText?: string
  rows: InfraRow[]
}

function pveRows(data: ProxmoxData | undefined): InfraRow[] {
  if (!data) return []
  const nodes = data.nodes
  const running = nodes.flatMap((n) => n.vms).filter((v) => v.status === 'running').length
  const total = nodes.flatMap((n) => n.vms).length
  const topCpu = [...nodes].sort((a, b) => b.cpu - a.cpu)[0]
  const topMem = [...nodes].sort(
    (a, b) => percent(b.mem, b.maxmem) - percent(a.mem, a.maxmem),
  )[0]
  const rows: InfraRow[] = [{ label: '运行 VM', value: `${running} / ${total}` }]
  if (topCpu) {
    const pct = Math.round(topCpu.cpu * 100)
    rows.push({ label: '最高 CPU', value: `${topCpu.name} ${pct}%`, warn: pct >= 85 })
  }
  if (topMem) {
    const pct = percent(topMem.mem, topMem.maxmem)
    rows.push({ label: '最高内存', value: `${topMem.name} ${pct}%`, warn: pct >= 85 })
  }
  return rows
}

function dockerRows(data: DockerData | undefined): InfraRow[] {
  if (!data) return []
  const running = data.containers.filter((c) => c.state === 'running').length
  const stopped = data.containers.length - running
  return [
    { label: '运行容器', value: `${running} / ${data.containers.length}` },
    { label: '已退出', value: String(stopped), warn: stopped > 0 },
  ]
}

function wrtRows(data: OpenWrtData | undefined): InfraRow[] {
  if (!data) return []
  const memPct = percent(data.memory.total - data.memory.available, data.memory.total)
  const downIfaces = data.interfaces.filter((i) => !i.up).length
  const rows: InfraRow[] = [
    { label: '运行时长', value: formatUptime(data.uptime_sec) },
    { label: '内存占用', value: `${memPct}%`, warn: memPct >= 85 },
    { label: '负载', value: data.load.map((l) => l.toFixed(2)).join(' / ') },
  ]
  if (downIfaces > 0) rows.push({ label: '接口断开', value: String(downIfaces), warn: true })
  return rows
}

const cards = computed<InfraCard[]>(() =>
  (['proxmox', 'docker', 'openwrt'] as const).map((source) => {
    const snap = props.snapshots.find((s) => s.source === source)
    const rows =
      source === 'proxmox'
        ? pveRows(snap?.data as ProxmoxData | undefined)
        : source === 'docker'
          ? dockerRows(snap?.data as DockerData | undefined)
          : wrtRows(snap?.data as OpenWrtData | undefined)
    return {
      source,
      title: SOURCE_LABELS[source],
      status: snap ? snap.status : 'unknown',
      collectedAt: snap?.collected_at,
      errorText: snap?.status === 'offline' ? humanizeError(snap.last_error) : undefined,
      rows,
    }
  }),
)

// 复用 Naive UI 主题变量：随明/暗主题自动切换状态色
const themeVars = useThemeVars()

const STATUS_META = {
  online: { label: '在线', type: 'success' },
  offline: { label: '离线', type: 'error' },
  unknown: { label: '未知', type: 'default' },
} as const
</script>

<template>
  <div class="section-title">基础设施状态</div>
  <n-grid cols="1 s:3" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
    <n-gi v-for="card in cards" :key="card.source">
      <n-card size="small" hoverable class="infra-card" @click="router.push('/nodes')">
        <template #header>
          <span class="infra-title">{{ card.title }}</span>
        </template>
        <template #header-extra>
          <n-tag :type="STATUS_META[card.status].type" size="small" round>
            {{ STATUS_META[card.status].label }}
          </n-tag>
        </template>
        <div class="infra-time" :title="formatAbsolute(card.collectedAt)">
          <template v-if="card.status === 'offline'">
            离线 · 最后采集于 {{ formatRelative(card.collectedAt, now) }}
          </template>
          <template v-else-if="card.collectedAt">
            采集于 {{ formatRelative(card.collectedAt, now) }}
          </template>
          <template v-else>等待首次采集</template>
        </div>
        <div v-if="card.errorText" class="infra-error">{{ card.errorText }}</div>
        <div class="infra-rows" :class="{ dimmed: card.status !== 'online' }">
          <div v-for="row in card.rows" :key="row.label" class="infra-row">
            <span class="infra-row-label">{{ row.label }}</span>
            <span :class="{ 'infra-warn': row.warn }">{{ row.value }}</span>
          </div>
          <div v-if="card.rows.length === 0" class="infra-empty">暂无数据</div>
        </div>
      </n-card>
    </n-gi>
  </n-grid>
</template>

<style scoped>
.section-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 10px;
}
.infra-card {
  cursor: pointer;
  height: 100%;
}
.infra-title {
  font-size: 14px;
}
.infra-time {
  font-size: 12px;
  opacity: 0.65;
}
.infra-error {
  font-size: 12px;
  color: v-bind('themeVars.errorColor');
  margin-top: 4px;
}
.infra-rows {
  margin-top: 8px;
}
.infra-rows.dimmed {
  opacity: 0.45;
}
.infra-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  padding: 2px 0;
  font-variant-numeric: tabular-nums;
}
.infra-row-label {
  opacity: 0.65;
}
.infra-warn {
  color: v-bind('themeVars.warningColor');
  font-weight: 600;
}
.infra-empty {
  font-size: 13px;
  opacity: 0.5;
  padding: 8px 0;
  text-align: center;
}
</style>
