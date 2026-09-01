<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import CopyValue from '../../components/CopyValue.vue'
import EvidenceDetails from '../../components/EvidenceDetails.vue'
import type { ResolveResult, ResolvedRoute } from '../../domain/locus'

const props = defineProps<{
  operationalReady: boolean
  resolving: boolean
  resolveResult?: ResolveResult
  resolveError?: Error | null
}>()
const emit = defineEmits<{ resolve: [] }>()
const target = defineModel<string>('target', { required: true })
const capability = defineModel<string>('capability', { required: true })
const { t } = useI18n()
const routes = computed<ResolvedRoute[]>(() => {
  if (!props.resolveResult) return []
  return props.resolveResult.route ? [props.resolveResult.route] : props.resolveResult.candidates
})
const alertType = computed(() => (props.resolveResult?.status === 'resolved' ? 'success' : 'warning'))
const command = (route: ResolvedRoute['steps'][number]) =>
  [route.native_hint.executable, ...route.native_hint.args].join(' ')
</script>

<template>
  <section class="graph-resolve-panel">
    <label class="graph-resolve-panel__field">
      <span>{{ t('graph.target') }}</span>
      <ElInput v-model="target" :aria-label="t('graph.target')" />
    </label>
    <label class="graph-resolve-panel__field">
      <span>{{ t('graph.capability') }}</span>
      <ElInput v-model="capability" :aria-label="t('graph.capability')" />
    </label>
    <ElButton
      type="primary"
      :loading="resolving"
      :disabled="!operationalReady || !target || !capability"
      @click="emit('resolve')"
    >
      {{ t('graph.resolveRoute') }}
    </ElButton>

    <template v-if="resolveResult">
      <ElAlert :type="alertType" :closable="false" :title="t(`graph.resolveStatus.${resolveResult.status}`)" />
      <ElDescriptions :column="1" size="small" border>
        <ElDescriptionsItem :label="t('details.inputTarget')"
          ><CopyValue :value="resolveResult.input_target"
        /></ElDescriptionsItem>
        <ElDescriptionsItem :label="t('details.canonicalTarget')"
          ><CopyValue :value="resolveResult.canonical_target"
        /></ElDescriptionsItem>
        <ElDescriptionsItem :label="t('graph.capability')"
          ><CopyValue :value="resolveResult.capability"
        /></ElDescriptionsItem>
        <ElDescriptionsItem v-if="resolveResult.binding" :label="t('details.binding')">
          <CopyValue :value="`${resolveResult.binding.role} → ${resolveResult.binding.target}`" />
        </ElDescriptionsItem>
        <ElDescriptionsItem v-if="resolveResult.target_entity" :label="t('details.targetEntity')">
          <CopyValue :value="`${resolveResult.target_entity.name} (${resolveResult.target_entity.kind})`" />
        </ElDescriptionsItem>
      </ElDescriptions>

      <ElCollapse v-if="routes.length">
        <ElCollapseItem v-for="route in routes" :key="route.canonical_id" :name="route.canonical_id">
          <template #title>
            <span class="technical-id">{{ route.canonical_id }} · {{ route.evidence_status }}</span>
          </template>
          <ElDescriptions :column="1" size="small" border>
            <ElDescriptionsItem :label="t('details.derivedTarget')"
              ><CopyValue :value="route.derived_target"
            /></ElDescriptionsItem>
            <ElDescriptionsItem :label="t('graph.provides')"
              ><CopyValue :value="route.derived_provides.join(', ')"
            /></ElDescriptionsItem>
            <ElDescriptionsItem v-if="route.documentation?.length" :label="t('graph.documents')">
              <RouterLink
                v-for="document in route.documentation"
                :key="document.ref"
                :to="{ path: '/knowledge', query: { search: document.ref } }"
              >
                <ElButton link type="primary">{{ document.title || document.ref }}</ElButton>
              </RouterLink>
            </ElDescriptionsItem>
          </ElDescriptions>
          <ElCollapse class="graph-resolve-panel__steps">
            <ElCollapseItem v-for="step in route.steps" :key="step.link_id" :name="step.link_id" :title="step.link_id">
              <ElDescriptions :column="1" size="small" border>
                <ElDescriptionsItem :label="t('status.provider')"
                  ><CopyValue :value="step.provider"
                /></ElDescriptionsItem>
                <ElDescriptionsItem :label="t('details.nativeHint')"
                  ><CopyValue :value="command(step)"
                /></ElDescriptionsItem>
                <ElDescriptionsItem
                  v-if="step.native_hint.credential_refs?.length"
                  :label="t('details.credentialRefs')"
                >
                  <CopyValue :value="step.native_hint.credential_refs.join(', ')" />
                </ElDescriptionsItem>
                <ElDescriptionsItem v-if="step.documentation?.length" :label="t('graph.documents')">
                  <RouterLink
                    v-for="document in step.documentation"
                    :key="document.ref"
                    :to="{ path: '/knowledge', query: { search: document.ref } }"
                  >
                    <ElButton link type="primary">{{ document.title || document.ref }}</ElButton>
                  </RouterLink>
                </ElDescriptionsItem>
              </ElDescriptions>
              <EvidenceDetails :status="step.evidence.status" :observation="step.evidence.observation" />
            </ElCollapseItem>
          </ElCollapse>
        </ElCollapseItem>
      </ElCollapse>
    </template>
    <ElAlert v-if="resolveError" type="error" :closable="false" :title="resolveError.message" />
  </section>
</template>

<style scoped>
.graph-resolve-panel {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-4);
  padding: var(--locus-space-1) 0;
}

.graph-resolve-panel__field {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-1);
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.graph-resolve-panel__steps {
  margin-top: var(--locus-space-3);
}
</style>
