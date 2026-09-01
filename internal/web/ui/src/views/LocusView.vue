<script setup lang="ts">
import { RefreshRight } from '@element-plus/icons-vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import 'element-plus/theme-chalk/el-message.css'
import { useI18n } from 'vue-i18n'
import { getDependencies, getLocusScopes, refreshDependencies } from '../api'
import AsyncState from '../components/AsyncState.vue'
import CopyValue from '../components/CopyValue.vue'
import GraphWorkspace from '../components/GraphWorkspace.vue'
import PageHeader from '../components/PageHeader.vue'
import type { DependencySnapshot } from '../domain/locus'
import DependencyCanvas from '../features/dependencies/DependencyCanvas.vue'
import { scopePath } from '../scope-context'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const catalogQuery = useQuery({ queryKey: ['locus-scopes'], queryFn: getLocusScopes })
const rootScopeID = ref(typeof route.query.root === 'string' ? route.query.root : '')
const selectedScopeID = ref(typeof route.query.selected === 'string' ? route.query.selected : '')
const search = ref('')
const issuesOnly = ref(false)
const tab = computed(() => (route.path === '/locus/dependencies' ? 'dependencies' : 'catalog'))

const openableScopes = computed(() => (catalogQuery.data.value?.scopes ?? []).filter(scope => scope.openable))
watch(openableScopes, scopes => {
  if (rootScopeID.value && !scopes.some(scope => scope.scope_id === rootScopeID.value)) {
    rootScopeID.value = ''
  }
})

watch(rootScopeID, value => {
  if (tab.value !== 'dependencies') return
  void router.replace({
    path: '/locus/dependencies',
    query: { ...route.query, root: value || undefined, selected: undefined },
  })
  selectedScopeID.value = value
})

function mergeDependencySnapshots(snapshots: DependencySnapshot[]): DependencySnapshot {
  const nodes = new Map<string, DependencySnapshot['nodes'][number]>()
  const edges = new Map<string, DependencySnapshot['edges'][number]>()
  for (const snapshot of snapshots) {
    for (const node of snapshot.nodes) {
      const aliasPaths = node.alias_paths.map(path => [snapshot.root_scope_id, ...path])
      const existing = nodes.get(node.scope_id)
      nodes.set(node.scope_id, {
        ...(existing ?? node),
        root: Boolean(existing?.root || node.root),
        alias_paths: [...(existing?.alias_paths ?? []), ...aliasPaths],
      })
    }
    for (const edge of snapshot.edges) {
      const key = `${edge.source_scope_id}:${edge.alias}:${edge.target_scope_id ?? ''}:${edge.status}`
      if (!edges.has(key)) edges.set(key, { ...edge, alias_path: [snapshot.root_scope_id, ...edge.alias_path] })
    }
  }
  return {
    root_scope_id: '',
    root_digest: '',
    snapshot_digest: snapshots.map(snapshot => snapshot.snapshot_digest).join(':'),
    collected_at: snapshots.reduce(
      (latest, snapshot) => (snapshot.collected_at > latest ? snapshot.collected_at : latest),
      '',
    ),
    completeness: snapshots.some(snapshot => snapshot.completeness === 'partial') ? 'partial' : 'complete',
    nodes: [...nodes.values()],
    edges: [...edges.values()],
    blocked_imports: snapshots.flatMap(snapshot => snapshot.blocked_imports),
  }
}

const dependencyQuery = useQuery({
  queryKey: computed(() => [
    'dependencies',
    rootScopeID.value || 'all',
    ...openableScopes.value.map(scope => scope.scope_id),
  ]),
  queryFn: async () => {
    if (rootScopeID.value) return getDependencies(rootScopeID.value)
    const snapshots = await Promise.all(openableScopes.value.map(scope => getDependencies(scope.scope_id)))
    return mergeDependencySnapshots(snapshots)
  },
  enabled: computed(() => Boolean(openableScopes.value.length && tab.value === 'dependencies')),
})

