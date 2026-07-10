<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { NAlert, NCard, NGrid, NGi, NProgress, NTag, NStatistic } from 'naive-ui'
import { useStatusStore } from '../stores/status'
import { formatBytes, formatUptime, percent } from '../utils/format'
import type { DockerData, OpenWrtData, ProxmoxData } from '../types'

const store = useStatusStore()
onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())

const pve = computed(() => store.bySource('proxmox'))
const docker = computed(() => store.bySource('docker'))
const openwrt = computed(() => store.bySource('openwrt'))

const pveData = computed(() => pve.value?.data as ProxmoxData | undefined)
const dockerData = computed(() => docker.value?.data as DockerData | undefined)
const wrtData = computed(() => openwrt.value?.data as OpenWrtData | undefined)

const runningContainers = computed(
  () => dockerData.value?.containers.filter((c) => c.state === 'running').length ?? 0,
)
</script>

<template>
  <n-alert v-if="store.error" type="error" style="margin-bottom: 16px">
    {{ store.error }}
  </n-alert>

  <n-grid :cols="3" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
    <n-gi span="3 m:1">
      <n-card title="Proxmox VE">
        <template #header-extra>
          <n-tag :type="pve?.status === 'online' ? 'success' : 'error'" size="small">
            {{ pve?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="pveData">
          <div v-for="node in pveData.nodes" :key="node.name">
            <n-statistic :label="`节点 ${node.name} · 运行 ${formatUptime(node.uptime)}`">
              {{ node.vms.filter((v) => v.status === 'running').length }}/{{ node.vms.length }} VM 运行中
            </n-statistic>
            <div style="margin-top: 8px">
              CPU {{ Math.round(node.cpu * 100) }}%
              <n-progress type="line" :percentage="Math.round(node.cpu * 100)" :show-indicator="false" />
            </div>
            <div style="margin-top: 8px">
              内存 {{ formatBytes(node.mem) }} / {{ formatBytes(node.maxmem) }}
              <n-progress type="line" :percentage="percent(node.mem, node.maxmem)" :show-indicator="false" />
            </div>
          </div>
        </template>
        <span v-else>{{ pve?.last_error ?? '等待数据…' }}</span>
      </n-card>
    </n-gi>

    <n-gi span="3 m:1">
      <n-card title="飞牛 Docker">
        <template #header-extra>
          <n-tag :type="docker?.status === 'online' ? 'success' : 'error'" size="small">
            {{ docker?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="dockerData">
          <n-statistic label="容器">
            {{ runningContainers }}/{{ dockerData.containers.length }} 运行中
          </n-statistic>
        </template>
        <span v-else>{{ docker?.last_error ?? '等待数据…' }}</span>
      </n-card>
    </n-gi>

    <n-gi span="3 m:1">
      <n-card title="ImmortalWrt">
        <template #header-extra>
          <n-tag :type="openwrt?.status === 'online' ? 'success' : 'error'" size="small">
            {{ openwrt?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="wrtData">
          <n-statistic :label="`${wrtData.hostname} · ${wrtData.release}`">
            运行 {{ formatUptime(wrtData.uptime_sec) }}
          </n-statistic>
          <div style="margin-top: 8px">负载 {{ wrtData.load.map((l) => l.toFixed(2)).join(' / ') }}</div>
          <div style="margin-top: 8px">
            内存可用 {{ formatBytes(wrtData.memory.available) }} / {{ formatBytes(wrtData.memory.total) }}
            <n-progress
              type="line"
              :percentage="percent(wrtData.memory.total - wrtData.memory.available, wrtData.memory.total)"
              :show-indicator="false"
            />
          </div>
        </template>
        <span v-else>{{ openwrt?.last_error ?? '等待数据…' }}</span>
      </n-card>
    </n-gi>
  </n-grid>
</template>
