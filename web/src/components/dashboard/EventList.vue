<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NCard, NEmpty, NTag } from 'naive-ui'
import type { Event } from '../../types'
import { formatAbsolute, formatRelative } from '../../utils/format'

const MAX_EVENTS = 5

const props = defineProps<{ events: Event[]; now: number }>()
const router = useRouter()

const visible = computed(() => props.events.slice(0, MAX_EVENTS))

function tagType(e: Event): 'error' | 'success' | 'warning' | 'default' {
  if (e.type === 'offline') return 'error'
  if (e.type === 'online') return 'success'
  if (e.type === 'reboot') return 'warning'
  return 'default'
}
</script>

<template>
  <n-card title="最近事件" size="small" class="event-card">
    <template #header-extra>
      <n-button text type="primary" size="small" @click="router.push('/health')">
        查看全部事件
      </n-button>
    </template>
    <n-empty v-if="visible.length === 0" description="暂无状态变化事件" size="small" />
    <div v-for="e in visible" :key="e.id" class="event-row">
      <n-tag :type="tagType(e)" size="small">{{ e.title || e.type }}</n-tag>
      <span class="event-text">
        {{ e.device_name }}<template v-if="e.message"> · {{ e.message }}</template>
      </span>
      <span class="event-time" :title="formatAbsolute(e.created_at)">
        {{ formatRelative(e.created_at, now) }}
      </span>
    </div>
  </n-card>
</template>

<style scoped>
.event-card {
  height: 100%;
}
.event-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  font-size: 13px;
  min-width: 0;
}
.event-text {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.event-time {
  opacity: 0.55;
  white-space: nowrap;
  font-size: 12px;
}
</style>
