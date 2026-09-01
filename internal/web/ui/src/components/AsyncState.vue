<script setup lang="ts">
import { Loading, RefreshRight } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

withDefaults(
  defineProps<{
    loading: boolean
    error?: Error | null
    empty?: boolean
    emptyText?: string
    loadingText?: string
    retryable?: boolean
    retrying?: boolean
    skeleton?: 'app' | 'graph' | 'status' | 'knowledge' | 'document'
  }>(),
  {
    error: null,
    empty: false,
    emptyText: '',
    loadingText: '',
    retryable: false,
    retrying: false,
    skeleton: 'app',
  },
)
const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>

<template>
  <section
    v-if="loading"
    class="locus-async-state locus-async-state--loading"
    role="status"
    aria-live="polite"
    :aria-label="loadingText || t('common.loading')"
  >
    <div class="locus-async-state__feedback">
      <ElIcon class="is-loading"><Loading /></ElIcon>
      <span>{{ loadingText || t('common.loading') }}</span>
    </div>

    <ElSkeleton animated>
      <template #template>
        <div v-if="skeleton === 'status'" class="locus-async-state__status-skeleton">
          <div class="locus-async-state__filter-skeleton">
            <ElSkeletonItem variant="rect" />
            <ElSkeletonItem variant="rect" />
          </div>
          <div class="locus-async-state__stat-skeleton">
            <ElSkeletonItem v-for="index in 4" :key="index" variant="rect" />
          </div>
          <div class="locus-async-state__panel-skeleton locus-async-state__panel-skeleton--split">
            <ElSkeletonItem variant="rect" />
            <ElSkeletonItem variant="rect" />
          </div>
        </div>

        <div
          v-else-if="skeleton === 'knowledge'"
          class="locus-async-state__workspace-skeleton locus-async-state__workspace-skeleton--knowledge"
        >
          <div class="locus-async-state__index-skeleton">
            <ElSkeletonItem variant="rect" />
            <ElSkeletonItem v-for="index in 5" :key="index" variant="text" />
          </div>
          <div class="locus-async-state__document-skeleton">
            <ElSkeletonItem variant="h1" />
            <ElSkeletonItem v-for="index in 7" :key="index" variant="text" />
          </div>
        </div>

        <div v-else-if="skeleton === 'document'" class="locus-async-state__document-skeleton">
          <ElSkeletonItem variant="h1" />
          <ElSkeletonItem v-for="index in 8" :key="index" variant="text" />
        </div>

        <div
          v-else-if="skeleton === 'graph'"
          class="locus-async-state__workspace-skeleton locus-async-state__workspace-skeleton--graph"
        >
          <ElSkeletonItem class="locus-async-state__canvas-skeleton" variant="rect" />
          <div class="locus-async-state__inspector-skeleton">
            <ElSkeletonItem variant="h3" />
            <ElSkeletonItem v-for="index in 7" :key="index" variant="text" />
          </div>
        </div>

        <div v-else class="locus-async-state__app-skeleton">
          <ElSkeletonItem variant="h1" />
          <div class="locus-async-state__stat-skeleton">
            <ElSkeletonItem v-for="index in 3" :key="index" variant="rect" />
          </div>
          <ElSkeletonItem class="locus-async-state__app-panel" variant="rect" />
        </div>
      </template>
    </ElSkeleton>
  </section>

  <ElResult
    v-else-if="error"
    class="locus-async-state locus-async-state--error"
    icon="error"
    :title="t('common.error')"
    :sub-title="error.message || t('common.errorHint')"
    role="alert"
  >
    <template v-if="retryable" #extra>
      <ElButton type="primary" :loading="retrying" @click="emit('retry')">
        <ElIcon v-if="!retrying"><RefreshRight /></ElIcon>
        {{ t('common.retry') }}
      </ElButton>
    </template>
  </ElResult>

  <ElEmpty
    v-else-if="empty"
    class="locus-async-state locus-async-state--empty"
    :description="emptyText || t('common.noData')"
  />
  <slot v-else />
