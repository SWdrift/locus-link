<script setup lang="ts">
import { Search } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../domain/locus'
import AsyncState from '../components/AsyncState.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { usePreferences } from '../preferences'
import { useStatusQuery } from '../queries'

const { t } = useI18n()
const preferences = usePreferences()
const statusQuery = useStatusQuery()
const statusOrder: EvidenceStatus[] = ['success', 'failure', 'stale', 'unknown']
const searchText = ref('')
const statusFilter = ref<'all' | EvidenceStatus>('all')
const linkPage = ref(1)
const routePage = ref(1)
const pageSize = 5

const filteredLinks = computed(() => {
  const term = searchText.value.trim().toLocaleLowerCase()
  return (statusQuery.data.value?.links ?? []).filter(link => {
    if (statusFilter.value !== 'all' && link.status !== statusFilter.value) return false
    if (!term) return true
    return [link.link_id, link.status, link.observation?.provider ?? ''].some(value =>
      value.toLocaleLowerCase().includes(term),
    )
  })
})
const filteredRoutes = computed(() => {
  const term = searchText.value.trim().toLocaleLowerCase()
  return (statusQuery.data.value?.routes ?? []).filter(route => {
    if (statusFilter.value !== 'all' && route.evidence.status !== statusFilter.value) return false
    if (!term) return true
    return [route.route_id, route.evidence.status, ...route.evidence.links.map(link => link.link_id)].some(value =>
      value.toLocaleLowerCase().includes(term),
    )
  })
})
const pagedLinks = computed(() => {
  const start = (linkPage.value - 1) * pageSize
  return filteredLinks.value.slice(start, start + pageSize)
})
const pagedRoutes = computed(() => {
  const start = (routePage.value - 1) * pageSize
  return filteredRoutes.value.slice(start, start + pageSize)
})

