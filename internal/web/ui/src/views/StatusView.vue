<script setup lang="ts">
import { ElCard, ElStatistic, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../api'
import AsyncState from '../components/AsyncState.vue'
import PageHeader from '../components/PageHeader.vue'
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
    <PageHeader :eyebrow="t('status.eyebrow')" :title="t('status.title')" :description="t('status.description', { vantage: context.vantage || '—' })">
      <template v-if="status.isFetching.value && !status.isPending.value" #actions><ElTag type="info">{{ t('common.loading') }}</ElTag></template>
    </PageHeader>

    <AsyncState :loading="status.isPending.value" :error="status.error.value" :empty="empty">
      <ElCard shadow="never" class="status-overview">
        <div class="status-summary">
          <ElStatistic v-for="state in order" :key="state" :title="t(`status.${state}`)" :value="status.data.value?.summary[state] ?? 0">
            <template #suffix><small>{{ t('status.linkCount') }}</small></template>
          </ElStatistic>
        </div>
      </ElCard>

      <div class="status-details">
        <ElCard shadow="never" class="detail-panel">
          <div class="panel-heading"><span class="eyebrow">{{ t('status.links') }}</span><h3>{{ t('status.measuredEvidence') }}</h3></div>
          <ElTable :data="status.data.value?.links ?? []" stripe size="small" :empty-text="t('common.noData')">
            <ElTableColumn prop="link_id" :label="t('status.links')" min-width="220"><template #default="{ row }"><span class="technical-id">{{ row.link_id }}</span></template></ElTableColumn>
            <ElTableColumn :label="t('status.state')" width="88"><template #default="{ row }"><StatusBadge :status="row.status" /></template></ElTableColumn>
            <ElTableColumn :label="t('status.provider')" width="100"><template #default="{ row }">{{ row.observation?.provider ?? '—' }}</template></ElTableColumn>
            <ElTableColumn :label="t('status.observed')" min-width="168"><template #default="{ row }">{{ row.observation ? formatObservedAt(row.observation.observed_at) : t('common.never') }}</template></ElTableColumn>
          </ElTable>
        </ElCard>

        <ElCard shadow="never" class="detail-panel">
          <div class="panel-heading"><span class="eyebrow">{{ t('status.routes') }}</span><h3>{{ t('status.derivedStatus') }}</h3></div>
          <ElTable :data="status.data.value?.routes ?? []" stripe size="small" :empty-text="t('common.noData')">
            <ElTableColumn prop="route_id" :label="t('status.routes')" min-width="220"><template #default="{ row }"><span class="technical-id">{{ row.route_id }}</span></template></ElTableColumn>
            <ElTableColumn :label="t('status.state')" width="88"><template #default="{ row }"><StatusBadge :status="row.evidence.status" /></template></ElTableColumn>
            <ElTableColumn :label="t('status.steps')" width="72"><template #default="{ row }">{{ row.evidence.links.length }}</template></ElTableColumn>
          </ElTable>
        </ElCard>
      </div>
    </AsyncState>
  </section>
</template>
