import { defineStore } from 'pinia'
import { apiDelete, apiGet, apiPost, apiPut } from '../api/client'
import { resolveCategorySelection } from '../utils/navCategory'
import type { CategorySelection } from '../utils/navCategory'
import type { NavCategory, NavItem } from '../types'

export const useNavStore = defineStore('nav', {
  state: () => ({
    categories: [] as NavCategory[],
    error: '' as string,
  }),
  actions: {
    async load() {
      try {
        this.categories = await apiGet<NavCategory[]>('/api/v1/nav')
        this.error = ''
      } catch (e) {
        this.error = e instanceof Error ? e.message : String(e)
      }
    },
    async createCategory(name: string, sortOrder: number) {
      await apiPost('/api/v1/nav/categories', { name, sort_order: sortOrder })
      await this.load()
    },
    async updateCategory(id: number, name: string, sortOrder: number) {
      await apiPut(`/api/v1/nav/categories/${id}`, { name, sort_order: sortOrder })
      await this.load()
    },
    async deleteCategory(id: number) {
      await apiDelete(`/api/v1/nav/categories/${id}`)
      await this.load()
    },
    async saveItem(item: Omit<NavItem, 'id'> & { id?: number }) {
      const body = {
        category_id: item.category_id,
        name: item.name,
        url: item.url,
        icon: item.icon,
        sort_order: item.sort_order,
        device_id: item.device_id ?? null,
      }
      if (item.id) {
        await apiPut(`/api/v1/nav/items/${item.id}`, body)
      } else {
        await apiPost('/api/v1/nav/items', body)
      }
      await this.load()
    },
    /** 保存条目，分类可为已有 id / 新分类名 / 未选（自动落入「未分类」，按需创建分类） */
    async saveItemWithCategory(
      item: Omit<NavItem, 'id' | 'category_id'> & { id?: number },
      sel: CategorySelection,
    ) {
      const resolved = resolveCategorySelection(this.categories, sel)
      const categoryId =
        resolved.kind === 'existing'
          ? resolved.id
          : (
              await apiPost<NavCategory>('/api/v1/nav/categories', {
                name: resolved.name,
                sort_order: resolved.sortOrder,
              })
            ).id
      await this.saveItem({ ...item, category_id: categoryId })
    },
    async deleteItem(id: number) {
      await apiDelete(`/api/v1/nav/items/${id}`)
      await this.load()
    },
    async linkDevice(itemId: number, deviceId: number): Promise<void> {
      const allItems = this.categories.flatMap((c) => c.items)
      const item = allItems.find((i) => i.id === itemId)
      if (!item) throw new Error(`nav item ${itemId} not found`)
      await apiPut(`/api/v1/nav/items/${itemId}`, {
        category_id: item.category_id,
        name: item.name,
        url: item.url,
        icon: item.icon,
        sort_order: item.sort_order,
        device_id: deviceId,
      })
      await this.load()
    },
    async unlinkDevice(itemId: number): Promise<void> {
      const allItems = this.categories.flatMap((c) => c.items)
      const item = allItems.find((i) => i.id === itemId)
      if (!item) throw new Error(`nav item ${itemId} not found`)
      await apiPut(`/api/v1/nav/items/${itemId}`, {
        category_id: item.category_id,
        name: item.name,
        url: item.url,
        icon: item.icon,
        sort_order: item.sort_order,
        device_id: null,
      })
      await this.load()
    },
  },
})
