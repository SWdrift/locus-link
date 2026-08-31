<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import { ElButton, ElCard } from 'element-plus'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDocument, getKnowledge } from '../api'
import AsyncState from '../components/AsyncState.vue'
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
    <div class="section-intro"><span class="eyebrow">{{ t('knowledge.eyebrow') }}</span><h2>{{ t('knowledge.title') }}</h2><p>{{ t('knowledge.description') }}</p></div>
    <AsyncState :loading="knowledge.isPending.value" :error="knowledge.error.value" :empty="empty" :empty-text="t('knowledge.noDocumentationHint')">
      <div class="knowledge-layout">
        <aside class="document-list">
          <ElButton v-for="item in knowledge.data.value?.documents" :key="item.id" text class="document-choice" :class="{ active: selected === item.id }" :aria-pressed="selected === item.id" @click="selected = item.id">
            <span>{{ item.scope_id }}</span><strong>{{ item.title }}</strong><small>{{ t('knowledge.references', { count: item.associations.length }) }} · {{ item.path }}</small>
          </ElButton>
        </aside>
        <ElCard shadow="never" class="document-reader">
          <AsyncState :loading="documentQuery.isPending.value" :error="documentQuery.error.value" :empty="!selected">
            <template v-if="documentQuery.data.value">
              <header><div><span class="eyebrow">{{ documentQuery.data.value.scope_id }}</span><h3>{{ documentQuery.data.value.title }}</h3></div><small>{{ documentQuery.data.value.path }}</small></header>
              <div v-if="documentQuery.data.value.format === 'markdown'" class="markdown-body" v-html="rendered"></div>
              <pre v-else class="plain-document">{{ documentQuery.data.value.body }}</pre>
              <footer><span v-for="item in documentQuery.data.value.associations" :key="item.object_id + item.ref">{{ item.object_type }} · {{ item.object_id }}</span></footer>
            </template>
          </AsyncState>
        </ElCard>
      </div>
    </AsyncState>
  </section>
</template>
