<script setup lang="ts">
import { VueFlow } from '@vue-flow/core'
import type { Edge } from '@vue-flow/core'
import { computed } from 'vue'
import type { EvidenceStatus } from '../../api'
import ElkEdge from './ElkEdge.vue'
import type { GraphLayout, LayoutEdgeData } from './graph-layout'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'

const props = defineProps<{ layout: GraphLayout; linkStatus: Map<string, EvidenceStatus>; activeSteps: Set<string>; selectedLink: string }>()
const emit = defineEmits<{ selectLink: [id: string] }>()
const statusColor: Record<EvidenceStatus, string> = {
  success: 'var(--status-success)',
  failure: 'var(--status-failure)',
  stale: 'var(--status-stale)',
  unknown: 'var(--status-unknown)',
}
const edges = computed<Edge<LayoutEdgeData>[]>(() => props.layout.edges.map(edge => {
  const active = props.activeSteps.has(edge.id)
  const selected = edge.id === props.selectedLink
  return {
    ...edge,
    animated: active,
    selected,
    style: { stroke: active || selected ? 'var(--accent)' : statusColor[props.linkStatus.get(edge.id) ?? 'unknown'], strokeWidth: active || selected ? 3 : 1.8 },
  }
}))
</script>

<template>
  <div class="flow-panel">
    <VueFlow :nodes="layout.nodes" :edges="edges" :min-zoom="0.3" :max-zoom="1.8" :nodes-draggable="false" :nodes-connectable="false" fit-view-on-init @edge-click="emit('selectLink', $event.edge.id)">
      <template #node-default="{ data }">
        <div class="entity-node"><span>{{ data.kind }}</span><strong>{{ data.label }}</strong><small>{{ data.scope }}</small><i v-if="data.docs">{{ data.docs }}</i></div>
      </template>
      <template #edge-elk="edgeProps"><ElkEdge :id="edgeProps.id" :data="edgeProps.data" :style="edgeProps.style" :marker-end="edgeProps.markerEnd" /></template>
    </VueFlow>
  </div>
</template>
