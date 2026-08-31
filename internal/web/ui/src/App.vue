<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { getContext } from './api'

const context = useQuery({ queryKey: ['context'], queryFn: getContext })
const scopeName = computed(() => context.data.value?.active_scope.id ?? 'Loading workspace')
const vantage = computed(() => context.data.value?.runtime.vantage ?? '—')
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">L</span>
        <div>
          <strong>Locus Link</strong>
          <small>Operational context</small>
        </div>
      </div>

      <nav aria-label="Primary navigation">
        <RouterLink to="/graph">Graph</RouterLink>
        <RouterLink to="/status">Status</RouterLink>
        <RouterLink to="/knowledge">Knowledge</RouterLink>
      </nav>

      <div class="workspace-card">
        <span>Active scope</span>
        <strong>{{ scopeName }}</strong>
        <small>Vantage · {{ vantage }}</small>
      </div>
    </aside>

    <main>
      <header class="topbar">
        <div>
          <span class="eyebrow">Workspace</span>
          <h1>{{ scopeName }}</h1>
        </div>
        <span class="connection" :class="{ error: context.isError.value }">
          {{ context.isError.value ? 'Unavailable' : 'Local service' }}
        </span>
      </header>

      <div v-if="context.isError.value" class="error-panel">
        {{ context.error.value?.message }}
      </div>
      <RouterView v-else :context="context.data.value" />
    </main>
  </div>
</template>
