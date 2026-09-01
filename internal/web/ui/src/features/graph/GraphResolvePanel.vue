<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ResolveResult } from '../../domain/locus'

defineProps<{
  operationalReady: boolean
  resolving: boolean
  resolveResult?: ResolveResult
  resolveError?: Error | null
}>()
const emit = defineEmits<{ resolve: [] }>()
const target = defineModel<string>('target', { required: true })
const capability = defineModel<string>('capability', { required: true })
const { t } = useI18n()
</script>

<template>
  <section class="graph-resolve-panel">
    <label class="graph-resolve-panel__field">
      <span>{{ t('graph.target') }}</span>
      <ElInput v-model="target" :aria-label="t('graph.target')" />
    </label>
    <label class="graph-resolve-panel__field">
      <span>{{ t('graph.capability') }}</span>
      <ElInput v-model="capability" :aria-label="t('graph.capability')" />
    </label>
    <ElButton
      type="primary"
      :loading="resolving"
      :disabled="!operationalReady || !target || !capability"
      @click="emit('resolve')"
    >
      {{ t('graph.resolveRoute') }}
    </ElButton>
    <ElAlert
      v-if="resolveResult"
      type="success"
      :closable="false"
      :title="t(`graph.resolveStatus.${resolveResult.status}`)"
    >
      <span class="technical-id">
        {{
          resolveResult.route?.canonical_id ?? t('graph.candidates', { count: resolveResult.candidates?.length ?? 0 })
        }}
      </span>
    </ElAlert>
    <ElAlert v-if="resolveError" type="error" :closable="false" :title="resolveError.message" />
  </section>
</template>

<style scoped>
.graph-resolve-panel {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-4);
  padding: var(--locus-space-1) 0;
}

.graph-resolve-panel__field {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-1);
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}
</style>
