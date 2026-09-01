<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { useI18n } from 'vue-i18n'
import { getGraph, probe, resolveRoute } from '../api'
import AsyncState from '../components/AsyncState.vue'
import PageHeader from '../components/PageHeader.vue'
import GraphCanvas from '../features/graph/GraphCanvas.vue'
import GraphInspector from '../features/graph/GraphInspector.vue'
import { layoutGraph } from '../features/graph/graph-layout'
import type { GraphLayout } from '../features/graph/graph-layout'
import type { GraphSelection } from '../features/graph/graph-types'
import { useOperationalContext } from '../operational-context'
import { useStatusQuery } from '../queries'

const { t } = useI18n()
const context = useOperationalContext()
const graphQuery = useQuery({ queryKey: ['graph'], queryFn: getGraph })
const statusQuery = useStatusQuery()
const queryClient = useQueryClient()
const selectedRoute = ref('')
const selection = ref<GraphSelection | null>(null)
const target = ref('')
const capability = ref('shell')
const layout = ref<GraphLayout>()
const layoutPending = ref(false)
const layoutError = ref<Error | null>(null)
const layoutRevision = ref(0)
let layoutRequest = 0

async function calculateLayout() {
  const graph = graphQuery.data.value
  if (!graph?.entities.length) return

  const request = ++layoutRequest
  layoutPending.value = true
  layoutError.value = null
  try {
    const result = await layoutGraph(graph)
    if (request !== layoutRequest) return
    layout.value = result
    layoutRevision.value += 1
  } catch (error) {
    if (request === layoutRequest) {
      layoutError.value = error instanceof Error ? error : new Error(String(error))
    }
  } finally {
    if (request === layoutRequest) layoutPending.value = false
  }
}

watch(() => graphQuery.data.value, (graph) => {
  if (!graph) return

  if (!graph.routes.some(route => route.canonical_id === selectedRoute.value)) {
    selectedRoute.value = graph.routes[0]?.canonical_id ?? ''
  }

  const selectionExists = selection.value?.kind === 'entity'
    ? graph.entities.some(entity => entity.canonical_id === selection.value?.id)
    : graph.links.some(link => link.canonical_id === selection.value?.id)
  if (!selectionExists) {
    const firstEntity = graph.entities[0]
    selection.value = firstEntity ? { kind: 'entity', id: firstEntity.canonical_id } : null
  }

  if (!target.value) target.value = graph.bindings[0]?.role ?? graph.entities[0]?.canonical_id ?? ''
  void calculateLayout()
}, { immediate: true })

const activeRoute = computed(() => graphQuery.data.value?.routes.find(route => route.canonical_id === selectedRoute.value))
const activeSteps = computed(() => new Set(activeRoute.value?.steps ?? []))
const linkStatus = computed(() => new Map(statusQuery.data.value?.links.map(item => [item.link_id, item.status]) ?? []))
const empty = computed(() => Boolean(graphQuery.data.value && !graphQuery.data.value.entities.length))
const operationalReady = computed(() => Boolean(context.from && context.vantage))
const resolveMutation = useMutation({
  mutationFn: () => resolveRoute(target.value, capability.value, context.from, context.vantage),
})
const probeMutation = useMutation({
  mutationFn: (subject: string) => probe(subject, context.from, context.vantage),
  onSuccess: async () => {
    await queryClient.invalidateQueries({ queryKey: ['status'] })
  },
})
</script>

<template>
  <section class="graph-view">
    <PageHeader :eyebrow="t('graph.eyebrow')" :title="t('graph.title')">
      <template #actions>
        <div class="graph-view__counts" aria-live="polite">
          <span><strong>{{ graphQuery.data.value?.entities.length ?? 0 }}</strong> {{ t('graph.entities') }}</span>
          <span><strong>{{ graphQuery.data.value?.links.length ?? 0 }}</strong> {{ t('graph.links') }}</span>
          <span><strong>{{ graphQuery.data.value?.routes.length ?? 0 }}</strong> {{ t('graph.routes') }}</span>
        </div>
      </template>
    </PageHeader>

    <AsyncState
      :loading="graphQuery.isPending.value"
      :error="graphQuery.error.value"
      :empty="empty"
      :empty-text="t('graph.empty')"
      :retrying="graphQuery.isFetching.value"
      retryable
      skeleton="graph"
      @retry="graphQuery.refetch()"
    >
      <AsyncState
        :loading="layoutPending"
        :loading-text="t('graph.layout')"
        :error="layoutError"
        retryable
        skeleton="graph"
        @retry="calculateLayout"
      >
        <ElContainer v-if="layout && graphQuery.data.value" class="graph-view__workspace">
        <ElMain class="graph-view__canvas-region">
          <GraphCanvas
            :key="layout.fingerprint + layoutRevision"
            :layout="layout"
            :link-status="linkStatus"
            :active-steps="activeSteps"
            :selection="selection"
            @select="selection = $event"
          />
        </ElMain>
        <ElAside class="graph-view__inspector-region" width="328px">
          <GraphInspector
            v-model:selected-route="selectedRoute"
            v-model:selection="selection"
            v-model:target="target"
            v-model:capability="capability"
            :graph="graphQuery.data.value"
            :status="statusQuery.data.value"
            :operational-ready="operationalReady"
            :probing="probeMutation.isPending.value"
            :resolving="resolveMutation.isPending.value"
            :probe-result="probeMutation.data.value"
            :resolve-result="resolveMutation.data.value"
            :probe-error="probeMutation.error.value"
            :resolve-error="resolveMutation.error.value"
            :status-error="statusQuery.error.value"
            @probe="probeMutation.mutate"
            @resolve="resolveMutation.mutate()"
            @relayout="calculateLayout"
          />
        </ElAside>
      </ElContainer>
      </AsyncState>
    </AsyncState>
  </section>
</template>

<style scoped>
.graph-view {
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.graph-view__counts {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--locus-space-6);
  color: var(--text-muted);
  font-size: var(--locus-font-size-md);
}

.graph-view__counts strong {
  color: var(--text-primary);
  font-size: var(--locus-font-size-base);
}


.graph-view__workspace {
  min-width: 0;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-md);
  background: var(--surface-panel);
}

.graph-view__canvas-region {
  min-width: 0;
  padding: 0;
  overflow: hidden;
}

.graph-view__inspector-region {
  min-width: 0;
  overflow: hidden;
}

@media (max-width: 1100px) {
  .graph-view {
    height: auto;
  }

  .graph-view__workspace {
    min-height: 0;
    display: block;
  }

  .graph-view__canvas-region {
    height: 540px;
  }

  .graph-view__inspector-region {
    width: 100% !important;
    border-top: 1px solid var(--border-subtle);
  }
}

@media (max-width: 600px) {
  .graph-view__counts {
    justify-content: flex-start;
  }

  .graph-view__canvas-region {
    height: 440px;
  }
}
</style>
