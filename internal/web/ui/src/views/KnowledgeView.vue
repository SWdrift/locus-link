<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import { useI18n } from 'vue-i18n'
import { getDocument, getKnowledge } from '../api'
import AsyncState from '../components/AsyncState.vue'
import FilterToolbar from '../components/FilterToolbar.vue'
import PageHeader from '../components/PageHeader.vue'

const { t } = useI18n()
const route = useRoute()
const knowledgeQuery = useQuery({ queryKey: ['knowledge'], queryFn: getKnowledge })
const selectedDocument = ref('')
const searchText = ref(typeof route.query.search === 'string' ? route.query.search : '')
const scopeFilter = ref('all')
const markdown = new MarkdownIt({ html: false, linkify: true, typographer: true })

const scopeOptions = computed(() => [
  { value: 'all', label: t('knowledge.allScopes') },
  ...[...new Set(knowledgeQuery.data.value?.documents.map(document => document.scope_id) ?? [])]
    .sort()
    .map(scope => ({ value: scope, label: scope })),
])
const filteredDocuments = computed(() => {
  const term = searchText.value.trim().toLocaleLowerCase()
  return (knowledgeQuery.data.value?.documents ?? []).filter(document => {
    if (scopeFilter.value !== 'all' && document.scope_id !== scopeFilter.value) return false
    if (!term) return true
    const associations = document.associations.flatMap(association => [
      association.object_id,
      association.object_type,
      association.ref,
    ])
    return [document.title, document.path, document.scope_id, ...associations].some(value =>
      value.toLocaleLowerCase().includes(term),
    )
  })
})

watch(
  [filteredDocuments, () => route.query.document],
  ([documents, documentQuery]) => {
    const requested = typeof documentQuery === 'string' ? documentQuery : ''
    if (requested && documents.some(document => document.id === requested)) {
      selectedDocument.value = requested
      return
    }
    if (!documents.some(document => document.id === selectedDocument.value)) {
      selectedDocument.value = documents[0]?.id ?? ''
    }
  },
  { immediate: true },
)

const documentQuery = useQuery({
  queryKey: computed(() => ['document', selectedDocument.value]),
  queryFn: () => getDocument(selectedDocument.value),
  enabled: computed(() => Boolean(selectedDocument.value)),
})
const rendered = computed(() => {
  const document = documentQuery.data.value
  if (document?.format !== 'markdown') return ''
  return DOMPurify.sanitize(markdown.render(document.body), { USE_PROFILES: { html: true } })
})
const empty = computed(() => Boolean(knowledgeQuery.data.value && !knowledgeQuery.data.value.documents.length))
</script>

<template>
  <section class="knowledge-view">
    <PageHeader :eyebrow="t('knowledge.eyebrow')" :title="t('knowledge.title')" />

    <AsyncState
      :loading="knowledgeQuery.isPending.value"
      :error="knowledgeQuery.error.value"
      :empty="empty"
      :empty-text="t('knowledge.noDocumentationHint')"
      :retrying="knowledgeQuery.isFetching.value"
      retryable
      skeleton="knowledge"
      @retry="knowledgeQuery.refetch()"
    >
      <ElContainer class="knowledge-view__workspace">
        <ElAside class="knowledge-view__index" width="296px">
          <FilterToolbar
            v-model:search="searchText"
            v-model:filter="scopeFilter"
            class="knowledge-view__filters"
            layout="stacked"
            :search-placeholder="t('knowledge.search')"
            :search-label="t('knowledge.search')"
            :filter-label="t('knowledge.scopeFilter')"
            :options="scopeOptions"
          />

          <div class="knowledge-view__document-scroll">
            <ElMenu
              v-if="filteredDocuments.length"
              class="knowledge-view__menu"
              :default-active="selectedDocument"
              @select="selectedDocument = $event"
            >
              <ElMenuItem v-for="item in filteredDocuments" :key="item.id" :index="item.id">
                <span class="knowledge-view__document-summary">
                  <span>{{ item.scope_id }}</span>
                  <strong>{{ item.title }}</strong>
                  <small>{{ t('knowledge.references', { count: item.associations.length }) }} · {{ item.path }}</small>
                </span>
              </ElMenuItem>
            </ElMenu>
            <ElEmpty v-else :description="t('knowledge.noMatches')" />
          </div>
        </ElAside>

        <ElMain class="knowledge-view__reader">
          <AsyncState
            :loading="documentQuery.isPending.value"
            :error="documentQuery.error.value"
            :empty="!selectedDocument"
            :retrying="documentQuery.isFetching.value"
            retryable
            skeleton="document"
            @retry="documentQuery.refetch()"
          >
            <template v-if="documentQuery.data.value">
              <header class="knowledge-view__document-header">
                <div>
                  <span>{{ documentQuery.data.value.scope_id }}</span>
                  <h2>{{ documentQuery.data.value.title }}</h2>
                </div>
                <small>{{ documentQuery.data.value.path }}</small>
              </header>
              <div
                v-if="documentQuery.data.value.format === 'markdown'"
                class="knowledge-view__markdown"
                v-html="rendered"
              />
              <pre v-else class="knowledge-view__plain-text">{{ documentQuery.data.value.body }}</pre>
              <footer class="knowledge-view__associations">
                <ElTag
                  v-for="item in documentQuery.data.value.associations"
                  :key="item.object_id + item.ref"
                  type="info"
                  effect="light"
                >
                  {{ item.object_type }} · {{ item.object_id }} · {{ item.ref }}
                </ElTag>
              </footer>
            </template>
          </AsyncState>
        </ElMain>
      </ElContainer>
    </AsyncState>
  </section>
