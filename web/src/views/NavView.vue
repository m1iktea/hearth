<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  NAlert, NButton, NCard, NForm, NFormItem, NGrid, NGi, NInput, NInputNumber,
  NModal, NPopconfirm, NSelect, NSpace, NSwitch,
} from 'naive-ui'
import { useNavStore } from '../stores/nav'
import type { NavItem } from '../types'

const store = useNavStore()
onMounted(() => store.load())

const manageMode = ref(false)

// --- 分类编辑 ---
const catModal = ref(false)
const catForm = reactive({ id: 0, name: '', sort_order: 0 })
function openCatModal(id = 0, name = '', sortOrder = 0) {
  Object.assign(catForm, { id, name, sort_order: sortOrder })
  catModal.value = true
}
async function saveCat() {
  if (!catForm.name.trim()) return
  if (catForm.id) await store.updateCategory(catForm.id, catForm.name, catForm.sort_order)
  else await store.createCategory(catForm.name, catForm.sort_order)
  catModal.value = false
}

// --- 条目编辑 ---
const itemModal = ref(false)
const itemForm = reactive<Omit<NavItem, 'id'> & { id?: number }>({
  category_id: 0, name: '', url: '', icon: '', sort_order: 0,
})
function openItemModal(categoryId: number, item?: NavItem) {
  Object.assign(itemForm, item ?? {
    id: undefined, category_id: categoryId, name: '', url: '', icon: '', sort_order: 0,
  })
  itemModal.value = true
}
async function saveItem() {
  if (!itemForm.name.trim() || !itemForm.url.trim()) return
  await store.saveItem({ ...itemForm })
  itemModal.value = false
}
</script>

<template>
  <n-space justify="space-between" style="margin-bottom: 16px">
    <span style="font-size: 16px">管理模式 <n-switch v-model:value="manageMode" /></span>
    <n-button v-if="manageMode" type="primary" @click="openCatModal()">新增分类</n-button>
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
        <n-button size="tiny" type="primary" @click="openItemModal(cat.id)">加链接</n-button>
      </template>
    </n-space>
    <n-grid :cols="4" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
      <n-gi v-for="item in cat.items" :key="item.id" span="4 m:1">
        <n-card size="small" hoverable>
          <a :href="item.url" target="_blank" rel="noopener" style="text-decoration: none; color: inherit">
            <strong>{{ item.icon ? item.icon + ' ' : '' }}{{ item.name }}</strong>
            <div style="font-size: 12px; opacity: 0.6">{{ item.url }}</div>
          </a>
          <n-space v-if="manageMode" style="margin-top: 8px">
            <n-button size="tiny" @click="openItemModal(cat.id, item)">编辑</n-button>
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

  <n-modal v-model:show="itemModal" preset="card" title="链接" style="width: 400px">
    <n-form>
      <n-form-item label="分类">
        <n-select
          v-model:value="itemForm.category_id"
          :options="store.categories.map((c) => ({ label: c.name, value: c.id }))"
        />
      </n-form-item>
      <n-form-item label="名称"><n-input v-model:value="itemForm.name" /></n-form-item>
      <n-form-item label="URL"><n-input v-model:value="itemForm.url" placeholder="https://..." /></n-form-item>
      <n-form-item label="图标（emoji）"><n-input v-model:value="itemForm.icon" placeholder="🖥️" /></n-form-item>
      <n-form-item label="排序"><n-input-number v-model:value="itemForm.sort_order" /></n-form-item>
    </n-form>
    <n-button type="primary" block @click="saveItem">保存</n-button>
  </n-modal>
</template>
