<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import { computed, ref, watch } from 'vue'
import type { LocusContext } from '../api'
import { getDocument, getKnowledge } from '../api'

const props = defineProps<{ context?: LocusContext; from: string; vantage: string }>()
const knowledge = useQuery({ queryKey: ['knowledge'], queryFn: getKnowledge })
const selected = ref('')
watch(() => knowledge.data.value, value => {
  if (!selected.value && value?.documents.length) selected.value = value.documents[0].id
}, { immediate: true })
const document = useQuery({ queryKey: computed(() => ['document', selected.value]), queryFn: () => getDocument(selected.value), enabled: computed(() => Boolean(selected.value)) })
const markdown = new MarkdownIt({ html: false, linkify: true, typographer: true })
const rendered = computed(() => {
  const value = document.data.value
  if (!value) return ''
  const html = value.format === 'markdown' ? markdown.render(value.body) : `<pre>${value.body}</pre>`
  return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } })
})
</script>

<template>
  <section class="knowledge-page">
    <div class="section-intro"><span class="eyebrow">Documentation</span><h2>Knowledge</h2><p>Validated references remain attached to their owning declarations and are loaded only from the Scope docs directory.</p></div>
    <div v-if="knowledge.isError.value" class="error-panel">{{ knowledge.error.value?.message }}</div>
    <div v-else class="knowledge-layout">
      <aside class="document-list">
        <button v-for="item in knowledge.data.value?.documents" :key="item.id" :class="{ active: selected === item.id }" @click="selected = item.id">
          <span>{{ item.scope_id }}</span><strong>{{ item.title }}</strong><small>{{ item.associations.length }} references · {{ item.path }}</small>
        </button>
        <div v-if="!knowledge.data.value?.documents.length" class="empty-state"><strong>No documentation</strong><small>This Registry has no validated documentation references.</small></div>
      </aside>
      <article class="document-reader">
        <header v-if="document.data.value"><div><span class="eyebrow">{{ document.data.value.scope_id }}</span><h3>{{ document.data.value.title }}</h3></div><small>{{ document.data.value.path }}</small></header>
        <div v-if="document.isError.value" class="error-panel">{{ document.error.value?.message }}</div>
        <div v-else class="markdown-body" v-html="rendered"></div>
        <footer v-if="document.data.value"><span v-for="item in document.data.value.associations" :key="item.object_id + item.ref">{{ item.object_type }} · {{ item.object_id }}</span></footer>
      </article>
    </div>
  </section>
</template>
