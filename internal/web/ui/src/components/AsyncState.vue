<script setup lang="ts">
import { ElAlert, ElEmpty } from 'element-plus'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{ loading: boolean; error?: Error | null; empty?: boolean; emptyText?: string }>(), { error: null, empty: false, emptyText: '' })
const { t } = useI18n()
const errorText = computed(() => props.error?.message || t('common.error'))
</script>

<template>
  <div v-if="loading" class="async-state" role="status" aria-live="polite"><span class="loading-dot" aria-hidden="true"></span><span>{{ t('common.loading') }}</span></div>
  <ElAlert v-else-if="error" type="error" :closable="false" :title="t('common.error')" :description="errorText" show-icon />
  <ElEmpty v-else-if="empty" class="async-state" :description="emptyText || t('common.noData')" />
  <slot v-else />
</template>