</template>

<style scoped>
.knowledge-view {
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.knowledge-view__workspace {
  min-width: 0;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-md);
  background: var(--surface-panel);
}

.knowledge-view__index {
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: var(--locus-space-5);
  overflow: hidden;
  border-right: 1px solid var(--border-subtle);
}

.knowledge-view__filters {
  padding-bottom: var(--locus-space-4);
}

.knowledge-view__document-scroll {
  min-height: 0;
  flex: 1;
  overflow: auto;
}

.knowledge-view__menu {
  border: 0;
}

.knowledge-view__document-summary {
  width: 100%;
  min-width: 0;
  display: grid;
  gap: var(--locus-space-1);
  line-height: var(--locus-line-height-tight);
}

.knowledge-view__document-summary span,
.knowledge-view__document-summary small {
  max-width: 100%;
  overflow: hidden;
  color: var(--text-muted);
  font-size: var(--locus-font-size-xs);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.knowledge-view__reader {
  min-width: 0;
  padding: var(--locus-space-6) var(--locus-space-8);
  overflow: auto;
}

.knowledge-view__document-header {
  min-width: 0;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--locus-space-6);
  padding-bottom: var(--locus-space-4);
  border-bottom: 1px solid var(--border-subtle);
}

.knowledge-view__document-header span,
.knowledge-view__document-header small {
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.knowledge-view__document-header h2 {
  margin-top: var(--locus-space-1);
  font-size: var(--locus-font-size-xl);
}

.knowledge-view__document-header small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.knowledge-view__markdown,
.knowledge-view__plain-text {
  max-width: 820px;
  margin: 0;
  padding: var(--locus-space-6) 0;
  color: var(--text-secondary);
  font: inherit;
  line-height: var(--locus-line-height-reading);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.knowledge-view__markdown :deep(> :first-child) {
  margin-top: 0;
}

.knowledge-view__markdown :deep(h1),
.knowledge-view__markdown :deep(h2),
.knowledge-view__markdown :deep(h3) {
  margin: 1em 0 0.4em;
  color: var(--text-primary);
}

.knowledge-view__markdown :deep(h1) {
  font-size: var(--locus-font-size-3xl);
}

.knowledge-view__markdown :deep(h2) {
  font-size: var(--locus-font-size-2xl);
}

.knowledge-view__markdown :deep(h3) {
  font-size: var(--locus-font-size-xl);
}

.knowledge-view__markdown :deep(code) {
  padding: var(--locus-space-1) var(--locus-space-2);
  border-radius: var(--locus-radius-sm);
  color: var(--accent);
  background: var(--accent-muted);
}

.knowledge-view__associations {
  display: flex;
  flex-wrap: wrap;
  gap: var(--locus-space-3);
  padding-top: var(--locus-space-4);
  border-top: 1px solid var(--border-subtle);
}

@media (max-width: 800px) {
  .knowledge-view {
    height: auto;
  }

  .knowledge-view__workspace {
    display: block;
  }

  .knowledge-view__index {
    width: 100% !important;
    max-height: 248px;
    border-right: 0;
    border-bottom: 1px solid var(--border-subtle);
  }

  .knowledge-view__reader {
    padding: var(--locus-space-6);
    overflow: visible;
  }
}

@media (max-width: 600px) {
  .knowledge-view__document-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
