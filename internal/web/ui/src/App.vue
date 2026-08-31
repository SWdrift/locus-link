<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { getContext, getValidation } from './api'

const context = useQuery({ queryKey: ['context'], queryFn: getContext })
const validation = useQuery({ queryKey: ['validation'], queryFn: getValidation })
const from = ref('')
const vantage = ref('')
watch(() => context.data.value, (value) => {
  if (!value) return
  if (!from.value) from.value = value.runtime.current_entity ?? ''
  if (!vantage.value) vantage.value = value.runtime.vantage
}, { immediate: true })
const scopeName = computed(() => context.data.value?.active_scope.id ?? 'Loading workspace')
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand"><span class="brand-mark">L</span><div><strong>Locus Link</strong><small>Operational context</small></div></div>
      <nav aria-label="Primary navigation">
        <RouterLink to="/graph">Graph</RouterLink>
        <RouterLink to="/status">Status</RouterLink>
        <RouterLink to="/knowledge">Knowledge</RouterLink>
      </nav>
      <div class="workspace-card">
        <span>Active scope</span><strong>{{ scopeName }}</strong><small>Vantage · {{ vantage || '—' }}</small>
      </div>
    </aside>

    <main>
      <header class="topbar">
        <div><span class="eyebrow">Workspace</span><h1>{{ scopeName }}</h1></div>
        <div class="context-controls">
          <label>From<input v-model="from" aria-label="Current entity" /></label>
          <label>Vantage<input v-model="vantage" aria-label="Observation vantage" /></label>
          <span class="connection" :class="{ error: context.isError.value || validation.isError.value }">
            {{ validation.data.value?.valid ? 'Validated' : context.isError.value ? 'Unavailable' : 'Local service' }}
          </span>
        </div>
      </header>

      <div v-if="context.isError.value" class="error-panel">{{ context.error.value?.message }}</div>
      <RouterView v-else :context="context.data.value" :from="from" :vantage="vantage" />
    </main>
  </div>
</template>
