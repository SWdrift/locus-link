<script setup lang="ts">
import { TopRight } from '@element-plus/icons-vue'
import { MarkerType, Position, VueFlow } from '@vue-flow/core'
import type { Edge, Node } from '@vue-flow/core'
import { useI18n } from 'vue-i18n'
import type { DependencySnapshot } from '../../domain/locus'
import ElkEdge from '../graph/ElkEdge.vue'
import { GRAPH_NODE_HEIGHT, GRAPH_NODE_WIDTH, layoutLayeredGraph, type LayoutEdgeData } from '../graph/graph-layout'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'

const props = defineProps<{ snapshot: DependencySnapshot; selectedScopeID: string }>()
const emit = defineEmits<{ select: [scopeID: string]; open: [scopeID: string] }>()
const { t } = useI18n()

interface DependencyNodeData {
  scopeID: string
  kind: string
  digest: string
  availability: string
  blocked: boolean
  openable: boolean
}

interface DependencyElements {
  nodes: Node<DependencyNodeData>[]
  edges: Edge<LayoutEdgeData>[]
}

const edgeID = (source: string, alias: string, aliasPath: string[]) => `${source}:${alias}:${aliasPath.join('::')}`

function openNode(data: DependencyNodeData) {
  if (data.openable && !data.blocked) emit('open', data.scopeID)
}

const elements = computed<DependencyElements>(() => {
  const nodes: Node<DependencyNodeData>[] = props.snapshot.nodes.map(node => ({
    id: node.scope_id,
    position: { x: 0, y: 0 },
    data: {
      scopeID: node.scope_id,
      kind: node.kind,
      digest: node.content_digest,
      availability: node.availability,
      blocked: false,
      openable: node.openable,
    },
  }))
  const nodeIDs = new Set(nodes.map(node => node.id))
  const blockedTargets = new Map<string, string>()

  for (const edge of props.snapshot.edges) {
    if (edge.status !== 'blocked' || (edge.target_scope_id && nodeIDs.has(edge.target_scope_id))) continue
    const id = edgeID(edge.source_scope_id, edge.alias, edge.alias_path)
    const targetID = `blocked:${id}`
    blockedTargets.set(id, targetID)
    nodes.push({
      id: targetID,
      position: { x: 0, y: 0 },
      data: {
        scopeID: edge.target_scope_id || edge.alias,
        kind: 'blocked',
        digest: '',
        availability: edge.reason || 'blocked',
        blocked: true,
        openable: false,
      },
    })
  }

  const edges: Edge<LayoutEdgeData>[] = props.snapshot.edges.map(edge => {
    const id = edgeID(edge.source_scope_id, edge.alias, edge.alias_path)
    const blocked = edge.status === 'blocked'
    return {
      id,
      source: edge.source_scope_id,
      target: blockedTargets.get(id) ?? edge.target_scope_id ?? edge.source_scope_id,
      type: 'elk',
      data: { label: edge.alias },
      markerEnd: MarkerType.ArrowClosed,
      animated: false,
      style: {
        stroke: blocked ? 'var(--status-failure)' : 'var(--border-strong)',
        strokeWidth: blocked ? 2.2 : 1.6,
        strokeDasharray: blocked ? '6 4' : undefined,
      },
    }
  })
  return { nodes, edges }
})

const layout = shallowRef<DependencyElements>({ nodes: [], edges: [] })
let layoutVersion = 0
async function calculateLayout(value = elements.value) {
  const version = ++layoutVersion
  const positions = await layoutLayeredGraph(
    value.nodes.map(node => node.id),
    value.edges.map(edge => ({ id: edge.id, source: edge.source, target: edge.target })),
  )
  if (version !== layoutVersion) return
  layout.value = {
    nodes: value.nodes.map(node => ({
      ...node,
      position: positions.get(node.id) ?? { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      style: { width: `${GRAPH_NODE_WIDTH}px`, height: `${GRAPH_NODE_HEIGHT}px` },
    })),
    edges: value.edges,
  }
}

watch(elements, calculateLayout, { immediate: true })
defineExpose({ relayout: () => calculateLayout() })
</script>

<template>
  <div class="dependency-canvas">
    <VueFlow
      :nodes="layout.nodes"
      :edges="layout.edges"
      :nodes-connectable="false"
      v-if="layout.nodes.length"
      :nodes-draggable="true"
      :min-zoom="0.3"
      :max-zoom="1.8"
      fit-view-on-init
    >
      <template #node-default="{ data }">
        <button
          class="dependency-canvas__node"
          :class="{
            'dependency-canvas__node--selected': selectedScopeID === data.scopeID,
            'dependency-canvas__node--blocked': data.blocked,
          }"
          type="button"
          :disabled="data.blocked"
          :title="data.openable ? t('locus.openScope') : undefined"
          @click.stop="emit('select', data.scopeID)"
          @dblclick.stop="openNode(data)"
        >
          <span class="dependency-canvas__node-meta">
            <span>{{ data.kind }} · {{ data.availability }}</span>
            <span
              v-if="data.openable"
              class="dependency-canvas__openable"
              role="img"
              :aria-label="t('locus.openable')"
              :title="t('locus.openable')"
            >
              <TopRight />
            </span>
          </span>
          <strong>{{ data.scopeID }}</strong>
          <small v-if="data.digest">{{ data.digest }}</small>
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
.dependency-canvas {
  width: 100%;
  height: 100%;
  min-height: 420px;
  background: var(--surface-page);
}

.dependency-canvas :deep(.vue-flow__node) {
  padding: 0;
  border: 0;
  border-radius: var(--locus-radius-md);
  background: transparent;
  box-shadow: none;
}

.dependency-canvas__node {
  width: 100%;
  height: 100%;
  display: grid;
  align-content: center;
  gap: var(--locus-space-2);
  padding: var(--locus-space-5) var(--locus-space-6);
  overflow: hidden;
  border: 1px solid var(--border-strong);
  border-radius: var(--locus-radius-md);
  color: var(--text-primary);
  text-align: left;
  background: var(--surface-raised);
  cursor: grab;
}

.dependency-canvas__node:hover,
.dependency-canvas__node:focus-visible {
  border-color: var(--accent);
}

.dependency-canvas__node:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.dependency-canvas__node--selected {
  border-color: var(--accent);
  box-shadow: inset 0 0 0 1px var(--accent);
}

.dependency-canvas__node--blocked {
  border-color: var(--status-failure);
  border-style: dashed;
  cursor: not-allowed;
}

.dependency-canvas__node:active {
  cursor: grabbing;
}

.dependency-canvas__node-meta {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--locus-space-2);
  color: var(--text-muted);
  font-size: var(--locus-font-size-xs);
  line-height: var(--locus-line-height-tight);
}

.dependency-canvas__node-meta > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dependency-canvas__openable {
  width: calc(var(--locus-icon-size-md) - var(--locus-space-3));
  height: calc(var(--locus-icon-size-md) - var(--locus-space-3));
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  color: var(--status-success);
}

.dependency-canvas__openable svg {
  width: 100%;
  height: 100%;
}

.dependency-canvas__node strong,
.dependency-canvas__node small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dependency-canvas__node strong {
  font-size: var(--locus-font-size-base);
}

.dependency-canvas__node small {
  color: var(--text-muted);
  font-size: var(--locus-font-size-xs);
}
</style>
