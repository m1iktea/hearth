<script setup lang="ts">
import { h, computed, onMounted, onUnmounted, ref, watch } from 'vue'
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

const mobileQuery = window.matchMedia('(max-width: 767px)')
const isMobile = ref(mobileQuery.matches)
const collapsed = ref(mobileQuery.matches)

function onMediaChange(e: MediaQueryListEvent) {
  isMobile.value = e.matches
  collapsed.value = e.matches
}
onMounted(() => mobileQuery.addEventListener('change', onMediaChange))
onUnmounted(() => mobileQuery.removeEventListener('change', onMediaChange))

// 移动端点击菜单跳转后收起侧边栏，避免抽屉遮挡内容
watch(
  () => route.fullPath,
  () => {
    if (isMobile.value) collapsed.value = true
  },
)

const menuOptions: MenuOption[] = [
  { label: () => h(RouterLink, { to: '/' }, { default: () => '仪表盘' }), key: 'dashboard' },
  { label: () => h(RouterLink, { to: '/nav' }, { default: () => '导航' }), key: 'nav' },
  { label: () => h(RouterLink, { to: '/nodes' }, { default: () => '节点详情' }), key: 'nodes' },
  { label: () => h(RouterLink, { to: '/devices' }, { default: () => '设备中心' }), key: 'devices' },
  { label: () => h(RouterLink, { to: '/health' }, { default: () => '健康中心' }), key: 'health' },
]
</script>

<template>
  <n-config-provider :theme="naiveTheme" style="height: 100vh">
    <n-global-style />
    <n-layout has-sider style="height: 100%">
      <n-layout-sider
        bordered
        :width="180"
        :collapsed="collapsed"
        :collapsed-width="0"
        collapse-mode="transform"
        show-trigger="bar"
        @update:collapsed="(v: boolean) => (collapsed = v)"
      >
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
      <n-layout-content :content-style="isMobile ? 'padding: 16px' : 'padding: 24px'">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-config-provider>
</template>
