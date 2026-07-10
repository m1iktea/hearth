import { defineStore } from 'pinia'
import { apiGet } from '../api/client'
import type { Snapshot } from '../types'

export const useStatusStore = defineStore('status', {
  state: () => ({
    snapshots: [] as Snapshot[],
    loading: false,
    error: '' as string,
    timer: 0 as number,
  }),
  getters: {
    bySource: (state) => (source: string) =>
      state.snapshots.find((s) => s.source === source),
  },
  actions: {
    async fetchNow() {
      this.loading = true
      try {
        this.snapshots = await apiGet<Snapshot[]>('/api/v1/status')
        this.error = ''
      } catch (e) {
        this.error = e instanceof Error ? e.message : String(e)
      } finally {
        this.loading = false
      }
    },
    startPolling(intervalMs = 10_000) {
      this.stopPolling()
      this.fetchNow()
      this.timer = window.setInterval(() => this.fetchNow(), intervalMs)
    },
    stopPolling() {
      if (this.timer) {
        window.clearInterval(this.timer)
        this.timer = 0
      }
    },
  },
})
