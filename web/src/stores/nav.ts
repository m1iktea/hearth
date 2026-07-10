import { defineStore } from 'pinia'
import { apiDelete, apiGet, apiPost, apiPut } from '../api/client'
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
      if (item.id) {
        await apiPut(`/api/v1/nav/items/${item.id}`, item)
      } else {
        await apiPost('/api/v1/nav/items', item)
      }
      await this.load()
    },
    async deleteItem(id: number) {
      await apiDelete(`/api/v1/nav/items/${id}`)
      await this.load()
    },
  },
})
