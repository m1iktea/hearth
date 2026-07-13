<script setup lang="ts">
import { h, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  NButton, NConfigProvider, NGlobalStyle, NLayout, NLayoutSider, NLayoutContent, NMenu,
  darkTheme,
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { useThemeStore } from './stores/theme'

const route = useRoute()
const activeKey = computed(() => (route.name as string) ?? 'dashboard')

const theme = useThemeStore()
const naiveTheme = computed(() => (theme.mode === 'dark' ? darkTheme : null))

const menuOptions: MenuOption[] = [
  { label: () => h(RouterLink, { to: '/' }, { default: () => '仪表盘' }), key: 'dashboard' },
  { label: () => h(RouterLink, { to: '/nav' }, { default: () => '导航' }), key: 'nav' },
  { label: () => h(RouterLink, { to: '/nodes' }, { default: () => '节点详情' }), key: 'nodes' },
]
</script>

<template>
  <n-config-provider :theme="naiveTheme" style="height: 100vh">
    <n-global-style />
    <n-layout has-sider style="height: 100%">
      <n-layout-sider bordered :width="180">
        <div style="padding: 16px; display: flex; align-items: center; justify-content: space-between">
          <span class="brand">Hearth</span>
          <n-button
            quaternary
            circle
            size="small"
            :title="theme.mode === 'dark' ? '切换到日间模式' : '切换到夜间模式'"
            @click="theme.toggle()"
          >
            {{ theme.mode === 'dark' ? '🌙' : '☀️' }}
          </n-button>
        </div>
        <n-menu :options="menuOptions" :value="activeKey" />
      </n-layout-sider>
      <n-layout-content content-style="padding: 24px">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-config-provider>
</template>
