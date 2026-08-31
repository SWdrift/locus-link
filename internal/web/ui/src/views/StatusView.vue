<script setup lang="ts">
import { ElCard, ElStatistic, ElTable, ElTableColumn } from 'element-plus'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../api'
import AsyncState from '../components/AsyncState.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useOperationalContext } from '../operational-context'
import { usePreferences } from '../preferences'
import { useStatusQuery } from '../queries'
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
        <ElCard v-for="state in order" :key="state" shadow="never">
          <ElStatistic :title="t(`status.${state}`)" :value="status.data.value?.summary[state] ?? 0"><template #suffix><small>{{ t('status.linkCount') }}</small></template></ElStatistic>
        </ElCard>
      </div>

      <ElCard shadow="never" class="detail-panel">
        <div class="panel-heading"><span class="eyebrow">{{ t('status.links') }}</span><h3>{{ t('status.measuredEvidence') }}</h3></div>
        <ElTable :data="status.data.value?.links ?? []" stripe size="small" :empty-text="t('common.noData')">
          <ElTableColumn prop="link_id" :label="t('status.links')" min-width="260"><template #default="{ row }"><span class="technical-id">{{ row.link_id }}</span></template></ElTableColumn>
          <ElTableColumn :label="t('status.state')" width="100"><template #default="{ row }"><StatusBadge :status="row.status" /></template></ElTableColumn>
          <ElTableColumn :label="t('status.provider')" width="120"><template #default="{ row }">{{ row.observation?.provider ?? '—' }}</template></ElTableColumn>
          <ElTableColumn :label="t('status.observed')" min-width="180"><template #default="{ row }">{{ row.observation ? formatObservedAt(row.observation.observed_at) : t('common.never') }}</template></ElTableColumn>
        </ElTable>
      </ElCard>

      <ElCard shadow="never" class="detail-panel">
        <div class="panel-heading"><span class="eyebrow">{{ t('status.routes') }}</span><h3>{{ t('status.derivedStatus') }}</h3></div>
        <ElTable :data="status.data.value?.routes ?? []" stripe size="small" :empty-text="t('common.noData')">
          <ElTableColumn prop="route_id" :label="t('status.routes')" min-width="280"><template #default="{ row }"><span class="technical-id">{{ row.route_id }}</span></template></ElTableColumn>
          <ElTableColumn :label="t('status.state')" width="100"><template #default="{ row }"><StatusBadge :status="row.evidence.status" /></template></ElTableColumn>
          <ElTableColumn :label="t('status.steps')" width="100"><template #default="{ row }">{{ row.evidence.links.length }}</template></ElTableColumn>
        </ElTable>
      </ElCard>
    </AsyncState>
  </section>
</template>