watch([searchText, statusFilter, () => statusQuery.data.value], () => {
  linkPage.value = 1
  routePage.value = 1
})
const empty = computed(() =>
  Boolean(statusQuery.data.value && !statusQuery.data.value.links.length && !statusQuery.data.value.routes.length),
)
const formatObservedAt = (value: string) =>
  new Intl.DateTimeFormat(preferences.locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(
    new Date(value),
  )
</script>

<template>
  <section class="status-view">
    <PageHeader :eyebrow="t('status.eyebrow')" :title="t('status.title')">
      <template v-if="statusQuery.isFetching.value && !statusQuery.isPending.value" #actions>
        <ElTag type="info">{{ t('common.loading') }}</ElTag>
      </template>
    </PageHeader>

    <AsyncState
      :loading="statusQuery.isPending.value"
      :error="statusQuery.error.value"
      :empty="empty"
      :retrying="statusQuery.isFetching.value"
      retryable
      skeleton="status"
      @retry="statusQuery.refetch()"
    >
      <div class="status-view__filters">
        <ElInput v-model="searchText" clearable :placeholder="t('status.search')" :aria-label="t('status.search')">
          <template #prefix
            ><ElIcon><Search /></ElIcon
          ></template>
        </ElInput>
        <ElSelect v-model="statusFilter" :aria-label="t('status.filter')">
          <ElOption :label="t('status.allStatuses')" value="all" />
          <ElOption v-for="state in statusOrder" :key="state" :label="t(`status.${state}`)" :value="state" />
        </ElSelect>
      </div>

      <ElRow class="status-view__summary">
        <ElCol v-for="state in statusOrder" :key="state" class="status-view__summary-column" :xs="12" :sm="6">
          <ElStatistic :title="t(`status.${state}`)" :value="statusQuery.data.value?.summary[state] ?? 0">
            <template #suffix
              ><small>{{ t('status.linkCount') }}</small></template
            >
          </ElStatistic>
        </ElCol>
      </ElRow>

      <ElRow class="status-view__details" :gutter="10">
        <ElCol :xs="24" :xl="14">
          <section class="status-view__panel">
            <header class="status-view__panel-header">
              <div>
                <span>{{ t('status.links') }}</span>
                <strong>{{ t('status.measuredEvidence') }}</strong>
              </div>
              <small>{{ filteredLinks.length }}</small>
            </header>
            <div class="status-view__table-region">
              <ElTable
                class="status-view__table"
                :data="pagedLinks"
                stripe
                size="small"
                height="100%"
                :empty-text="t('common.noData')"
              >
                <ElTableColumn prop="link_id" :label="t('status.links')" min-width="220">
                  <template #default="{ row }"
                    ><span class="technical-id">{{ row.link_id }}</span></template
                  >
                </ElTableColumn>
                <ElTableColumn :label="t('status.state')" width="88">
                  <template #default="{ row }"><StatusBadge :status="row.status" /></template>
                </ElTableColumn>
                <ElTableColumn :label="t('status.provider')" width="100">
                  <template #default="{ row }">{{ row.observation?.provider ?? '—' }}</template>
                </ElTableColumn>
                <ElTableColumn :label="t('status.observed')" min-width="168">
                  <template #default="{ row }">
                    {{ row.observation ? formatObservedAt(row.observation.observed_at) : t('common.never') }}
                  </template>
                </ElTableColumn>
              </ElTable>
            </div>
            <footer class="status-view__pagination-region">
              <ElPagination
                v-model:current-page="linkPage"
                class="status-view__pagination"
                small
                background
                layout="total, prev, pager, next"
                :page-size="pageSize"
                :total="filteredLinks.length"
                :hide-on-single-page="filteredLinks.length <= pageSize"
              />
            </footer>
          </section>
        </ElCol>

        <ElCol :xs="24" :xl="10">
          <section class="status-view__panel">
            <header class="status-view__panel-header">
              <div>
                <span>{{ t('status.routes') }}</span>
                <strong>{{ t('status.derivedStatus') }}</strong>
              </div>
              <small>{{ filteredRoutes.length }}</small>
            </header>
            <div class="status-view__table-region">
              <ElTable
                class="status-view__table"
                :data="pagedRoutes"
                stripe
                size="small"
                height="100%"
                :empty-text="t('common.noData')"
              >
                <ElTableColumn prop="route_id" :label="t('status.routes')" min-width="220">
                  <template #default="{ row }"
                    ><span class="technical-id">{{ row.route_id }}</span></template
                  >
                </ElTableColumn>
                <ElTableColumn :label="t('status.state')" width="88">
                  <template #default="{ row }"><StatusBadge :status="row.evidence.status" /></template>
                </ElTableColumn>
                <ElTableColumn :label="t('status.steps')" width="72">
                  <template #default="{ row }">{{ row.evidence.links.length }}</template>
                </ElTableColumn>
              </ElTable>
            </div>
            <footer class="status-view__pagination-region">
              <ElPagination
                v-model:current-page="routePage"
                class="status-view__pagination"
                small
                background
                layout="total, prev, pager, next"
                :page-size="pageSize"
                :total="filteredRoutes.length"
                :hide-on-single-page="filteredRoutes.length <= pageSize"
              />
            </footer>
          </section>
        </ElCol>
      </ElRow>
    </AsyncState>
  </section>
</template>

<style scoped>
.status-view {
  min-width: 0;
}

.status-view__filters {
  display: grid;
  grid-template-columns: minmax(220px, 320px) 140px;
  gap: var(--locus-space-3);
  margin-bottom: var(--locus-space-4);
}

.status-view__summary {
  margin-bottom: var(--locus-space-5);
  padding: var(--locus-space-5) var(--locus-space-6);
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-md);
  background: var(--surface-panel);
}

.status-view__summary-column {
  min-width: 0;
  padding: 0 var(--locus-space-7);
  border-left: 1px solid var(--border-subtle);
}

.status-view__summary-column:first-child {
  padding-left: 0;
  border-left: 0;
}

.status-view__summary-column:last-child {
  padding-right: 0;
}

.status-view__details {
  row-gap: var(--locus-space-5);
}

.status-view__panel {
  min-width: 0;
  height: var(--locus-data-panel-height);
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) var(--locus-pagination-height);
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-md);
  background: var(--surface-panel);
}

.status-view__panel-header {
  min-height: var(--locus-pagination-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--locus-space-4);
  padding: var(--locus-space-4) var(--locus-space-6);
  border-bottom: 1px solid var(--border-subtle);
}

.status-view__panel-header div {
  display: grid;
  gap: var(--locus-space-1);
}

.status-view__panel-header span,
.status-view__panel-header small {
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.status-view__panel-header strong {
  font-size: var(--locus-font-size-base);
}

.status-view__table-region {
  min-height: 0;
}

.status-view__table {
  height: 100%;
}

.status-view__pagination-region {
  min-width: 0;
  min-height: var(--locus-pagination-height);
  display: flex;
  align-items: center;
  padding: 0 var(--locus-space-5);
  border-top: 1px solid var(--border-subtle);
}

.status-view__pagination {
  width: 100%;
  justify-content: flex-end;
}

@media (max-width: 600px) {
  .status-view__filters {
    grid-template-columns: 1fr;
  }

  .status-view__summary-column:nth-child(3) {
    padding-left: 0;
    border-left: 0;
  }

  .status-view__summary-column:nth-child(n + 3) {
    margin-top: var(--locus-space-6);
  }
}
</style>
