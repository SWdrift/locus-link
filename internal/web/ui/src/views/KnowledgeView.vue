<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import DOMPurify from 'dompurify'
import { ElCard, ElMenu, ElMenuItem, ElTag } from 'element-plus'
import MarkdownIt from 'markdown-it'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDocument, getKnowledge } from '../api'
import AsyncState from '../components/AsyncState.vue'
import PageHeader from '../components/PageHeader.vue'
import './knowledge.css'

const { t } = useI18n()
const knowledge = useQuery({ queryKey: ['knowledge'], queryFn: getKnowledge })
const selected = ref('')
watch(() => knowledge.data.value, value => {
  if (value && !value.documents.some(item => item.id === selected.value)) selected.value = value.documents[0]?.id ?? ''
}, { immediate: true })
const documentQuery = useQuery({ queryKey: computed(() => ['document', selected.value]), queryFn: () => getDocument(selected.value), enabled: computed(() => Boolean(selected.value)) })
const markdown = new MarkdownIt({ html: false, linkify: true, typographer: true })
const rendered = computed(() => {
  const value = documentQuery.data.value
  return value?.format === 'markdown' ? DOMPurify.sanitize(markdown.render(value.body), { USE_PROFILES: { html: true } }) : ''
})
const empty = computed(() => Boolean(knowledge.data.value && !knowledge.data.value.documents.length))
</script>

<template>
  <section class="knowledge-page">
    <PageHeader :eyebrow="t('knowledge.eyebrow')" :title="t('knowledge.title')" :description="t('knowledge.description')" />
    <AsyncState :loading="knowledge.isPending.value" :error="knowledge.error.value" :empty="empty" :empty-text="t('knowledge.noDocumentationHint')">
      <ElCard shadow="never" class="knowledge-shell">
        <div class="knowledge-layout">
          <aside class="document-list">
            <ElMenu :default-active="selected" @select="selected = $event">
              <ElMenuItem v-for="item in knowledge.data.value?.documents" :key="item.id" :index="item.id">
                <span class="document-summary"><span>{{ item.scope_id }}</span><strong>{{ item.title }}</strong><small>{{ t('knowledge.references', { count: item.associations.length }) }} · {{ item.path }}</small></span>
              </ElMenuItem>
            </ElMenu>
          </aside>
          <article class="document-reader">
            <AsyncState :loading="documentQuery.isPending.value" :error="documentQuery.error.value" :empty="!selected">
              <template v-if="documentQuery.data.value">
                <header><div><span class="eyebrow">{{ documentQuery.data.value.scope_id }}</span><h3>{{ documentQuery.data.value.title }}</h3></div><small>{{ documentQuery.data.value.path }}</small></header>
                <div v-if="documentQuery.data.value.format === 'markdown'" class="markdown-body" v-html="rendered"></div>
                <pre v-else class="plain-document">{{ documentQuery.data.value.body }}</pre>
                <footer><ElTag v-for="item in documentQuery.data.value.associations" :key="item.object_id + item.ref" type="info" effect="light">{{ item.object_type }} · {{ item.object_id }}</ElTag></footer>
              </template>
            </AsyncState>
          </article>
        </div>
      </ElCard>
    </AsyncState>
  </section>
</template>
