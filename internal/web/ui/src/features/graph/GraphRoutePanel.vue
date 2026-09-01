<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import CopyValue from '../../components/CopyValue.vue'
import type { EvidenceStatus, GraphView, StatusView } from '../../domain/locus'
import StatusBadge from '../../components/StatusBadge.vue'
import { scopePath, useScopeID } from '../../scope-context'

const props = defineProps<{
  graph: GraphView
  status?: StatusView
  operationalReady: boolean
  probing: boolean
}>()
const emit = defineEmits<{ probe: [subject: string] }>()
const selectedRoute = defineModel<string>({ required: true })
const { t } = useI18n()
const scopeID = useScopeID()

const activeRoute = computed(() => props.graph.routes.find(route => route.canonical_id === selectedRoute.value))
const routeStatus = computed<EvidenceStatus>(() => {
  return props.status?.routes.find(item => item.route_id === selectedRoute.value)?.evidence.status ?? 'unknown'
})
</script>

<template>
  <section class="graph-route-panel">
    <label class="graph-route-panel__field">
      <span>{{ t('graph.activeRoute') }}</span>
      <ElSelect v-model="selectedRoute" :aria-label="t('graph.activeRoute')">
        <ElOption
          v-for="route in graph.routes"
          :key="route.canonical_id"
          :label="route.canonical_id"
          :value="route.canonical_id"
        />
      </ElSelect>
    </label>

    <template v-if="activeRoute">
      <div class="graph-route-panel__heading">
        <CopyValue :value="activeRoute.canonical_id" />
        <StatusBadge :status="routeStatus" />
      </div>
      <ElDescriptions :column="1" size="small">
        <ElDescriptionsItem :label="t('graph.scope')"><CopyValue :value="activeRoute.scope_id" /></ElDescriptionsItem>
        <ElDescriptionsItem :label="t('graph.documents')">
          <RouterLink
            v-for="id in activeRoute.documentation_ids ?? []"
            :key="id"
            :to="{ path: scopePath(scopeID, 'knowledge'), query: { document: id } }"
          >
            <ElButton link type="primary">{{ id }}</ElButton>
          </RouterLink>
          <span v-if="!activeRoute.documentation_ids?.length">—</span>
        </ElDescriptionsItem>
      </ElDescriptions>
      <ol class="graph-route-panel__steps">
        <li v-for="step in activeRoute.steps" :key="step"><CopyValue :value="step" /></li>
      </ol>
      <ElButton
        type="primary"
        :loading="probing"
        :disabled="!operationalReady"
        @click="emit('probe', activeRoute.canonical_id)"
      >
        {{ t('graph.probeRoute') }}
      </ElButton>
    </template>
    <ElEmpty v-else :description="t('common.noData')" />
  </section>
</template>

<style scoped>
.graph-route-panel {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-4);
  padding: var(--locus-space-1) 0;
}

.graph-route-panel__field {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-1);
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.graph-route-panel__heading {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--locus-space-3);
}

.graph-route-panel__heading strong {
  min-width: 0;
}

.graph-route-panel__steps {
  display: grid;
  gap: var(--locus-space-2);
  margin: 0;
  padding-left: var(--locus-space-9);
  color: var(--text-secondary);
  font-size: var(--locus-font-size-md);
}
</style>
