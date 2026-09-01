<script setup lang="ts">
import { CopyDocument } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ value?: string; empty?: string }>()
const { t } = useI18n()
const copied = ref(false)

async function copyValue() {
  if (!props.value) return
  await navigator.clipboard.writeText(props.value)
  copied.value = true
  window.setTimeout(() => (copied.value = false), 1200)
}
</script>

<template>
  <span class="copy-value">
    <span class="technical-id">{{ value || empty || '—' }}</span>
    <ElButton
      v-if="value"
      text
      circle
      size="small"
      :icon="CopyDocument"
      :aria-label="copied ? t('common.copied') : t('common.copy')"
      @click="copyValue"
    />
  </span>
</template>

<style scoped>
.copy-value {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: var(--locus-space-1);
}

.copy-value .technical-id {
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
