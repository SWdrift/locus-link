<script setup lang="ts">
import { VueFlow } from '@vue-flow/core'
import type { Edge } from '@vue-flow/core'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../../domain/locus'
import ElkEdge from './ElkEdge.vue'
import type { GraphLayout, LayoutEdgeData } from './graph-layout'
import type { GraphSelection } from './graph-types'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'

const props = defineProps<{
  layout: GraphLayout
  linkStatus: Map<string, EvidenceStatus>
  activeSteps: Set<string>
  selection: GraphSelection | null
}>()
const emit = defineEmits<{ select: [selection: GraphSelection] }>()
const { t } = useI18n()

const statusColor: Record<EvidenceStatus, string> = {
  success: 'var(--status-success)',
  failure: 'var(--status-failure)',
  stale: 'var(--status-stale)',
  unknown: 'var(--status-unknown)',
}

const edges = computed<Edge<LayoutEdgeData>[]>(() => props.layout.edges.map((edge) => {
  const active = props.activeSteps.has(edge.id)
  const selected = props.selection?.kind === 'link' && edge.id === props.selection.id
  return {
    ...edge,
    animated: active,
    selected,
    style: {
      stroke: active || selected ? 'var(--accent)' : statusColor[props.linkStatus.get(edge.id) ?? 'unknown'],
      strokeWidth: active || selected ? 2.6 : 1.6,
    },
  }
}))
</script>

<template>
  <div class="graph-canvas">
    <VueFlow
      :nodes="layout.nodes"
      :edges="edges"
      :min-zoom="0.3"
      :max-zoom="1.8"
      :nodes-draggable="true"
      :nodes-connectable="false"
      fit-view-on-init
      @edge-click="emit('select', { kind: 'link', id: $event.edge.id })"
    >
      <template #node-default="{ id, data }">
        <button
          class="graph-canvas__node"
          :class="{ 'graph-canvas__node--selected': selection?.kind === 'entity' && selection.id === id }"
          type="button"
          :aria-label="`${data.label} · ${data.canonicalId}`"
          @click.stop="emit('select', { kind: 'entity', id })"
        >
          <span class="graph-canvas__node-meta">
            <span>{{ data.kind }}</span>
            <span v-if="data.documentationCount">{{ data.documentationCount }} {{ t('graph.documents') }}</span>
          </span>
          <strong>{{ data.label }}</strong>
          <small>{{ data.scope }}</small>
        </button>
      </template>
      <template #edge-elk="edgeProps">
        <ElkEdge
          :id="edgeProps.id"
          :data="edgeProps.data"
          :style="edgeProps.style"
          :source-x="edgeProps.sourceX"
          :source-y="edgeProps.sourceY"
          :target-x="edgeProps.targetX"
          :target-y="edgeProps.targetY"
          :source-position="edgeProps.sourcePosition"
          :target-position="edgeProps.targetPosition"
          :marker-end="edgeProps.markerEnd"
        />
      </template>
    </VueFlow>
  </div>
</template>

<style scoped>
.graph-canvas {
  min-width: 0;
  height: 100%;
  background: var(--surface-page);
}

.graph-canvas :deep(.vue-flow) {
  width: 100%;
  height: 100%;
}

.graph-canvas :deep(.vue-flow__node) {
  padding: 0;
  border: 0;
  border-radius: var(--locus-radius-md);
  background: transparent;
  box-shadow: none;
}

.graph-canvas__node {
  width: 196px;
  height: 76px;
  display: grid;
  align-content: center;
  gap: var(--locus-space-2);
  padding: var(--locus-space-5) var(--locus-space-6);
  border: 1px solid var(--border-strong);
  border-radius: var(--locus-radius-md);
  color: var(--text-primary);
  text-align: left;
  background: var(--surface-raised);
  cursor: grab;
}

.graph-canvas__node:hover,
.graph-canvas__node:focus-visible {
  border-color: var(--accent);
}

.graph-canvas__node:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.graph-canvas__node--selected {
  border-color: var(--accent);
  box-shadow: inset 0 0 0 1px var(--accent);
}

.graph-canvas__node:active {
  cursor: grabbing;
}

.graph-canvas__node-meta {
  display: flex;
  justify-content: space-between;
  gap: var(--locus-space-4);
  color: var(--text-muted);
  font-size: var(--locus-font-size-xs);
  line-height: var(--locus-line-height-tight);
}

.graph-canvas__node strong {
  overflow: hidden;
  font-size: var(--locus-font-size-base);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.graph-canvas__node small {
  overflow: hidden;
  color: var(--text-muted);
  font-size: var(--locus-font-size-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
