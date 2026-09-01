<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../domain/locus'
import AsyncState from '../components/AsyncState.vue'
import CopyValue from '../components/CopyValue.vue'
import FilterToolbar from '../components/FilterToolbar.vue'
import EvidenceDetails from '../components/EvidenceDetails.vue'
import PageHeader from '../components/PageHeader.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { usePreferences } from '../preferences'
import { useStatusQuery } from '../queries'
import { scopePath, useScopeID } from '../scope-context'

const { t } = useI18n()
const preferences = usePreferences()
const statusQuery = useStatusQuery()
const scopeID = useScopeID()
const statusOrder: EvidenceStatus[] = ['success', 'failure', 'stale', 'unknown']
const linkSearchText = ref('')
const linkStatusFilter = ref<'all' | EvidenceStatus>('all')
const routeSearchText = ref('')
const routeStatusFilter = ref<'all' | EvidenceStatus>('all')
const linkPage = ref(1)
const routePage = ref(1)
const linkPageSize = ref(10)
const routePageSize = ref(10)
const statusFilterOptions = computed(() => [
  { value: 'all', label: t('status.allStatuses') },
  ...statusOrder.map(state => ({ value: state, label: t(`status.${state}`) })),
])

const filteredLinks = computed(() => {
  const term = linkSearchText.value.trim().toLocaleLowerCase()
  return (statusQuery.data.value?.links ?? []).filter(link => {
    if (linkStatusFilter.value !== 'all' && link.status !== linkStatusFilter.value) return false
    if (!term) return true
    return [link.link_id, link.status, link.observation?.provider ?? ''].some(value =>
      value.toLocaleLowerCase().includes(term),
    )
  })
})
const filteredRoutes = computed(() => {
  const term = routeSearchText.value.trim().toLocaleLowerCase()
  return (statusQuery.data.value?.routes ?? []).filter(route => {
    if (routeStatusFilter.value !== 'all' && route.evidence.status !== routeStatusFilter.value) return false
    if (!term) return true
    return [route.route_id, route.evidence.status, ...route.evidence.links.map(link => link.link_id)].some(value =>
      value.toLocaleLowerCase().includes(term),
    )
  })
})
const pagedLinks = computed(() => {
  const start = (linkPage.value - 1) * linkPageSize.value
  return filteredLinks.value.slice(start, start + linkPageSize.value)
})
const pagedRoutes = computed(() => {
  const start = (routePage.value - 1) * routePageSize.value
  return filteredRoutes.value.slice(start, start + routePageSize.value)
})

