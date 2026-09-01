<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { EntityView, GraphView, LinkView, StatusView } from '../../domain/locus'
import CopyValue from '../../components/CopyValue.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import type { GraphSelection } from './graph-types'
import { scopePath, useScopeID } from '../../scope-context'

const props = defineProps<{
  graph: GraphView
  status?: StatusView
  operationalReady: boolean
  probing: boolean
}>()
const emit = defineEmits<{ probe: [subject: string] }>()
const selection = defineModel<GraphSelection | null>({ required: true })
const { t } = useI18n()
const scopeID = useScopeID()

const selectedEntity = computed<EntityView | undefined>(() => {
  if (selection.value?.kind !== 'entity') return undefined
  return props.graph.entities.find(entity => entity.canonical_id === selection.value?.id)
})
const selectedLink = computed<LinkView | undefined>(() => {
  if (selection.value?.kind !== 'link') return undefined
  return props.graph.links.find(link => link.canonical_id === selection.value?.id)
})
const linkStatus = computed(() => new Map(props.status?.links.map(item => [item.link_id, item.status]) ?? []))
</script>

<template>
  <section v-if="selectedEntity" class="graph-selection-panel">
    <div>
      <span class="graph-selection-panel__kind">{{ selectedEntity.kind }}</span>
      <h3>{{ selectedEntity.name }}</h3>
    </div>
    <ElDescriptions :column="1" size="small">
      <ElDescriptionsItem :label="t('graph.canonicalId')">
        <CopyValue :value="selectedEntity.canonical_id" />
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="t('graph.scope')"><CopyValue :value="selectedEntity.scope_id" /></ElDescriptionsItem>
      <ElDescriptionsItem v-if="selectedEntity.labels" :label="t('details.labels')">
        <CopyValue :value="JSON.stringify(selectedEntity.labels)" />
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="t('graph.documents')">
        <RouterLink
          v-for="id in selectedEntity.documentation_ids ?? []"
          :key="id"
          :to="{ path: scopePath(scopeID, 'knowledge'), query: { document: id } }"
        >
          <ElButton link type="primary">{{ id }}</ElButton>
        </RouterLink>
        <span v-if="!selectedEntity.documentation_ids?.length">—</span>
      </ElDescriptionsItem>
    </ElDescriptions>
  </section>

  <section v-else-if="selectedLink" class="graph-selection-panel">
    <div class="graph-selection-panel__heading">
      <div>
        <span class="graph-selection-panel__kind">{{ t('graph.link') }}</span>
        <h3 class="technical-id">{{ selectedLink.canonical_id }}</h3>
      </div>
      <StatusBadge :status="linkStatus.get(selectedLink.canonical_id) ?? 'unknown'" />
    </div>
    <ElDescriptions :column="1" size="small">
      <ElDescriptionsItem :label="t('graph.from')">
        <CopyValue :value="selectedLink.from" />
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="t('graph.to')">
        <CopyValue :value="selectedLink.to" />
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="t('graph.scope')"><CopyValue :value="selectedLink.scope_id" /></ElDescriptionsItem>
      <ElDescriptionsItem :label="t('status.provider')"
        ><CopyValue :value="selectedLink.provider"
      /></ElDescriptionsItem>
      <ElDescriptionsItem :label="t('graph.requires')">
        {{ selectedLink.requires?.join(', ') || '—' }}
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="t('graph.provides')">
        {{ selectedLink.provides?.join(', ') || '—' }}
      </ElDescriptionsItem>
      <ElDescriptionsItem :label="t('graph.documents')">
        <RouterLink
          v-for="id in selectedLink.documentation_ids ?? []"
          :key="id"
          :to="{ path: scopePath(scopeID, 'knowledge'), query: { document: id } }"
        >
          <ElButton link type="primary">{{ id }}</ElButton>
        </RouterLink>
        <span v-if="!selectedLink.documentation_ids?.length">—</span>
      </ElDescriptionsItem>
    </ElDescriptions>
    <ElButton
      type="primary"
      :loading="probing"
      :disabled="!operationalReady"
      @click="emit('probe', selectedLink.canonical_id)"
    >
      {{ t('graph.probeLink') }}
    </ElButton>
  </section>

  <ElEmpty v-else :description="t('graph.selectObject')" />
</template>

<style scoped>
.graph-selection-panel {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-4);
  padding: var(--locus-space-1) 0;
}

.graph-selection-panel__heading {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--locus-space-3);
}

.graph-selection-panel__kind {
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.graph-selection-panel h3 {
  margin-top: var(--locus-space-1);
  font-size: var(--locus-font-size-base);
  font-weight: var(--locus-font-weight-strong);
}
</style>
