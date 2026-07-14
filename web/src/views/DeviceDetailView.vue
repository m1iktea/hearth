<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { NAlert, NButton, NCard, NForm, NFormItem, NInput, NInputNumber, NModal, NPopconfirm, NSpace, NTable, NTag } from 'naive-ui'
import { useInventoryStore } from '../stores/inventory'
import type { DeviceDetail } from '../types'

const route = useRoute(); const store = useInventoryStore(); const detail = ref<DeviceDetail | null>(null); const error = ref(''); const checkModal = ref(false); const editModal = ref(false)
const check = reactive({ name: '', type: 'ping', target: '', port: 0, expected_status: 0, enabled: true })
const edit = reactive({ name: '', kind: '', hostname: '', ip_address: '', mac_address: '', location: '', notes: '', url: '', enabled: true })
const id = Number(route.params.id)
async function load() { try { detail.value = await store.detail(id); error.value = '' } catch (e) { error.value = String(e) } }
onMounted(load)
async function addCheck() { if (!detail.value) return; await store.createCheck(id, check as never); checkModal.value = false; Object.assign(check, { name: '', type: 'ping', target: '', port: 0, expected_status: 0, enabled: true }); await load() }
function openEdit() { if (!detail.value) return; Object.assign(edit, detail.value.device); editModal.value = true }
async function saveEdit() { await store.updateDevice(id, edit); editModal.value = false; await load() }
async function removeCheck(checkID: number) { await store.deleteCheck(id, checkID); await load() }
function statusType(status: string) { return status === 'online' ? 'success' : status === 'offline' ? 'error' : 'default' }
</script>
<template>
  <n-alert v-if="error" type="error">{{ error }}</n-alert>
  <template v-if="detail">
    <n-space justify="space-between" align="center" style="margin-bottom: 16px"><div><h2 style="margin: 0">{{ detail.device.name }}</h2><span style="opacity:.65">{{ detail.device.kind }} · {{ detail.device.ip_address || detail.device.hostname || '未填写地址' }}</span></div><n-space><n-button @click="openEdit">编辑资料</n-button><n-button v-if="detail.device.url" tag="a" :href="detail.device.url" target="_blank">打开管理入口</n-button></n-space></n-space>
    <n-card title="设备资料" style="margin-bottom:16px"><n-table size="small"><tbody><tr><td>IP 地址</td><td>{{ detail.device.ip_address || '-' }}</td></tr><tr><td>MAC 地址</td><td>{{ detail.device.mac_address || '-' }}</td></tr><tr><td>位置</td><td>{{ detail.device.location || '-' }}</td></tr><tr><td>备注</td><td>{{ detail.device.notes || '-' }}</td></tr></tbody></n-table></n-card>
    <n-card title="健康检查"><template #header-extra><n-button size="small" type="primary" @click="checkModal = true">＋ 添加检查</n-button></template><n-table size="small"><thead><tr><th>名称</th><th>类型 / 目标</th><th>状态</th><th>延迟</th><th>详情</th><th></th></tr></thead><tbody>
      <tr v-for="item in detail.checks" :key="item.id"><td>{{ item.name }}</td><td>{{ item.type.toUpperCase() }} · {{ item.target || detail.device.ip_address }}<span v-if="item.port">:{{ item.port }}</span></td><td><n-tag :type="statusType(item.last_status)" size="small">{{ item.last_status }}</n-tag></td><td>{{ item.latency_ms ? `${item.latency_ms} ms` : '-' }}</td><td>{{ item.last_error || '-' }}</td><td><n-popconfirm @positive-click="removeCheck(item.id)"><template #trigger><n-button size="tiny" type="error">删除</n-button></template>确认删除检查？</n-popconfirm></td></tr>
      <tr v-if="!detail.checks.length"><td colspan="6">尚未添加检查</td></tr>
    </tbody></n-table></n-card>
    <n-modal v-model:show="checkModal" preset="card" title="添加健康检查" style="width:500px"><n-form><n-form-item label="名称"><n-input v-model:value="check.name" placeholder="例如：NAS 在线" /></n-form-item><n-form-item label="类型"><n-input v-model:value="check.type" placeholder="ping / tcp / http" /></n-form-item><n-form-item label="目标"><n-input v-model:value="check.target" placeholder="Ping/TCP 留空则使用设备 IP；HTTP 填完整 URL" /></n-form-item><n-form-item label="TCP 端口"><n-input-number v-model:value="check.port" :min="0" :max="65535" /></n-form-item><n-form-item label="期望 HTTP 状态码"><n-input-number v-model:value="check.expected_status" :min="0" :max="599" /></n-form-item></n-form><n-alert type="info" style="margin-bottom:12px">HTTP 留空状态码时接受 2xx/3xx；Ping 需要部署容器具备 NET_RAW 能力。</n-alert><n-button type="primary" block @click="addCheck">保存</n-button></n-modal>
    <n-modal v-model:show="editModal" preset="card" title="编辑设备" style="width:520px"><n-form><n-form-item label="名称"><n-input v-model:value="edit.name" /></n-form-item><n-form-item label="类型"><n-input v-model:value="edit.kind" /></n-form-item><n-form-item label="IP / 主机名"><n-input v-model:value="edit.ip_address" /></n-form-item><n-form-item label="MAC 地址"><n-input v-model:value="edit.mac_address" /></n-form-item><n-form-item label="位置"><n-input v-model:value="edit.location" /></n-form-item><n-form-item label="管理 URL"><n-input v-model:value="edit.url" /></n-form-item><n-form-item label="备注"><n-input v-model:value="edit.notes" type="textarea" /></n-form-item></n-form><n-button type="primary" block @click="saveEdit">保存</n-button></n-modal>
  </template>
</template>
