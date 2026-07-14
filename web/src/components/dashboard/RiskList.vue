<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NEmpty, NProgress } from 'naive-ui'
import type { ResourceRisk } from '../../utils/overview'
import { usageStatus } from '../../utils/format'

const MAX_RISKS = 5

const props = defineProps<{ risks: ResourceRisk[] }>()
const router = useRouter()

const visible = computed(() => props.risks.slice(0, MAX_RISKS))
</script>

<template>
  <n-card title="资源风险" size="small" class="risk-card">
    <n-empty v-if="visible.length === 0" description="当前未发现高资源占用" size="small" />
    <div
      v-for="risk in visible"
      :key="risk.id"
      class="risk-row"
      role="link"
      tabindex="0"
      @click="router.push(risk.detailPath)"
      @keydown.enter="router.push(risk.detailPath)"
    >
      <div class="risk-head">
        <span>{{ risk.label }}：{{ risk.metric }}</span>
        <span class="risk-value">{{ risk.value }}%</span>
      </div>
      <n-progress
        type="line"
        :percentage="risk.value"
        :show-indicator="false"
        :status="usageStatus(risk.value)"
        :height="6"
      />
    </div>
  </n-card>
</template>

<style scoped>
.risk-card {
  height: 100%;
}
.risk-row {
  padding: 6px 0;
  cursor: pointer;
}
.risk-head {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  margin-bottom: 4px;
}
.risk-value {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}
</style>
