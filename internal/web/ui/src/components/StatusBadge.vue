<script setup lang="ts">
import { ElTag } from 'element-plus'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../api'

const props = defineProps<{ status: EvidenceStatus }>()
const { t } = useI18n()
const label = computed(() => t(`status.${props.status}`))
const type = computed(() => ({ success: 'success', failure: 'danger', stale: 'warning', unknown: 'info' } as const)[props.status])
</script>

<template><ElTag :type="type" effect="light" size="small" round>{{ label }}</ElTag></template>
