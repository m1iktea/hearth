<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { NAlert, NCard, NTable, NTag } from 'naive-ui'
import { useInventoryStore } from '../stores/inventory'
const store = useInventoryStore()
onMounted(async () => { await Promise.all([store.loadHealth(), store.loadEvents()]) })
const offline = computed(() => store.health.filter((c) => c.last_status === 'offline').length)
function tag(status: string) { return status === 'online' ? 'success' : status === 'offline' ? 'error' : 'default' }
</script>
<template>
  <h2 style="margin-top: 0">健康中心</h2>
  <n-alert :type="offline ? 'warning' : 'success'" style="margin-bottom: 16px">{{ offline ? `${offline} 项健康检查异常` : '所有已检查目标正常' }}</n-alert>
  <n-alert v-if="store.error" type="error" style="margin-bottom: 16px">{{ store.error }}</n-alert>
  <n-card title="当前巡检状态" style="margin-bottom: 16px">
    <n-table size="small"><thead><tr><th>设备</th><th>检查</th><th>类型</th><th>状态</th><th>延迟</th><th>最后检查</th></tr></thead><tbody>
      <tr v-for="c in store.health" :key="c.id"><td>{{ c.device_name }}</td><td>{{ c.name }}</td><td>{{ c.type.toUpperCase() }}</td><td><n-tag :type="tag(c.last_status)" size="small">{{ c.last_status }}</n-tag></td><td>{{ c.latency_ms ? `${c.latency_ms} ms` : '-' }}</td><td>{{ c.checked_at ? new Date(c.checked_at).toLocaleString() : '等待首次检查' }}</td></tr>
      <tr v-if="!store.health.length"><td colspan="6">还没有启用的健康检查</td></tr>
    </tbody></n-table>
  </n-card>
  <n-card title="最近事件"><n-table size="small"><thead><tr><th>时间</th><th>事件</th><th>设备</th><th>说明</th></tr></thead><tbody>
    <tr v-for="event in store.events" :key="event.id"><td>{{ new Date(event.created_at).toLocaleString() }}</td><td><n-tag :type="event.severity === 'warning' ? 'warning' : 'info'" size="small">{{ event.title }}</n-tag></td><td>{{ event.device_name }}</td><td>{{ event.message }}</td></tr>
    <tr v-if="!store.events.length"><td colspan="4">暂无状态切换事件</td></tr>
  </tbody></n-table></n-card>
</template>
