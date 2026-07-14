<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NAlert, NButton, NCard, NForm, NFormItem, NGrid, NGi, NInput, NModal, NPopconfirm, NSpace, NTag } from 'naive-ui'
import { useInventoryStore } from '../stores/inventory'

const store = useInventoryStore()
const router = useRouter()
const modal = ref(false); const scanning = ref(false); const scanMessage = ref('')
const form = reactive({ name: '', kind: 'other', hostname: '', ip_address: '', mac_address: '', location: '', notes: '', url: '', enabled: true })
onMounted(() => store.loadDevices())
function reset() { Object.assign(form, { name: '', kind: 'other', hostname: '', ip_address: '', mac_address: '', location: '', notes: '', url: '', enabled: true }) }
async function save() { if (!form.name.trim()) return; await store.createDevice(form); modal.value = false; reset(); await store.loadDevices() }
async function remove(id: number) { await store.deleteDevice(id); await store.loadDevices() }
async function scan() { scanning.value = true; scanMessage.value = ''; try { const result = await store.discoverARP(); scanMessage.value = `扫描完成：新增 ${result.new_count} 台，更新 ${result.updated_count} 台`; await store.loadDevices() } finally { scanning.value = false } }
</script>

<template>
  <n-space justify="space-between" align="center" style="margin-bottom: 16px">
    <div><h2 style="margin: 0">设备中心</h2><span style="opacity: .65">主动 ARP 发现，按 MAC 自动去重并纳管；可补充管理入口和备注</span></div>
    <n-space><n-button :loading="scanning" @click="scan">扫描局域网</n-button><n-button type="primary" @click="modal = true">＋ 添加设备</n-button></n-space>
  </n-space>
  <n-alert v-if="store.error" type="error" style="margin-bottom: 16px">{{ store.error }}</n-alert>
  <n-alert v-if="scanMessage" type="success" style="margin-bottom: 16px">{{ scanMessage }}</n-alert>
  <n-grid :cols="3" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
    <n-gi v-for="device in store.devices" :key="device.id" span="3 m:1">
      <n-card hoverable class="device-card" @click="router.push(`/devices/${device.id}`)">
        <template #header>{{ device.name }}</template>
        <template #header-extra><n-tag :type="device.enabled ? 'success' : 'default'" size="small">{{ device.enabled ? '已纳管' : '已停用' }}</n-tag></template>
        <div>{{ device.kind || 'other' }} <span v-if="device.location">· {{ device.location }}</span></div>
        <div class="muted">{{ device.ip_address || device.hostname || '未填写网络地址' }}</div>
        <n-space style="margin-top: 12px" @click.stop>
          <n-button v-if="device.url" size="tiny" tag="a" :href="device.url" target="_blank">管理入口</n-button>
          <n-popconfirm @positive-click="remove(device.id)"><template #trigger><n-button size="tiny" type="error">删除</n-button></template>删除设备及其检查记录？</n-popconfirm>
        </n-space>
      </n-card>
    </n-gi>
  </n-grid>
  <n-card v-if="!store.devices.length" size="small" style="margin-top: 16px">还没有设备。先录入路由器、NAS、PVE、交换机或服务所在主机。</n-card>

  <n-modal v-model:show="modal" preset="card" title="添加设备" style="width: 520px">
    <n-form>
      <n-form-item label="名称"><n-input v-model:value="form.name" placeholder="例如：飞牛 NAS" /></n-form-item>
      <n-form-item label="类型"><n-input v-model:value="form.kind" placeholder="nas / server / router / switch / iot" /></n-form-item>
      <n-form-item label="IP / 主机名"><n-input v-model:value="form.ip_address" placeholder="192.168.1.10" /></n-form-item>
      <n-form-item label="MAC 地址"><n-input v-model:value="form.mac_address" placeholder="可选" /></n-form-item>
      <n-form-item label="位置"><n-input v-model:value="form.location" placeholder="书房 / 机柜" /></n-form-item>
      <n-form-item label="管理 URL"><n-input v-model:value="form.url" placeholder="https://..." /></n-form-item>
      <n-form-item label="备注"><n-input v-model:value="form.notes" type="textarea" /></n-form-item>
    </n-form>
    <n-button type="primary" block @click="save">保存并添加健康检查</n-button>
  </n-modal>
</template>

<style scoped>
.device-card { cursor: pointer; height: 100%; }
.muted { margin-top: 6px; font-size: 13px; opacity: .65; }
</style>
