<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { useI18n } from 'vue-i18n'
import { getContext, getGraph, getValidation } from '../api'
import AsyncState from '../components/AsyncState.vue'
import CopyValue from '../components/CopyValue.vue'
import FilterToolbar from '../components/FilterToolbar.vue'
import PageHeader from '../components/PageHeader.vue'
import type { BindingView, EntityView, LinkView, RouteView } from '../domain/locus'

const { t } = useI18n()
const route = useRoute()
const contextQuery = useQuery({ queryKey: ['context'], queryFn: getContext })
const graphQuery = useQuery({ queryKey: ['graph'], queryFn: getGraph })
const validationQuery = useQuery({ queryKey: ['validation'], queryFn: getValidation })
const tab = ref('context')
const search = ref('')
const declarationFilter = ref('all')
type Declaration =
  | ({ object_type: 'binding' } & BindingView)
  | ({ object_type: 'entity' } & EntityView)
  | ({ object_type: 'link' } & LinkView)
  | ({ object_type: 'route' } & RouteView)

const declarationFilterOptions = computed(() => [
  { value: 'all', label: t('inspect.allTypes') },
  { value: 'binding', label: t('inspect.binding') },
  { value: 'entity', label: t('graph.entities') },
  { value: 'link', label: t('graph.links') },
  { value: 'route', label: t('graph.routes') },
])

watch(
  () => route.query.tab,
  requested => {
    tab.value =
      typeof requested === 'string' && ['context', 'scopes', 'declarations', 'validation'].includes(requested)
        ? requested
        : 'context'
  },
  { immediate: true },
)

const declarations = computed<Declaration[]>(() => {
  const graph = graphQuery.data.value
  if (!graph) return []
  return [
    ...graph.bindings.map(item => ({ ...item, object_type: 'binding' as const })),
    ...graph.entities.map(item => ({ ...item, object_type: 'entity' as const })),
    ...graph.links.map(item => ({ ...item, object_type: 'link' as const })),
    ...graph.routes.map(item => ({ ...item, object_type: 'route' as const })),
  ]
})
const filteredDeclarations = computed(() => {
  const term = search.value.trim().toLocaleLowerCase()
  return declarations.value.filter(item => {
    if (declarationFilter.value !== 'all' && item.object_type !== declarationFilter.value) return false
    const values = [item.object_type, item.canonical_id, item.scope_id]
    if (item.object_type === 'binding') values.push(item.role, item.target)
    if (item.object_type === 'entity') values.push(item.kind, item.name, ...Object.entries(item.labels ?? {}).flat())
    if (item.object_type === 'link')
      values.push(item.from, item.to, item.provider, ...(item.requires ?? []), ...(item.provides ?? []))
    if (item.object_type === 'route') values.push(...item.steps)
    return !term || values.some(value => value.toLocaleLowerCase().includes(term))
  })
})
const blockedImports = computed(() => validationQuery.data.value?.blocked_imports ?? [])
</script>