watch([linkSearchText, linkStatusFilter, () => statusQuery.data.value], () => {
  linkPage.value = 1
})
watch([routeSearchText, routeStatusFilter, () => statusQuery.data.value], () => {
  routePage.value = 1
})
watch(linkPageSize, () => (linkPage.value = 1))
watch(routePageSize, () => (routePage.value = 1))
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
        <ElCol :xs="24" :xl="12">
          <section class="status-view__panel">
            <header class="status-view__panel-header">
              <div>
                <span>{{ t('status.links') }}</span>
                <strong>{{ t('status.measuredEvidence') }}</strong>
              </div>
              <small>{{ filteredLinks.length }}</small>
            </header>
            <FilterToolbar
              v-model:search="linkSearchText"
              v-model:filter="linkStatusFilter"
              class="status-view__panel-filters"
              :search-placeholder="t('status.search')"
              :search-label="t('status.search')"
              :filter-label="t('status.filter')"
              :options="statusFilterOptions"
            />
            <div class="status-view__table-region">
              <ElTable
                class="status-view__table"
                :data="pagedLinks"
                size="small"
                height="100%"
                scrollbar-always-on
                :empty-text="t('common.noData')"
              >
                <ElTableColumn type="expand">
                  <template #default="{ row }">
                    <EvidenceDetails :status="row.status" :observation="row.observation" />
                  </template>
                </ElTableColumn>
                <ElTableColumn prop="link_id" :label="t('status.links')" min-width="220">
                  <template #default="{ row }">
                    <RouterLink :to="{ path: scopePath(scopeID, 'graph'), query: { kind: 'link', id: row.link_id } }">
                      <ElButton link type="primary" class="technical-id">{{ row.link_id }}</ElButton>
                    </RouterLink>
                  </template>
                </ElTableColumn>
                <ElTableColumn :label="t('status.state')" width="88">
                  <template #default="{ row }"><StatusBadge :status="row.status" /></template>
                </ElTableColumn>
                <ElTableColumn :label="t('status.provider')" width="100">
                  <template #default="{ row }">{{ row.provider }}</template>
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
                v-model:page-size="linkPageSize"
                class="status-view__pagination"
                small
                background
                layout="sizes, prev, pager, next"
                :page-sizes="[5, 10, 20]"
                :pager-count="5"
                :total="filteredLinks.length"
              />
            </footer>
          </section>
        </ElCol>

        <ElCol :xs="24" :xl="12">
          <section class="status-view__panel">
            <header class="status-view__panel-header">
              <div>
                <span>{{ t('status.routes') }}</span>
                <strong>{{ t('status.derivedStatus') }}</strong>
              </div>
              <small>{{ filteredRoutes.length }}</small>
            </header>
            <FilterToolbar
              v-model:search="routeSearchText"
              v-model:filter="routeStatusFilter"
              class="status-view__panel-filters"
              :search-placeholder="t('status.search')"
              :search-label="t('status.search')"
              :filter-label="t('status.filter')"
              :options="statusFilterOptions"
            />
            <div class="status-view__table-region">
              <ElTable
                class="status-view__table"
                :data="pagedRoutes"
                size="small"
                height="100%"
                scrollbar-always-on
                :empty-text="t('common.noData')"
              >
                <ElTableColumn type="expand">
                  <template #default="{ row }">
                    <div class="status-view__route-evidence">
                      <section v-for="(link, index) in row.evidence.links" :key="link.link_id">
                        <CopyValue :value="`${Number(index) + 1}. ${link.link_id}`" />
                        <EvidenceDetails :status="link.status" :observation="link.observation" />
                      </section>
                    </div>
                  </template>
                </ElTableColumn>
                <ElTableColumn prop="route_id" :label="t('status.routes')" min-width="220">
                  <template #default="{ row }">
                    <RouterLink :to="{ path: scopePath(scopeID, 'graph'), query: { route: row.route_id } }">
                      <ElButton link type="primary" class="technical-id">{{ row.route_id }}</ElButton>
                    </RouterLink>
                  </template>
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
                v-model:page-size="routePageSize"
                class="status-view__pagination"
                small
                background
                layout="sizes, prev, pager, next"
                :page-sizes="[5, 10, 20]"
                :pager-count="5"
                :total="filteredRoutes.length"
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
  align-items: flex-start;
  row-gap: var(--locus-space-5);
}

.status-view__panel {
  --status-row-height: 2.5rem;

  min-width: 0;
  height: var(--locus-data-panel-height);
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) var(--locus-pagination-height);
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

.status-view__panel-filters {
  padding: var(--locus-space-3) var(--locus-space-5);
}

.status-view__table-region {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.status-view__table {
  height: 100%;
}

.status-view__table :deep(.el-table__header-wrapper th.el-table__cell),
.status-view__table :deep(.el-table__body tr.el-table__row) {
  height: var(--status-row-height);
}

.status-view__table .technical-id {
  white-space: nowrap;
}

.status-view__route-evidence {
  display: grid;
  gap: var(--locus-space-3);
}

.status-view__pagination-region {
  min-width: 0;
  min-height: var(--locus-pagination-height);
  display: flex;
  align-items: center;
  padding: 0 var(--locus-space-5);
  container-type: inline-size;
  border-top: 1px solid var(--border-subtle);
}

.status-view__pagination {
  width: 100%;
  justify-content: flex-end;
}

@container (max-width: 360px) {
  .status-view__pagination {
    justify-content: center;
  }
}

@media (max-width: 767px) {
  .status-view__summary-column:nth-child(odd) {
    padding-left: 0;
    border-left: 0;
  }

  .status-view__summary-column:nth-child(even) {
    padding-right: 0;
  }

  .status-view__summary-column:nth-child(n + 3) {
    margin-top: var(--locus-space-6);
  }
}
</style>
