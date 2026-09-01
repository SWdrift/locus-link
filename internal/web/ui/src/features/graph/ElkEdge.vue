<script setup lang="ts">
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath } from '@vue-flow/core'
import type { Position } from '@vue-flow/core'
import type { CSSProperties } from 'vue'
import type { LayoutEdgeData } from './graph-layout'

const props = defineProps<{
  id: string
  data: LayoutEdgeData
  sourceX: number
  sourceY: number
  targetX: number
  targetY: number
  sourcePosition: Position
  targetPosition: Position
  style?: CSSProperties
  markerEnd?: string
}>()
const edgePath = computed(() =>
  getSmoothStepPath({
    sourceX: props.sourceX,
    sourceY: props.sourceY,
    targetX: props.targetX,
    targetY: props.targetY,
    sourcePosition: props.sourcePosition,
    targetPosition: props.targetPosition,
  }),
)
</script>

<template>
  <BaseEdge :id="id" :path="edgePath[0]" :style="style" :marker-end="markerEnd" />
  <EdgeLabelRenderer>
    <span
      class="graph-edge__label"
      :style="{ transform: `translate(-50%, -50%) translate(${edgePath[1]}px, ${edgePath[2]}px)` }"
    >
      {{ data.label }}
    </span>
  </EdgeLabelRenderer>
</template>

<style scoped>
.graph-edge__label {
  position: absolute;
  z-index: 2;
  padding: var(--locus-space-1) var(--locus-space-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-sm);
  color: var(--text-secondary);
  background: var(--surface-panel);
  font-size: var(--locus-font-size-xs);
  line-height: var(--locus-line-height-tight);
  pointer-events: none;
}
</style>