<template>
  <section class="inspect-view">
    <PageHeader :eyebrow="t('inspect.eyebrow')" :title="t('inspect.title')" />
    <section class="inspect-view__workspace">
      <ElTabs v-model="tab" class="inspect-view__tabs">
        <ElTabPane :label="t('inspect.context')" name="context">
          <AsyncState
            :loading="contextQuery.isPending.value"
            :error="contextQuery.error.value"
            retryable
            @retry="contextQuery.refetch()"
          >
            <div v-if="contextQuery.data.value" class="inspect-view__stack">
              <ElDescriptions :title="t('inspect.runtime')" :column="1" border>
                <ElDescriptionsItem :label="t('graph.scope')"
                  ><CopyValue :value="contextQuery.data.value.active_scope.id"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.rootOrigin')">{{
                  contextQuery.data.value.root.root_origin
                }}</ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.registryPath')"
                  ><CopyValue :value="contextQuery.data.value.root.registry_path"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.userRegistryPath')"
                  ><CopyValue :value="contextQuery.data.value.root.user_registry_path"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.registered')">{{
                  contextQuery.data.value.root.registered ? t('common.yes') : t('common.no')
                }}</ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.userImport')">{{
                  contextQuery.data.value.root.has_user_import ? t('common.yes') : t('common.no')
                }}</ElDescriptionsItem>
                <ElDescriptionsItem :label="t('context.currentEntity')"
                  ><CopyValue :value="contextQuery.data.value.runtime.current_entity"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('context.vantage')"
                  ><CopyValue :value="contextQuery.data.value.runtime.vantage"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.cwd')"
                  ><CopyValue :value="contextQuery.data.value.runtime.cwd"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.store')"
                  ><CopyValue :value="contextQuery.data.value.observation_store"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.tools')"
                  ><CopyValue :value="contextQuery.data.value.runtime.available_tools.join(', ')"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.mechanismBindings')"
                  ><CopyValue :value="contextQuery.data.value.runtime.mechanism_bindings_source"
                /></ElDescriptionsItem>
              </ElDescriptions>
              <ElDescriptions
                v-if="contextQuery.data.value.root.registration"
                :title="t('inspect.registration')"
                :column="1"
                border
              >
                <ElDescriptionsItem :label="t('graph.scope')"
                  ><CopyValue :value="contextQuery.data.value.root.registration.scope_id"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.registryPath')"
                  ><CopyValue :value="contextQuery.data.value.root.registration.registry_path"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('inspect.registeredAt')">{{
                  contextQuery.data.value.root.registration.registered_at
                }}</ElDescriptionsItem>
              </ElDescriptions>
              <ElTable :data="contextQuery.data.value.root.source_cache ?? []" :empty-text="t('common.noData')">
                <ElTableColumn prop="import_alias" :label="t('diagnostics.aliasPath')" />
                <ElTableColumn prop="last_refresh_status" :label="t('inspect.refreshStatus')" />
                <ElTableColumn prop="resolved_revision" :label="t('inspect.revision')" />
                <ElTableColumn prop="active_content_digest" :label="t('inspect.digest')" min-width="220" />
                <ElTableColumn prop="last_refresh_error" :label="t('details.error')" min-width="180" />
              </ElTable>
            </div>
          </AsyncState>
        </ElTabPane>
        <ElTabPane :label="t('inspect.scopesImports')" name="scopes">
          <AsyncState
            :loading="graphQuery.isPending.value"
            :error="graphQuery.error.value"
            retryable
            @retry="graphQuery.refetch()"
          >
            <div class="inspect-view__stack">
              <ElTable :data="graphQuery.data.value?.scopes ?? []" :empty-text="t('common.noData')">
                <ElTableColumn :label="t('graph.scope')" min-width="180"
                  ><template #default="{ row }"><CopyValue :value="row.id" /></template
                ></ElTableColumn>
                <ElTableColumn :label="t('diagnostics.source')" min-width="220"
                  ><template #default="{ row }"><CopyValue :value="`${row.source.kind}:${row.source.uri}`" /></template
                ></ElTableColumn>
                <ElTableColumn :label="t('inspect.revision')" min-width="140"
                  ><template #default="{ row }"><CopyValue :value="row.resolved_revision" /></template
                ></ElTableColumn>
                <ElTableColumn :label="t('inspect.digest')" min-width="220"
                  ><template #default="{ row }"><CopyValue :value="row.content_digest" /></template
                ></ElTableColumn>
                <ElTableColumn :label="t('inspect.aliases')" min-width="180"
                  ><template #default="{ row }">{{
                    row.alias_paths.map((path: string[]) => path.join('::')).join(', ') || '—'
                  }}</template></ElTableColumn
                >
              </ElTable>
              <ElTable :data="graphQuery.data.value?.import_edges ?? []" :empty-text="t('common.noData')">
                <ElTableColumn prop="source_scope_id" :label="t('inspect.fromScope')" min-width="180" />
                <ElTableColumn prop="target_scope_id" :label="t('inspect.toScope')" min-width="180" />
                <ElTableColumn prop="alias" :label="t('diagnostics.aliasPath')" />
                <ElTableColumn prop="source.kind" :label="t('inspect.sourceKind')" />
                <ElTableColumn prop="source.uri" :label="t('inspect.sourceUri')" min-width="220" />
              </ElTable>
            </div>
          </AsyncState>
        </ElTabPane>
        <ElTabPane :label="t('inspect.declarations')" name="declarations">
          <AsyncState
            :loading="graphQuery.isPending.value"
            :error="graphQuery.error.value"
            retryable
            @retry="graphQuery.refetch()"
          >
            <div class="inspect-view__declarations">
              <FilterToolbar
                v-model:search="search"
                v-model:filter="declarationFilter"
                :search-placeholder="t('inspect.search')"
                :search-label="t('inspect.search')"
                :filter-label="t('inspect.typeFilter')"
                :options="declarationFilterOptions"
              />
              <ElTable :data="filteredDeclarations" height="100%" scrollbar-always-on :empty-text="t('common.noData')">
                <ElTableColumn type="expand">
                  <template #default="{ row }">
                    <ElDescriptions :column="1" border>
                      <ElDescriptionsItem v-for="(value, key) in row" :key="key" :label="String(key)">
                        <CopyValue :value="typeof value === 'string' ? value : JSON.stringify(value)" />
                      </ElDescriptionsItem>
                      <ElDescriptionsItem v-if="row.documentation_ids?.length" :label="t('graph.documents')">
                        <RouterLink
                          v-for="id in row.documentation_ids"
                          :key="id"
                          :to="{ path: '/knowledge', query: { document: id } }"
                        >
                          <ElButton link type="primary">{{ id }}</ElButton>
                        </RouterLink>
                      </ElDescriptionsItem>
                    </ElDescriptions>
                  </template>
                </ElTableColumn>
                <ElTableColumn prop="object_type" :label="t('inspect.type')" width="100" />
                <ElTableColumn :label="t('graph.canonicalId')" min-width="260"
                  ><template #default="{ row }"><CopyValue :value="row.canonical_id" /></template
                ></ElTableColumn>
                <ElTableColumn prop="scope_id" :label="t('graph.scope')" min-width="180" />
              </ElTable>
            </div>
          </AsyncState>
        </ElTabPane>
        <ElTabPane :label="t('inspect.validation')" name="validation">
          <AsyncState
            :loading="validationQuery.isPending.value"
            :error="validationQuery.error.value"
            retryable
            @retry="validationQuery.refetch()"
          >
            <div v-if="validationQuery.data.value" class="inspect-view__validation">
              <ElDescriptions :title="t('inspect.validation')" :column="1" border>
                <ElDescriptionsItem :label="t('inspect.validationStatus')">
                  <ElTag :type="validationQuery.data.value.completeness === 'complete' ? 'success' : 'warning'">
                    {{
                      validationQuery.data.value.completeness === 'complete'
                        ? t('diagnostics.complete')
                        : t('diagnostics.partial')
                    }}
                  </ElTag>
                </ElDescriptionsItem>
              </ElDescriptions>
              <section class="inspect-view__statistics">
                <ElStatistic
                  v-for="key in ['entities', 'links', 'routes']"
                  :key="key"
                  :title="t(`graph.${key}`)"
                  :value="validationQuery.data.value[key as 'entities' | 'links' | 'routes']"
                />
              </section>
              <template v-if="blockedImports.length">
                <ElAlert
                  type="warning"
                  show-icon
                  :closable="false"
                  :title="t('diagnostics.partial')"
                  :description="t('diagnostics.blockedCount', { count: blockedImports.length })"
                />
                <ElTable :data="blockedImports" size="small">
                  <ElTableColumn prop="source_scope_id" :label="t('diagnostics.sourceScope')" min-width="180" />
                  <ElTableColumn :label="t('diagnostics.aliasPath')" min-width="180">
                    <template #default="{ row }"><CopyValue :value="row.alias_path.join('::')" /></template>
                  </ElTableColumn>
                  <ElTableColumn prop="reason" :label="t('details.error')" min-width="160" />
                </ElTable>
              </template>
            </div>
          </AsyncState>
        </ElTabPane>
      </ElTabs>
    </section>
  </section>
