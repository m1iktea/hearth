<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { DEFAULT_ICON, faviconURL } from '../utils/favicon'

const props = defineProps<{
  url: string
  /** 兜底 emoji（如旧数据里用户填过的 icon）；为空时用默认 emoji */
  fallback?: string
}>()

const failed = ref(false)
const src = computed(() => faviconURL(props.url))
watch(() => props.url, () => {
  failed.value = false
})
</script>

<template>
  <img
    v-if="src && !failed"
    :src="src"
    alt=""
    style="width: 16px; height: 16px; vertical-align: -2px"
    @error="failed = true"
  />
  <span v-else>{{ fallback || DEFAULT_ICON }}</span>
</template>
