<script setup lang="ts">
import { RefreshRight } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import type { GraphView, ProbeResult, ResolveResult, StatusView } from '../../domain/locus'
import GraphResolvePanel from './GraphResolvePanel.vue'
import GraphRoutePanel from './GraphRoutePanel.vue'
import GraphSelectionPanel from './GraphSelectionPanel.vue'
import type { GraphSelection } from './graph-types'

defineProps<{
  graph: GraphView
  status?: StatusView
  operationalReady: boolean
  probing: boolean
  resolving: boolean
  probeResult?: ProbeResult
  resolveResult?: ResolveResult
  probeError?: Error | null
  resolveError?: Error | null
  statusError?: Error | null
}>()
const emit = defineEmits<{ probe: [subject: string]; resolve: []; relayout: [] }>()
const selectedRoute = defineModel<string>('selectedRoute', { required: true })
const selection = defineModel<GraphSelection | null>('selection', { required: true })
const target = defineModel<string>('target', { required: true })
const capability = defineModel<string>('capability', { required: true })
const { t } = useI18n()
const inspectorTab = ref('routes')

watch(selection, () => {
  if (selection.value) inspectorTab.value = 'selection'
})
</script>

<template>
  <aside class="graph-inspector">
    <header class="graph-inspector__header">
      <div>
        <span>{{ t('graph.inspector') }}</span>
        <h2>{{ t('graph.details') }}</h2>
      </div>
      <ElButton text size="small" :icon="RefreshRight" @click="emit('relayout')">{{ t('graph.relayout') }}</ElButton>
    </header>

    <ElTabs v-model="inspectorTab" stretch>
      <ElTabPane :label="t('graph.routes')" name="routes">
        <GraphRoutePanel
          v-model="selectedRoute"
          :graph="graph"
          :status="status"
          :operational-ready="operationalReady"
          :probing="probing"
          @probe="emit('probe', $event)"
        />
      </ElTabPane>

      <ElTabPane :label="t('graph.selection')" name="selection">
        <GraphSelectionPanel
          v-model="selection"
          :graph="graph"
          :status="status"
          :operational-ready="operationalReady"
          :probing="probing"
          @probe="emit('probe', $event)"
        />
      </ElTabPane>

      <ElTabPane :label="t('graph.resolve')" name="resolve">
        <GraphResolvePanel
          v-model:target="target"
          v-model:capability="capability"
          :operational-ready="operationalReady"
          :resolving="resolving"
          :resolve-result="resolveResult"
          :resolve-error="resolveError"
          @resolve="emit('resolve')"
        />
      </ElTabPane>
    </ElTabs>

    <ElAlert
      v-if="probeResult"
      type="success"
      :closable="false"
      :title="t('graph.probeResult', { status: t(`status.${probeResult.status}`) })"
      :description="t('graph.observationsWritten', { count: probeResult.observations.length })"
    />
    <ElAlert v-if="probeError" type="error" :closable="false" :title="probeError.message" />
    <ElAlert v-if="statusError" type="error" :closable="false" :title="statusError.message" />
  </aside>
</template>

<style scoped>
.graph-inspector {
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--locus-space-4);
  padding: var(--locus-space-5);
  overflow: auto;
  border-left: 1px solid var(--border-subtle);
  background: var(--surface-panel);
}

.graph-inspector__header {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--locus-space-3);
}

.graph-inspector__header span {
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.graph-inspector__header h2 {
  margin-top: var(--locus-space-1);
  font-size: var(--locus-font-size-base);
  font-weight: var(--locus-font-weight-strong);
}
</style>
