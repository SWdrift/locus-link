<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus, Observation } from '../domain/locus'
import CopyValue from './CopyValue.vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps<{ status: EvidenceStatus; observation?: Observation }>()
const { t } = useI18n()
const formatTime = (value: string) =>
  value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value)) : '—'
const evidenceText = computed(() =>
  props.observation?.evidence ? JSON.stringify(props.observation.evidence, null, 2) : '',
)
</script>

<template>
  <ElDescriptions class="evidence-details" :column="1" size="small" border>
    <ElDescriptionsItem :label="t('status.state')"><StatusBadge :status="status" /></ElDescriptionsItem>
    <template v-if="observation">
      <ElDescriptionsItem :label="t('details.subject')"><CopyValue :value="observation.subject" /></ElDescriptionsItem>
      <ElDescriptionsItem :label="t('context.vantage')"><CopyValue :value="observation.vantage" /></ElDescriptionsItem>
      <ElDescriptionsItem :label="t('status.provider')"><CopyValue :value="observation.provider" /></ElDescriptionsItem>
      <ElDescriptionsItem :label="t('status.observed')">{{ formatTime(observation.observed_at) }}</ElDescriptionsItem>
      <ElDescriptionsItem :label="t('details.expires')">{{ formatTime(observation.expires_at) }}</ElDescriptionsItem>
      <ElDescriptionsItem v-if="observation.error" :label="t('details.error')">
        <CopyValue :value="observation.error" />
      </ElDescriptionsItem>
      <ElDescriptionsItem v-if="evidenceText" :label="t('details.evidence')">
        <pre>{{ evidenceText }}</pre>
      </ElDescriptionsItem>
    </template>
  </ElDescriptions>
</template>

<style scoped>
.evidence-details pre {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: inherit;
}
</style>
