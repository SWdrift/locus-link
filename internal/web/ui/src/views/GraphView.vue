<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import dagre from '@dagrejs/dagre'
import { computed, ref, watch } from 'vue'
import { VueFlow, type Edge, type Node } from '@vue-flow/core'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import type { GraphView, LocusContext } from '../api'
import { getGraph, getStatus, probe, resolveRoute } from '../api'

const props = defineProps<{ context?: LocusContext; from: string; vantage: string }>()
const graph = useQuery({ queryKey: ['graph'], queryFn: getGraph })
const status = useQuery({ queryKey: computed(() => ['status', props.vantage]), queryFn: () => getStatus(props.vantage), enabled: computed(() => Boolean(props.vantage)) })
const queryClient = useQueryClient()
const selectedRoute = ref('')
const selectedLink = ref('')
const target = ref('')
const capability = ref('shell')
watch(() => graph.data.value, (value) => {
  if (!value) return
  if (!selectedRoute.value && value.routes.length) selectedRoute.value = value.routes[0].canonical_id
  if (!target.value) target.value = value.bindings[0]?.role ?? value.entities[0]?.canonical_id ?? ''
}, { immediate: true })

const linkStatus = computed(() => new Map((status.data.value?.links ?? []).map(item => [item.link_id, item.status])))
const activeRoute = computed(() => graph.data.value?.routes.find(route => route.canonical_id === selectedRoute.value))
const activeSteps = computed(() => new Set(activeRoute.value?.steps ?? []))

function layout(value: GraphView): { nodes: Node[]; edges: Edge[] } {
  const model = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}))
  model.setGraph({ rankdir: 'LR', ranksep: 110, nodesep: 70, marginx: 45, marginy: 45 })
  for (const entity of value.entities) model.setNode(entity.canonical_id, { width: 190, height: 78 })
  for (const link of value.links) model.setEdge(link.from, link.to)
  dagre.layout(model)
  const nodes: Node[] = value.entities.map(entity => {
    const point = model.node(entity.canonical_id)
    return {
      id: entity.canonical_id,
      position: { x: point.x - 95, y: point.y - 39 },
      data: { label: entity.name, kind: entity.kind, scope: entity.scope_id, docs: entity.documentation_ids?.length ?? 0 },
      type: 'default',
    }
  })
  const edges: Edge[] = value.links.map(link => {
    const state = linkStatus.value.get(link.canonical_id) ?? 'unknown'
    const active = activeSteps.value.has(link.canonical_id)
    const colors: Record<string, string> = { success: '#59d596', failure: '#f47b73', stale: '#e7b45f', unknown: '#63748a' }
    return {
      id: link.canonical_id,
      source: link.from,
      target: link.to,
      label: link.provider,
      animated: active,
      style: { stroke: active ? '#79aef3' : colors[state], strokeWidth: active ? 3 : 1.7 },
      labelStyle: { fill: '#9eabba', fontSize: 11 },
      data: link,
    }
  })
  return { nodes, edges }
}

const flow = computed(() => graph.data.value ? layout(graph.data.value) : { nodes: [], edges: [] })
const selectedLinkView = computed(() => graph.data.value?.links.find(link => link.canonical_id === selectedLink.value))
const resolveMutation = useMutation({ mutationFn: () => resolveRoute(target.value, capability.value, props.from, props.vantage) })
const probeMutation = useMutation({
  mutationFn: (subject: string) => probe(subject, props.from, props.vantage),
  onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['status'] }) },
})
</script>

<template>
  <section class="graph-page">
    <div class="graph-toolbar">
      <div><span class="eyebrow">Declared view</span><h2>Operational graph</h2></div>
      <div class="summary-pills">
        <span>{{ graph.data.value?.entities.length ?? 0 }} entities</span>
        <span>{{ graph.data.value?.links.length ?? 0 }} links</span>
        <span>{{ graph.data.value?.routes.length ?? 0 }} routes</span>
      </div>
    </div>

    <div v-if="graph.isError.value" class="error-panel">{{ graph.error.value?.message }}</div>
    <div v-else class="graph-workspace">
      <div class="flow-panel">
        <VueFlow :nodes="flow.nodes" :edges="flow.edges" :min-zoom="0.3" :max-zoom="1.8" fit-view-on-init @edge-click="selectedLink = $event.edge.id">
          <template #node-default="{ data }">
            <div class="entity-node">
              <span>{{ data.kind }}</span><strong>{{ data.label }}</strong><small>{{ data.scope }}</small>
              <i v-if="data.docs">{{ data.docs }} doc</i>
            </div>
          </template>
        </VueFlow>
      </div>

      <aside class="inspector-panel">
        <section>
          <span class="eyebrow">Routes</span>
          <button v-for="route in graph.data.value?.routes" :key="route.canonical_id" class="route-choice" :class="{ active: selectedRoute === route.canonical_id }" @click="selectedRoute = route.canonical_id">
            <strong>{{ route.canonical_id.split('::').at(-1) }}</strong><small>{{ route.steps.length }} steps · {{ status.data.value?.routes.find(item => item.route_id === route.canonical_id)?.evidence.status ?? 'unknown' }}</small>
          </button>
          <button v-if="activeRoute" class="probe-button" :disabled="probeMutation.isPending.value || !from" @click="probeMutation.mutate(activeRoute.canonical_id)">Probe route</button>
        </section>

        <section v-if="selectedLinkView" class="selected-detail">
          <span class="eyebrow">Selected link</span><strong>{{ selectedLinkView.canonical_id }}</strong>
          <small>{{ selectedLinkView.provider }} · {{ linkStatus.get(selectedLinkView.canonical_id) ?? 'unknown' }}</small>
          <button class="probe-button" :disabled="probeMutation.isPending.value || !from" @click="probeMutation.mutate(selectedLinkView.canonical_id)">Probe link</button>
        </section>

        <section class="resolve-box">
          <span class="eyebrow">Resolve</span>
          <label>Target<input v-model="target" /></label>
          <label>Capability<input v-model="capability" /></label>
          <button :disabled="resolveMutation.isPending.value || !from || !target || !capability" @click="resolveMutation.mutate()">Resolve route</button>
          <div v-if="resolveMutation.data.value" class="result-card"><strong>{{ resolveMutation.data.value.status }}</strong><small>{{ resolveMutation.data.value.route?.canonical_id ?? `${resolveMutation.data.value.candidates?.length ?? 0} candidates` }}</small></div>
          <div v-if="resolveMutation.isError.value" class="inline-error">{{ resolveMutation.error.value?.message }}</div>
        </section>
        <div v-if="probeMutation.data.value" class="result-card"><strong>Probe {{ probeMutation.data.value.status }}</strong><small>{{ probeMutation.data.value.observations.length }} observations written</small></div>
        <div v-if="probeMutation.isError.value" class="inline-error">{{ probeMutation.error.value?.message }}</div>
      </aside>
    </div>
  </section>
</template>