watch(
  () => dependencyQuery.data.value,
  snapshot => {
    if (!snapshot) return
    if (!snapshot.nodes.some(node => node.scope_id === selectedScopeID.value)) {
      selectedScopeID.value = snapshot.root_scope_id
    }
  },
)

const isAliasPathPrefix = (prefix: string[], path: string[]) =>
  prefix.length <= path.length && prefix.every((segment, index) => segment === path[index])

function dependencySubgraph(snapshot: DependencySnapshot, relevantPaths: string[][], includeBlocked: boolean) {
  return {
    ...snapshot,
    nodes: snapshot.nodes.filter(
      node =>
        node.scope_id === snapshot.root_scope_id ||
        node.alias_paths.some(nodePath => relevantPaths.some(path => isAliasPathPrefix(nodePath, path))),
    ),
    edges: snapshot.edges.filter(
      edge =>
        (edge.status === 'active' || includeBlocked) &&
        relevantPaths.some(path => isAliasPathPrefix(edge.alias_path, path)),
    ),
  }
}

const displayedSnapshot = computed<DependencySnapshot | undefined>(() => {
  const snapshot = dependencyQuery.data.value
  if (!snapshot) return undefined
  const term = search.value.trim().toLocaleLowerCase()
  if (issuesOnly.value) {
    const problemPaths = snapshot.edges.filter(edge => edge.status === 'blocked').map(edge => edge.alias_path)
    return dependencySubgraph(snapshot, problemPaths, true)
  }
  if (term) {
    const matchPaths = snapshot.nodes
      .filter(node => node.scope_id.toLocaleLowerCase().includes(term))
      .flatMap(node => node.alias_paths)
    return dependencySubgraph(snapshot, matchPaths, true)
  }
  return snapshot
})

const selectedNode = computed(() =>
  dependencyQuery.data.value?.nodes.find(node => node.scope_id === selectedScopeID.value),
)
const dependencyCanvas = ref<{ relayout: () => void }>()
const confirmationVisible = ref(false)
const refreshMutation = useMutation({
  mutationFn: (input: { allow: boolean; expected?: string }) =>
    refreshDependencies(rootScopeID.value, input.allow, input.expected ?? ''),
  onSuccess: async result => {
    if (result.status === 'confirmation_required') {
      confirmationVisible.value = true
      return
    }
    confirmationVisible.value = false
    ElMessage({
      type: result.status === 'success' ? 'success' : 'warning',
      message: t(`locus.refreshStatus.${result.status}`),
    })
    await queryClient.invalidateQueries({ queryKey: ['dependencies', rootScopeID.value] })
    await queryClient.invalidateQueries({ queryKey: ['context', rootScopeID.value] })
    await queryClient.invalidateQueries({ queryKey: ['validation', rootScopeID.value] })
  },
  onError: error => ElMessage.error(error instanceof Error ? error.message : t('common.error')),
})
const refreshResult = computed(() =>
  refreshMutation.data.value?.status === 'confirmation_required' ? refreshMutation.data.value : undefined,
)

function inspectDependencies(scopeID: string) {
  rootScopeID.value = scopeID
  void router.push({ path: '/locus/dependencies', query: { root: scopeID } })
}

function selectNode(scopeID: string) {
  selectedScopeID.value = scopeID
  void router.replace({
    path: '/locus/dependencies',
    query: { ...route.query, root: rootScopeID.value, selected: scopeID },
  })
}

function openScope(scopeID: string) {
  void router.push(scopePath(scopeID, 'graph'))
}

function activateCandidate() {
  refreshMutation.mutate({ allow: true, expected: refreshResult.value?.candidate_snapshot?.snapshot_digest })
}
</script>

