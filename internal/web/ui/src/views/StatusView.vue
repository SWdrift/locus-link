<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import type { LocusContext } from '../api'
import { getStatus } from '../api'

const props = defineProps<{ context?: LocusContext; from: string; vantage: string }>()
const status = useQuery({ queryKey: computed(() => ['status', props.from, props.vantage]), queryFn: () => getStatus(props.from, props.vantage), enabled: computed(() => Boolean(props.from && props.vantage)) })
const order = ['success', 'failure', 'stale', 'unknown']
</script>

<template>
  <section class="single-page status-page">
    <div class="section-intro"><span class="eyebrow">Observation</span><h2>Status</h2><p>Latest Link evidence for <strong>{{ vantage }}</strong>. Route status is derived on every read and is never persisted.</p></div>
    <div v-if="status.isError.value" class="error-panel">{{ status.error.value?.message }}</div>
    <template v-else>
      <div class="status-summary">
        <article v-for="state in order" :key="state" class="metric-card"><span>{{ state }}</span><strong>{{ status.data.value?.summary[state] ?? 0 }}</strong><small>Links</small></article>
      </div>
      <article class="detail-panel status-table-panel">
        <div class="panel-heading"><div><span class="eyebrow">Links</span><h3>Measured evidence</h3></div></div>
        <table><thead><tr><th>Link</th><th>Status</th><th>Provider</th><th>Observed</th></tr></thead>
          <tbody><tr v-for="item in status.data.value?.links" :key="item.link_id"><td>{{ item.link_id }}</td><td><span class="state-badge" :class="item.status">{{ item.status }}</span></td><td>{{ item.observation?.provider ?? '—' }}</td><td>{{ item.observation ? new Date(item.observation.observed_at).toLocaleString() : 'Never' }}</td></tr></tbody>
        </table>
      </article>
      <article class="detail-panel status-table-panel">
        <div class="panel-heading"><div><span class="eyebrow">Routes</span><h3>Derived status</h3></div></div>
        <table><thead><tr><th>Route</th><th>Status</th><th>Steps</th></tr></thead>
          <tbody><tr v-for="item in status.data.value?.routes" :key="item.route_id"><td>{{ item.route_id }}</td><td><span class="state-badge" :class="item.evidence.status">{{ item.evidence.status }}</span></td><td>{{ item.evidence.links.length }}</td></tr></tbody>
        </table>
      </article>
    </template>
  </section>
</template>
