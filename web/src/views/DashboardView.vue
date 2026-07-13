<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { NAlert, NCard, NGrid, NGi, NTag } from 'naive-ui'
import MetricBar from '../components/MetricBar.vue'
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
const stoppedContainers = computed(() =>
  (dockerData.value?.containers ?? []).filter((c) => c.state !== 'running'),
)
</script>

<template>
  <n-alert v-if="store.error" type="error" style="margin-bottom: 16px">
    {{ store.error }}
  </n-alert>

  <n-grid :cols="3" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
    <!-- Proxmox VE -->
    <n-gi span="3 m:1">
      <n-card title="Proxmox VE" class="source-card">
        <template #header-extra>
          <n-tag :type="pve?.status === 'online' ? 'success' : 'error'" size="small" round>
            {{ pve?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="pveData">
          <div v-for="node in pveData.nodes" :key="node.name">
            <div class="sub-line">节点 {{ node.name }} · 运行 {{ formatUptime(node.uptime) }}</div>
            <div class="big-stat">
              {{ node.vms.filter((v) => v.status === 'running').length }}<span class="big-stat-dim">/{{ node.vms.length }}</span> VM 运行中
            </div>
            <MetricBar
              label="CPU"
              :value="`${Math.round(node.cpu * 100)}%`"
              :percentage="Math.round(node.cpu * 100)"
            />
            <MetricBar
              label="内存"
              :value="`${formatBytes(node.mem)} / ${formatBytes(node.maxmem)}`"
              :percentage="percent(node.mem, node.maxmem)"
            />
          </div>
        </template>
        <div v-else class="waiting">{{ pve?.last_error ?? '等待数据…' }}</div>
      </n-card>
    </n-gi>

    <!-- Docker -->
    <n-gi span="3 m:1">
      <n-card title="飞牛 Docker" class="source-card">
        <template #header-extra>
          <n-tag :type="docker?.status === 'online' ? 'success' : 'error'" size="small" round>
            {{ docker?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="dockerData">
          <div class="sub-line">共 {{ dockerData.containers.length }} 个容器</div>
          <div class="big-stat">
            {{ runningContainers }}<span class="big-stat-dim">/{{ dockerData.containers.length }}</span> 运行中
          </div>
          <MetricBar
            label="运行占比"
            :value="`${percent(runningContainers, dockerData.containers.length)}%`"
            :percentage="percent(runningContainers, dockerData.containers.length)"
          />
          <div class="sub-line" style="margin-top: 12px">
            <template v-if="stoppedContainers.length">
              未运行：{{ stoppedContainers.map((c) => c.name).join('、') }}
            </template>
            <template v-else>全部容器正常运行</template>
          </div>
        </template>
        <div v-else class="waiting">{{ docker?.last_error ?? '等待数据…' }}</div>
      </n-card>
    </n-gi>

    <!-- ImmortalWrt -->
    <n-gi span="3 m:1">
      <n-card title="ImmortalWrt" class="source-card">
        <template #header-extra>
          <n-tag :type="openwrt?.status === 'online' ? 'success' : 'error'" size="small" round>
            {{ openwrt?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="wrtData">
          <div class="sub-line">{{ wrtData.hostname }} · {{ wrtData.release }}</div>
          <div class="big-stat">运行 {{ formatUptime(wrtData.uptime_sec) }}</div>
          <MetricBar
            label="内存占用"
            :value="`可用 ${formatBytes(wrtData.memory.available)} / ${formatBytes(wrtData.memory.total)}`"
            :percentage="percent(wrtData.memory.total - wrtData.memory.available, wrtData.memory.total)"
          />
          <div class="sub-line" style="margin-top: 12px">
            负载 {{ wrtData.load.map((l) => l.toFixed(2)).join(' / ') }}
          </div>
        </template>
        <div v-else class="waiting">{{ openwrt?.last_error ?? '等待数据…' }}</div>
      </n-card>
    </n-gi>
  </n-grid>
</template>

<style scoped>
/* 卡片随所在行等高拉伸，消除三张卡高低不齐 */
.source-card {
  height: 100%;
}
.sub-line {
  font-size: 13px;
  opacity: 0.65;
}
.big-stat {
  font-size: 26px;
  font-weight: 600;
  line-height: 1.4;
  margin-top: 2px;
  font-variant-numeric: tabular-nums;
}
.big-stat-dim {
  opacity: 0.45;
  font-size: 20px;
}
.waiting {
  opacity: 0.6;
  padding: 16px 0;
  text-align: center;
}
</style>
