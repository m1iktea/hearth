import { defineStore } from 'pinia'
import { resolveInitialMode } from '../utils/theme'
import type { ThemeMode } from '../utils/theme'

const STORAGE_KEY = 'hearth-theme'

export const useThemeStore = defineStore('theme', {
  state: () => ({
    mode: resolveInitialMode(
      localStorage.getItem(STORAGE_KEY),
      window.matchMedia('(prefers-color-scheme: dark)').matches,
    ) as ThemeMode,
  }),
  actions: {
    toggle() {
      this.mode = this.mode === 'dark' ? 'light' : 'dark'
      localStorage.setItem(STORAGE_KEY, this.mode)
    },
  },
})
