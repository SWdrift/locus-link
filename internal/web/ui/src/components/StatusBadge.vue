<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus } from '../api'

const props = defineProps<{ status: EvidenceStatus }>()
const { t } = useI18n()
const label = computed(() => t(`status.${props.status}`))
</script>

<template><span class="state-badge" :class="status">{{ label }}</span></template>
