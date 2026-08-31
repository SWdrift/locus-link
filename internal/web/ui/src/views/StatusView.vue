<script setup lang="ts">
import { ElCard } from 'element-plus'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../api'
import AsyncState from '../components/AsyncState.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useOperationalContext } from '../operational-context'
import { useStatusQuery } from '../queries'
import { usePreferences } from '../preferences'

import './status.css'
const { t } = useI18n()
const context = useOperationalContext()
const preferences = usePreferences()
const status = useStatusQuery()
const order: EvidenceStatus[] = ['success', 'failure', 'stale', 'unknown']
const empty = computed(() => Boolean(status.data.value && !status.data.value.links.length && !status.data.value.routes.length))
const formatObservedAt = (value: string) => new Intl.DateTimeFormat(preferences.locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
</script>

<template>
  <section class="status-page">
    <div class="section-intro">
      <div><span class="eyebrow">{{ t('status.eyebrow') }}</span><h2>{{ t('status.title') }}</h2></div>
      <p>{{ t('status.description', { vantage: context.vantage || '—' }) }}</p>
      <span v-if="status.isFetching.value && !status.isPending.value" class="refresh-indicator" role="status">{{ t('common.loading') }}</span>
    </div>

    <AsyncState :loading="status.isPending.value" :error="status.error.value" :empty="empty">
      <div class="status-summary">
        <ElCard v-for="state in order" :key="state" shadow="never" class="metric-card">
          <span>{{ t(`status.${state}`) }}</span><strong>{{ status.data.value?.summary[state] ?? 0 }}</strong><small>{{ t('status.linkCount') }}</small>
        </ElCard>
      </div>

      <ElCard shadow="never" class="detail-panel status-table-panel">
        <div class="panel-heading"><span class="eyebrow">{{ t('status.links') }}</span><h3>{{ t('status.measuredEvidence') }}</h3></div>
        <div class="table-scroll">
          <table><thead><tr><th>{{ t('status.links') }}</th><th>{{ t('status.state') }}</th><th>{{ t('status.provider') }}</th><th>{{ t('status.observed') }}</th></tr></thead>
            <tbody><tr v-for="item in status.data.value?.links" :key="item.link_id"><td class="technical-id">{{ item.link_id }}</td><td><StatusBadge :status="item.status" /></td><td>{{ item.observation?.provider ?? '—' }}</td><td>{{ item.observation ? formatObservedAt(item.observation.observed_at) : t('common.never') }}</td></tr></tbody>
          </table>
        </div>
      </ElCard>

      <ElCard shadow="never" class="detail-panel status-table-panel">
        <div class="panel-heading"><span class="eyebrow">{{ t('status.routes') }}</span><h3>{{ t('status.derivedStatus') }}</h3></div>
        <div class="table-scroll">
          <table><thead><tr><th>{{ t('status.routes') }}</th><th>{{ t('status.state') }}</th><th>{{ t('status.steps') }}</th></tr></thead>
            <tbody><tr v-for="item in status.data.value?.routes" :key="item.route_id"><td class="technical-id">{{ item.route_id }}</td><td><StatusBadge :status="item.evidence.status" /></td><td>{{ item.evidence.links.length }}</td></tr></tbody>
          </table>
        </div>
      </ElCard>
    </AsyncState>
  </section>
</template>
