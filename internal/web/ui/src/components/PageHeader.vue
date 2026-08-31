<script setup lang="ts">
defineProps<{
  eyebrow: string
  title: string
}>()
</script>

<template>
  <header class="locus-page-header">
    <div class="locus-page-header__title">
      <span>{{ eyebrow }}</span>
      <h1>{{ title }}</h1>
    </div>
    <div v-if="$slots.actions" class="locus-page-header__actions"><slot name="actions" /></div>
  </header>
</template>

<style scoped>
.locus-page-header {
  min-width: 0;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--locus-space-6);
  margin-bottom: var(--locus-space-4);
}

.locus-page-header__title {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-1);
}

.locus-page-header__title span {
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.locus-page-header__title h1 {
  font-size: var(--locus-font-size-2xl);
  font-weight: var(--locus-font-weight-strong);
  letter-spacing: -0.015em;
  line-height: var(--locus-line-height-heading);
}

.locus-page-header__actions {
  min-width: 0;
}

@media (max-width: 600px) {
  .locus-page-header {
    align-items: flex-start;
    flex-direction: column;
    gap: var(--locus-space-3);
  }

  .locus-page-header__actions {
    width: 100%;
  }
}
</style>
