<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NAlert, NButton, NCard, NForm, NFormItem, NGrid, NGi, NInput, NInputNumber,
  NModal, NPopconfirm, NSelect, NSpace, NSwitch, NTag,
} from 'naive-ui'
import SiteIcon from '../components/SiteIcon.vue'
import { useNavStore } from '../stores/nav'
import { useInventoryStore } from '../stores/inventory'
import { suggestNameFromURL } from '../utils/navCategory'
import { buildDeviceStatusMap } from '../utils/deviceStatus'
import type { CategorySelection } from '../utils/navCategory'
import type { NavItem } from '../types'

const store = useNavStore()
const inventoryStore = useInventoryStore()

// 按设备 id 索引在线状态：任一 check last_status 为 online → online；有 check 但全非 online → offline；无 check → unknown
const deviceStatusMap = computed(() => buildDeviceStatusMap(inventoryStore.health))

onMounted(() => {
  store.load()
  inventoryStore.loadHealth()
})

const manageMode = ref(false)

// --- 分类改名 ---
const catModal = ref(false)
const catForm = reactive({ id: 0, name: '', sort_order: 0 })
function openCatModal(id: number, name: string, sortOrder: number) {
  Object.assign(catForm, { id, name, sort_order: sortOrder })
  catModal.value = true
}
async function saveCat() {
  if (!catForm.name.trim()) return
  await store.updateCategory(catForm.id, catForm.name, catForm.sort_order)
  catModal.value = false
}

// --- 添加 / 编辑导航 ---
const itemModal = ref(false)
const itemForm = reactive({
  id: undefined as number | undefined,
  name: '',
  url: '',
  icon: '',
  sort_order: 0,
  category: null as CategorySelection,
})
const categoryOptions = computed(() =>
  store.categories.map((c) => ({ label: c.name, value: c.id })),
)

function openItemModal(item?: NavItem, categoryId?: number) {
  Object.assign(itemForm, item
    ? { id: item.id, name: item.name, url: item.url, icon: item.icon, sort_order: item.sort_order, category: item.category_id }
    : { id: undefined, name: '', url: '', icon: '', sort_order: 0, category: categoryId ?? null })
  itemModal.value = true
}
function fillNameFromURL() {
  if (!itemForm.name.trim() && itemForm.url.trim()) {
    itemForm.name = suggestNameFromURL(itemForm.url)
  }
}
async function saveItem() {
  if (!itemForm.url.trim()) return
  fillNameFromURL()
  if (!itemForm.name.trim()) return
  const { category, ...item } = itemForm
  await store.saveItemWithCategory({ ...item }, category)
  itemModal.value = false
}
</script>

<template>
  <n-space justify="space-between" align="center" style="margin-bottom: 16px">
    <n-button type="primary" @click="openItemModal()">＋ 添加导航</n-button>
    <span style="font-size: 16px">管理模式 <n-switch v-model:value="manageMode" /></span>
  </n-space>

  <n-alert v-if="store.error" type="error" style="margin-bottom: 16px">{{ store.error }}</n-alert>

  <div v-for="cat in store.categories" :key="cat.id" style="margin-bottom: 24px">
    <n-space align="center" style="margin-bottom: 8px">
      <h3 style="margin: 0">{{ cat.name }}</h3>
      <template v-if="manageMode">
        <n-button size="tiny" @click="openCatModal(cat.id, cat.name, cat.sort_order)">改名</n-button>
        <n-popconfirm @positive-click="store.deleteCategory(cat.id)">
          <template #trigger><n-button size="tiny" type="error">删除</n-button></template>
          删除分类会同时删除其下所有链接，确认？
        </n-popconfirm>
        <n-button size="tiny" type="primary" @click="openItemModal(undefined, cat.id)">加链接</n-button>
      </template>
    </n-space>
    <n-grid :cols="4" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
      <n-gi v-for="item in cat.items" :key="item.id" span="4 m:1">
        <n-card size="small" hoverable class="nav-card">
          <a :href="item.url" target="_blank" rel="noopener" style="text-decoration: none; color: inherit">
            <strong>
              <SiteIcon :url="item.url" :fallback="item.icon" /> {{ item.name }}
              <n-tag
                v-if="item.device_id != null"
                :type="deviceStatusMap.get(item.device_id) === 'online' ? 'success'
                     : deviceStatusMap.get(item.device_id) === 'offline' ? 'error' : 'default'"
                size="tiny"
                style="margin-left: 4px; vertical-align: middle"
              >
                {{ deviceStatusMap.get(item.device_id) === 'online' ? '在线'
                 : deviceStatusMap.get(item.device_id) === 'offline' ? '离线' : '未知' }}
              </n-tag>
            </strong>
            <div style="font-size: 12px; opacity: 0.6">{{ item.url }}</div>
          </a>
          <n-space v-if="manageMode" style="margin-top: 8px">
            <n-button size="tiny" @click="openItemModal(item)">编辑</n-button>
            <n-popconfirm @positive-click="store.deleteItem(item.id)">
              <template #trigger><n-button size="tiny" type="error">删除</n-button></template>
              确认删除该链接？
            </n-popconfirm>
          </n-space>
        </n-card>
      </n-gi>
    </n-grid>
  </div>

  <n-modal v-model:show="catModal" preset="card" title="分类" style="width: 400px">
    <n-form>
      <n-form-item label="名称"><n-input v-model:value="catForm.name" /></n-form-item>
      <n-form-item label="排序"><n-input-number v-model:value="catForm.sort_order" /></n-form-item>
    </n-form>
    <n-button type="primary" block @click="saveCat">保存</n-button>
  </n-modal>

  <n-modal v-model:show="itemModal" preset="card" :title="itemForm.id ? '编辑导航' : '添加导航'" style="width: 400px">
    <n-form>
      <n-form-item label="URL">
        <n-input v-model:value="itemForm.url" placeholder="https://..." @blur="fillNameFromURL" />
      </n-form-item>
      <n-form-item label="名称">
        <n-input v-model:value="itemForm.name" placeholder="留空则自动取域名" />
      </n-form-item>
      <n-form-item label="分类（可选，可直接输入新分类名）">
        <n-select
          v-model:value="itemForm.category"
          filterable
          tag
          clearable
          placeholder="默认：未分类"
          :options="categoryOptions"
        />
      </n-form-item>
      <n-form-item label="排序">
        <n-input-number v-model:value="itemForm.sort_order" />
      </n-form-item>
    </n-form>
    <n-button type="primary" block @click="saveItem">保存</n-button>
  </n-modal>
</template>