</template>

<style scoped>
.inspect-view {
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.inspect-view > *,
.inspect-view__stack > *,
.inspect-view__validation > * {
  min-width: 0;
}

.inspect-view__workspace {
  min-width: 0;
  min-height: 0;
  flex: 1 1 auto;
  padding: var(--locus-space-3) var(--locus-space-5) var(--locus-space-5);
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-md);
  background: var(--surface-panel);
}

.inspect-view__tabs {
  width: 100%;
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.inspect-view__tabs :deep(.el-tabs__header) {
  flex: 0 0 auto;
}

.inspect-view__tabs :deep(.el-tabs__content) {
  min-height: 0;
  flex: 1 1 auto;
}

.inspect-view__tabs :deep(.el-tab-pane) {
  height: 100%;
}

.inspect-view__stack,
.inspect-view__validation {
  width: 100%;
  min-width: 0;
  height: 100%;
  display: grid;
  align-content: start;
  gap: var(--locus-space-5);
  overflow-x: hidden;
  overflow-y: auto;
}

.inspect-view__declarations {
  min-width: 0;
  height: 100%;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--locus-space-4);
  overflow: hidden;
}

.inspect-view__statistics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--locus-space-4);
  padding: var(--locus-space-3) 0;
}

@media (max-width: 800px) {
  .inspect-view {
    height: auto;
  }

  .inspect-view__workspace {
    height: 32rem;
    flex: none;
  }
}

@media (max-width: 600px) {
  .inspect-view__workspace {
    padding: var(--locus-space-4);
  }

  .inspect-view__statistics {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
