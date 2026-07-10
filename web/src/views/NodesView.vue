<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { NCard, NSpace, NTable, NTag } from 'naive-ui'
import { useStatusStore } from '../stores/status'
import { formatBytes, formatUptime } from '../utils/format'
import type { DockerData, OpenWrtData, ProxmoxData } from '../types'

const store = useStatusStore()
onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())

const pveData = computed(() => store.bySource('proxmox')?.data as ProxmoxData | undefined)
const dockerData = computed(() => store.bySource('docker')?.data as DockerData | undefined)
const wrtData = computed(() => store.bySource('openwrt')?.data as OpenWrtData | undefined)
</script>

<template>
  <n-space vertical :size="16">
    <n-card title="Proxmox VM">
      <n-table v-if="pveData" size="small">
        <thead>
          <tr><th>VMID</th><th>名称</th><th>状态</th><th>CPU</th><th>内存</th><th>运行时长</th></tr>
        </thead>
        <tbody>
          <template v-for="node in pveData.nodes" :key="node.name">
            <tr v-for="vm in node.vms" :key="vm.vmid">
              <td>{{ vm.vmid }}</td>
              <td>{{ vm.name }}</td>
              <td>
                <n-tag :type="vm.status === 'running' ? 'success' : 'default'" size="small">
                  {{ vm.status }}
                </n-tag>
              </td>
              <td>{{ Math.round(vm.cpu * 100) }}%</td>
              <td>{{ formatBytes(vm.mem) }} / {{ formatBytes(vm.maxmem) }}</td>
              <td>{{ vm.status === 'running' ? formatUptime(vm.uptime) : '-' }}</td>
            </tr>
          </template>
        </tbody>
      </n-table>
      <span v-else>数据源离线或等待数据…</span>
    </n-card>

    <n-card title="Docker 容器">
      <n-table v-if="dockerData" size="small">
        <thead>
          <tr><th>名称</th><th>镜像</th><th>状态</th><th>详情</th></tr>
        </thead>
        <tbody>
          <tr v-for="c in dockerData.containers" :key="c.id">
            <td>{{ c.name }}</td>
            <td>{{ c.image }}</td>
            <td>
              <n-tag :type="c.state === 'running' ? 'success' : 'warning'" size="small">{{ c.state }}</n-tag>
            </td>
            <td>{{ c.status }}</td>
          </tr>
        </tbody>
      </n-table>
      <span v-else>数据源离线或等待数据…</span>
    </n-card>

    <n-card title="ImmortalWrt">
      <n-table v-if="wrtData" size="small">
        <tbody>
          <tr><td>主机名</td><td>{{ wrtData.hostname }}</td></tr>
          <tr><td>型号</td><td>{{ wrtData.model }}</td></tr>
          <tr><td>系统</td><td>{{ wrtData.release }}</td></tr>
          <tr><td>运行时长</td><td>{{ formatUptime(wrtData.uptime_sec) }}</td></tr>
          <tr><td>负载</td><td>{{ wrtData.load.map((l) => l.toFixed(2)).join(' / ') }}</td></tr>
          <tr>
            <td>内存</td>
            <td>可用 {{ formatBytes(wrtData.memory.available) }} / 共 {{ formatBytes(wrtData.memory.total) }}</td>
          </tr>
        </tbody>
      </n-table>
      <n-table v-if="wrtData" size="small" style="margin-top: 12px">
        <thead>
          <tr>
            <th>接口</th>
            <th>设备</th>
            <th>状态</th>
            <th>IPv4</th>
            <th>↓ 接收</th>
            <th>↑ 发送</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="iface in (wrtData.interfaces ?? [])" :key="iface.name">
            <td>{{ iface.name }}</td>
            <td>{{ iface.device }}</td>
            <td>
              <n-tag :type="iface.up ? 'success' : 'error'" size="small">
                {{ iface.up ? 'up' : 'down' }}
              </n-tag>
            </td>
            <td>{{ iface.ipv4 || '-' }}</td>
            <td>{{ formatBytes(iface.rx_bytes) }}</td>
            <td>{{ formatBytes(iface.tx_bytes) }}</td>
          </tr>
        </tbody>
      </n-table>
      <span v-else>数据源离线或等待数据…</span>
    </n-card>
  </n-space>
</template>
