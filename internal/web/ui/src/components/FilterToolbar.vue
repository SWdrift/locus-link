<script setup lang="ts">
import { Search } from '@element-plus/icons-vue'

withDefaults(
  defineProps<{
    searchPlaceholder: string
    searchLabel: string
    filterLabel: string
    options: Array<{ value: string; label: string }>
    layout?: 'inline' | 'stacked'
  }>(),
  {
    layout: 'inline',
  },
)

const search = defineModel<string>('search', { required: true })
const filter = defineModel<string>('filter', { required: true })
</script>

<template>
  <div class="filter-toolbar" :class="`filter-toolbar--${layout}`">
    <ElInput v-model="search" clearable size="small" :placeholder="searchPlaceholder" :aria-label="searchLabel">
      <template #prefix>
        <ElIcon><Search /></ElIcon>
      </template>
    </ElInput>
    <ElSelect v-model="filter" size="small" :aria-label="filterLabel">
      <ElOption v-for="option in options" :key="option.value" :label="option.label" :value="option.value" />
    </ElSelect>
  </div>
</template>

<style scoped>
.filter-toolbar {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(220px, 320px) 148px;
  gap: var(--locus-space-3);
}

.filter-toolbar--stacked {
  grid-template-columns: minmax(0, 1fr);
}

@media (max-width: 767px) {
  .filter-toolbar--inline {
    grid-template-columns: minmax(0, 1fr) 132px;
  }
}

@media (max-width: 479px) {
  .filter-toolbar--inline {
    grid-template-columns: 1fr;
  }
}
</style>