<template>
  <section class="locus-view">
    <PageHeader :eyebrow="t('locus.title')" :title="tab === 'catalog' ? t('locus.catalog') : t('locus.dependencies')" />
    <template v-if="tab === 'catalog'">
      <AsyncState
        :loading="catalogQuery.isPending.value"
        :error="catalogQuery.error.value"
        retryable
        @retry="catalogQuery.refetch()"
      >
        <ElTable v-if="catalogQuery.data.value" :data="catalogQuery.data.value.scopes" height="100%">
          <ElTableColumn prop="scope_id" :label="t('graph.scope')" min-width="210">
            <template #default="{ row }"><CopyValue :value="row.scope_id" /></template>
          </ElTableColumn>
          <ElTableColumn prop="kind" :label="t('locus.kind')" width="110" />
          <ElTableColumn prop="availability" :label="t('locus.availability')" width="150">
            <template #default="{ row }">
              <ElTag :type="row.availability === 'available' ? 'success' : 'danger'" effect="plain">
                {{ t(`locus.availabilityValues.${row.availability}`) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn prop="registry_path" :label="t('inspect.registryPath')" min-width="280">
            <template #default="{ row }"><CopyValue :value="row.registry_path" /></template>
          </ElTableColumn>
          <ElTableColumn :label="t('locus.actions')" width="210" fixed="right">
            <template #default="{ row }">
              <ElButton link type="primary" :disabled="!row.openable" @click="inspectDependencies(row.scope_id)">
                {{ t('locus.dependencies') }}
              </ElButton>
              <RouterLink v-if="row.openable" :to="scopePath(row.scope_id, 'graph')">
                <ElButton link type="primary">{{ t('locus.open') }}</ElButton>
              </RouterLink>
            </template>
          </ElTableColumn>
        </ElTable>
      </AsyncState>
    </template>

    <template v-else>
      <div class="locus-view__dependency-toolbar">
        <ElSelect v-model="rootScopeID" :aria-label="t('locus.rootScope')" class="locus-view__root-select">
          <ElOption :label="t('locus.allScopes')" value="" />
          <ElOption
            v-for="scope in openableScopes"
            :key="scope.scope_id"
            :label="scope.scope_id"
            :value="scope.scope_id"
          />
        </ElSelect>
        <ElInput v-model="search" clearable :placeholder="t('locus.search')" class="locus-view__search" />
        <ElCheckbox v-model="issuesOnly">{{ t('locus.issuesOnly') }}</ElCheckbox>
        <ElButton
          type="primary"
          :loading="refreshMutation.isPending.value"
          :disabled="!rootScopeID"
          @click="refreshMutation.mutate({ allow: false })"
        >
          {{ t('locus.refresh') }}
        </ElButton>
      </div>

      <ElDialog
        v-model="confirmationVisible"
        :title="t('locus.confirmationRequired')"
        width="min(480px, calc(100vw - 32px))"
        append-to-body
      >
        <p>{{ t('locus.confirmationHint') }}</p>
        <ElDescriptions v-if="refreshResult?.diff" :column="1" size="small" border>
          <ElDescriptionsItem :label="t('locus.nodeChanges')">{{ refreshResult.diff.nodes.length }}</ElDescriptionsItem>
          <ElDescriptionsItem :label="t('locus.edgeChanges')">{{ refreshResult.diff.edges.length }}</ElDescriptionsItem>
          <ElDescriptionsItem :label="t('locus.newBlocked')">{{
            refreshResult.diff.new_blocked_imports.length
          }}</ElDescriptionsItem>
        </ElDescriptions>
        <template #footer>
          <ElButton @click="confirmationVisible = false">{{ t('common.cancel') }}</ElButton>
          <ElButton type="warning" :loading="refreshMutation.isPending.value" @click="activateCandidate">
            {{ t('locus.activateCandidate') }}
          </ElButton>
        </template>
      </ElDialog>

      <AsyncState
        :loading="dependencyQuery.isPending.value"
        :error="dependencyQuery.error.value"
        retryable
        @retry="dependencyQuery.refetch()"
      >
        <GraphWorkspace v-if="displayedSnapshot">
          <template #canvas>
            <DependencyCanvas
              ref="dependencyCanvas"
              :snapshot="displayedSnapshot"
              :selected-scope-i-d="selectedScopeID"
              @select="selectNode"
              @open="openScope"
            />
          </template>
          <template #inspector>
            <aside class="locus-view__dependency-details">
              <header class="locus-view__dependency-details-header">
                <div>
                  <span>{{ t('graph.inspector') }}</span>
                  <h2>{{ t('graph.details') }}</h2>
                </div>
                <ElButton text size="small" :icon="RefreshRight" @click="dependencyCanvas?.relayout()">
                  {{ t('graph.relayout') }}
                </ElButton>
              </header>
              <template v-if="selectedNode">
                <div class="locus-view__detail-heading">
                  <ElTag effect="plain">{{ selectedNode.kind }}</ElTag>
                  <ElTag :type="selectedNode.availability === 'available' ? 'success' : 'danger'" effect="plain">
                    {{ t(`locus.availabilityValues.${selectedNode.availability}`) }}
                  </ElTag>
                </div>
                <CopyValue :value="selectedNode.scope_id" />
                <ElDescriptions :column="1" size="small" border>
                  <ElDescriptionsItem :label="t('inspect.digest')"
                    ><CopyValue :value="selectedNode.content_digest"
                  /></ElDescriptionsItem>
                  <ElDescriptionsItem :label="t('inspect.sourceKind')">{{
                    selectedNode.source.kind
                  }}</ElDescriptionsItem>
                  <ElDescriptionsItem :label="t('inspect.sourceUri')"
                    ><CopyValue :value="selectedNode.source.uri"
                  /></ElDescriptionsItem>
                  <ElDescriptionsItem :label="t('inspect.aliases')">
                    <div v-for="path in selectedNode.alias_paths" :key="path.join('::')" class="technical-id">
                      {{ path.join('::') || 'root' }}
                    </div>
                  </ElDescriptionsItem>
                </ElDescriptions>
                <RouterLink v-if="selectedNode.openable" :to="scopePath(selectedNode.scope_id, 'graph')">
                  <ElButton type="primary">{{ t('locus.openScope') }}</ElButton>
                </RouterLink>
              </template>
              <ElEmpty v-else :description="t('locus.selectNode')" />
            </aside>
          </template>
        </GraphWorkspace>
      </AsyncState>
    </template>
  </section>
</template>

<style scoped>
.locus-view {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.locus-view__dependency-toolbar {
  display: flex;
  align-items: center;
  gap: var(--locus-space-4);
  margin-bottom: var(--locus-space-4);
}

.locus-view__root-select {
  width: 220px;
}

.locus-view__search {
  width: min(280px, 30vw);
}

.locus-view__dependency-details {
  min-width: 0;
  height: 100%;
  display: grid;
  align-content: start;
  gap: var(--locus-space-5);
  padding: var(--locus-space-6);
  overflow: auto;
  border-left: 1px solid var(--border-subtle);
}

.locus-view__dependency-details-header {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--locus-space-3);
}

.locus-view__dependency-details-header span {
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.locus-view__dependency-details-header h2 {
  margin-top: var(--locus-space-1);
  font-size: var(--locus-font-size-base);
  font-weight: var(--locus-font-weight-strong);
}

.locus-view__detail-heading {
  display: flex;
  flex-wrap: wrap;
  gap: var(--locus-space-2);
}

@media (max-width: 1100px) {
  .locus-view__dependency-toolbar {
    align-items: stretch;
    flex-wrap: wrap;
  }

  .locus-view__root-select,
  .locus-view__search {
    width: min(100%, 320px);
  }

  .locus-view__dependency-details {
    border-left: 0;
  }
}
</style>
