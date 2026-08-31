<script setup lang="ts">
defineProps<{
  eyebrow: string
  title: string
  description?: string
}>()
</script>

<template>
  <header class="page-header">
    <div class="page-title">
      <span class="eyebrow">{{ eyebrow }}</span>
      <h2>{{ title }}</h2>
    </div>
    <p v-if="description">{{ description }}</p>
    <div v-if="$slots.actions" class="page-actions"><slot name="actions" /></div>
  </header>
</template>