</template>

<style scoped>
.locus-async-state {
  min-width: 0;
}

.locus-async-state--loading {
  display: grid;
  gap: var(--locus-space-5);
}

.locus-async-state__feedback {
  display: flex;
  align-items: center;
  gap: var(--locus-space-3);
  color: var(--text-muted);
  font-size: var(--locus-font-size-md);
}

.locus-async-state__app-skeleton,
.locus-async-state__status-skeleton {
  display: grid;
  gap: var(--locus-space-5);
}

.locus-async-state__app-skeleton > :first-child,
.locus-async-state__document-skeleton > :first-child {
  width: min(18rem, 55%);
}

.locus-async-state__app-panel {
  height: min(52vh, 32rem);
}

.locus-async-state__filter-skeleton {
  display: grid;
  grid-template-columns: minmax(13.75rem, 20rem) 8.75rem;
  gap: var(--locus-space-3);
}

.locus-async-state__filter-skeleton > * {
  height: var(--locus-control-height);
}

.locus-async-state__stat-skeleton {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--locus-space-3);
}

.locus-async-state__app-skeleton .locus-async-state__stat-skeleton {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.locus-async-state__stat-skeleton > * {
  height: 4.5rem;
}

.locus-async-state__panel-skeleton {
  display: grid;
  gap: var(--locus-space-5);
}

.locus-async-state__panel-skeleton--split {
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
}

.locus-async-state__panel-skeleton > * {
  height: var(--locus-data-panel-height);
}

.locus-async-state__workspace-skeleton {
  min-height: min(68vh, 42rem);
  display: grid;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-md);
  background: var(--surface-panel);
}

.locus-async-state__workspace-skeleton--graph {
  grid-template-columns: minmax(0, 1fr) 20.5rem;
}

.locus-async-state__workspace-skeleton--knowledge {
  grid-template-columns: 18.5rem minmax(0, 1fr);
}

.locus-async-state__canvas-skeleton {
  width: calc(100% - var(--locus-space-8) * 2);
  height: calc(100% - var(--locus-space-8) * 2);
  margin: var(--locus-space-8);
}

.locus-async-state__inspector-skeleton,
.locus-async-state__index-skeleton,
.locus-async-state__document-skeleton {
  display: grid;
  align-content: start;
  gap: var(--locus-space-6);
  padding: var(--locus-space-8);
}

.locus-async-state__inspector-skeleton,
.locus-async-state__index-skeleton {
  border-left: 1px solid var(--border-subtle);
}

.locus-async-state__workspace-skeleton--knowledge .locus-async-state__index-skeleton {
  border-right: 1px solid var(--border-subtle);
  border-left: 0;
}

.locus-async-state__index-skeleton > :first-child {
  height: var(--locus-control-height);
  margin-bottom: var(--locus-space-5);
}

.locus-async-state__document-skeleton > :nth-child(n + 4) {
  width: 88%;
}

.locus-async-state--error,
.locus-async-state--empty {
  min-height: 18rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

@media (max-width: 1100px) {
  .locus-async-state__panel-skeleton--split {
    grid-template-columns: 1fr;
  }

  .locus-async-state__workspace-skeleton--graph {
    grid-template-columns: 1fr;
  }

  .locus-async-state__inspector-skeleton {
    border-top: 1px solid var(--border-subtle);
    border-left: 0;
  }
}

@media (max-width: 600px) {
  .locus-async-state__filter-skeleton,
  .locus-async-state__workspace-skeleton--knowledge {
    grid-template-columns: 1fr;
  }

  .locus-async-state__stat-skeleton {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .locus-async-state__workspace-skeleton--knowledge .locus-async-state__index-skeleton {
    border-right: 0;
    border-bottom: 1px solid var(--border-subtle);
  }
}
</style>
