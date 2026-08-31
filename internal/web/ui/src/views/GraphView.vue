<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElAlert, ElSkeleton, ElTag } from 'element-plus'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getGraph, probe, resolveRoute } from '../api'
import AsyncState from '../components/AsyncState.vue'
import GraphCanvas from '../features/graph/GraphCanvas.vue'
import GraphInspector from '../features/graph/GraphInspector.vue'
import { layoutGraph } from '../features/graph/graph-layout'
import type { GraphLayout } from '../features/graph/graph-layout'
import { useOperationalContext } from '../operational-context'
import { useStatusQuery } from '../queries'
import '../features/graph/graph.css'

const { t } = useI18n()
const context = useOperationalContext()
const graph = useQuery({ queryKey: ['graph'], queryFn: getGraph })
const status = useStatusQuery()
const queryClient = useQueryClient()
const selectedRoute = ref('')
const selectedLink = ref('')
const target = ref('')
const capability = ref('shell')
const layout = ref<GraphLayout>()
const layoutPending = ref(false)
const layoutError = ref<Error | null>(null)
const layoutRevision = ref(0)
let layoutRequest = 0

async function calculateLayout() {
  const value = graph.data.value
  if (!value || !value.entities.length) return
  const request = ++layoutRequest
  layoutPending.value = true
  layoutError.value = null
  try {
    const result = await layoutGraph(value)
    if (request !== layoutRequest) return
    layout.value = result
    layoutRevision.value += 1
  } catch (error) {
    if (request === layoutRequest) layoutError.value = error instanceof Error ? error : new Error(String(error))
  } finally {
    if (request === layoutRequest) layoutPending.value = false
  }
}

watch(() => graph.data.value, value => {
  if (!value) return
  if (!value.routes.some(route => route.canonical_id === selectedRoute.value)) selectedRoute.value = value.routes[0]?.canonical_id ?? ''
  if (!value.links.some(link => link.canonical_id === selectedLink.value)) selectedLink.value = ''
  if (!target.value) target.value = value.bindings[0]?.role ?? value.entities[0]?.canonical_id ?? ''
  void calculateLayout()
}, { immediate: true })

const activeRoute = computed(() => graph.data.value?.routes.find(route => route.canonical_id === selectedRoute.value))
const activeSteps = computed(() => new Set(activeRoute.value?.steps ?? []))
const linkStatus = computed(() => new Map(status.data.value?.links.map(item => [item.link_id, item.status]) ?? []))
const empty = computed(() => Boolean(graph.data.value && !graph.data.value.entities.length))
const operationalReady = computed(() => Boolean(context.from && context.vantage))
const resolveMutation = useMutation({ mutationFn: () => resolveRoute(target.value, capability.value, context.from, context.vantage) })
const probeMutation = useMutation({
  mutationFn: (subject: string) => probe(subject, context.from, context.vantage),
  onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['status'] }) },
})
</script>

<template>
  <section class="graph-page">
    <div class="graph-toolbar">
      <div><span class="eyebrow">{{ t('graph.eyebrow') }}</span><h2>{{ t('graph.title') }}</h2></div>
      <div class="summary-pills" aria-live="polite">
        <ElTag effect="plain" round>{{ graph.data.value?.entities.length ?? 0 }} {{ t('graph.entities') }}</ElTag>
        <ElTag effect="plain" round>{{ graph.data.value?.links.length ?? 0 }} {{ t('graph.links') }}</ElTag>
        <ElTag effect="plain" round>{{ graph.data.value?.routes.length ?? 0 }} {{ t('graph.routes') }}</ElTag>
      </div>
    </div>

    <AsyncState :loading="graph.isPending.value" :error="graph.error.value" :empty="empty" :empty-text="t('graph.empty')">
      <ElSkeleton v-if="layoutPending" :rows="6" animated :aria-label="t('graph.layout')" />
      <ElAlert v-else-if="layoutError" type="error" :closable="false" :title="t('common.error')" :description="layoutError.message" />
      <div v-else-if="layout && graph.data.value" class="graph-workspace">
        <GraphCanvas :key="layout.fingerprint + layoutRevision" :layout="layout" :link-status="linkStatus" :active-steps="activeSteps" :selected-link="selectedLink" @select-link="selectedLink = $event" />
        <GraphInspector
          v-model:selected-route="selectedRoute"
          v-model:selected-link="selectedLink"
          v-model:target="target"
          v-model:capability="capability"
          :graph="graph.data.value"
          :status="status.data.value"
          :operational-ready="operationalReady"
          :probing="probeMutation.isPending.value"
          :resolving="resolveMutation.isPending.value"
          :probe-result="probeMutation.data.value"
          :resolve-result="resolveMutation.data.value"
          :probe-error="probeMutation.error.value"
          :resolve-error="resolveMutation.error.value"
          :status-error="status.error.value"
          @probe="probeMutation.mutate"
          @resolve="resolveMutation.mutate()"
          @relayout="calculateLayout"
        />
      </div>
    </AsyncState>
  </section>
</template>
