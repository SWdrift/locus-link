<script setup lang="ts">
import { BaseEdge, EdgeLabelRenderer } from '@vue-flow/core'
import type { CSSProperties } from 'vue'
import type { LayoutEdgeData } from './graph-layout'

defineProps<{ id: string; data: LayoutEdgeData; style?: CSSProperties; markerEnd?: string }>()
</script>

<template>
  <BaseEdge :id="id" :path="data.path" :style="style" :marker-end="markerEnd" />
  <EdgeLabelRenderer><span class="edge-label" :style="{ transform: 'translate(-50%, -50%) translate(' + data.labelX + 'px, ' + data.labelY + 'px)' }">{{ data.label }}</span></EdgeLabelRenderer>
</template>
