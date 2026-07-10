<script setup lang="ts">
import { h, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  NConfigProvider, NLayout, NLayoutSider, NLayoutContent, NMenu, darkTheme,
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'

const route = useRoute()
const activeKey = computed(() => (route.name as string) ?? 'dashboard')

const menuOptions: MenuOption[] = [
  { label: () => h(RouterLink, { to: '/' }, { default: () => '仪表盘' }), key: 'dashboard' },
  { label: () => h(RouterLink, { to: '/nav' }, { default: () => '导航' }), key: 'nav' },
  { label: () => h(RouterLink, { to: '/nodes' }, { default: () => '节点详情' }), key: 'nodes' },
]
</script>

<template>
  <n-config-provider :theme="darkTheme" style="height: 100vh">
    <n-layout has-sider style="height: 100%">
      <n-layout-sider bordered :width="180">
        <div style="padding: 16px; font-size: 18px; font-weight: 600">Hearth</div>
        <n-menu :options="menuOptions" :value="activeKey" />
      </n-layout-sider>
      <n-layout-content content-style="padding: 24px">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-config-provider>
</template>
