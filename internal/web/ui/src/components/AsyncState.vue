<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{ loading: boolean; error?: Error | null; empty?: boolean; emptyText?: string }>(), { error: null, empty: false, emptyText: '' })
const { t } = useI18n()
const errorText = computed(() => props.error?.message || t('common.error'))
</script>

<template>
  <ElSkeleton v-if="loading" class="locus-async-state" :rows="4" animated role="status" :aria-label="t('common.loading')" />
  <ElAlert v-else-if="error" type="error" :closable="false" :title="t('common.error')" :description="errorText" show-icon />
  <ElEmpty v-else-if="empty" class="locus-async-state" :description="emptyText || t('common.noData')" />
  <slot v-else />
</template>

<style scoped>
.locus-async-state {
  min-height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
}
</style>
