<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { ElConfigProvider, ElInput } from 'element-plus'
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getContext, getValidation } from './api'
import AsyncState from './components/AsyncState.vue'
import { provideOperationalContext } from './operational-context'
import { usePreferences } from './preferences'
const { t } = useI18n()
const preferences = usePreferences()
const contextQuery = useQuery({ queryKey: ['context'], queryFn: getContext })
const validation = useQuery({ queryKey: ['validation'], queryFn: getValidation })
const operationalContext = reactive({ from: '', vantage: '' })
provideOperationalContext(operationalContext)

watch(() => contextQuery.data.value, value => {
  if (!value) return
  if (!operationalContext.from) operationalContext.from = value.runtime.current_entity ?? ''
  if (!operationalContext.vantage) operationalContext.vantage = value.runtime.vantage
}, { immediate: true })

const scopeName = computed(() => contextQuery.data.value?.active_scope.id ?? t('common.loading'))
const connectionState = computed(() => {
  if (contextQuery.isError.value || validation.isError.value) return 'error'
  return validation.data.value?.valid ? 'valid' : 'local'
})
const connectionText = computed(() => connectionState.value === 'error' ? t('app.unavailable') : connectionState.value === 'valid' ? t('app.validated') : t('app.localService'))
</script>

<template>
  <ElConfigProvider :locale="preferences.componentLocale.value" size="small">
    <div class="app-root" :class="preferences.dark.value ? 'theme-dark' : 'theme-light'">
      <div class="app-shell">
        <aside class="sidebar">
          <div class="brand"><span class="brand-mark">L</span><div><strong>{{ t('app.name') }}</strong><small>{{ t('app.subtitle') }}</small></div></div>
          <nav :aria-label="t('nav.primary')">
            <RouterLink to="/graph">{{ t('nav.graph') }}</RouterLink>
            <RouterLink to="/status">{{ t('nav.status') }}</RouterLink>
            <RouterLink to="/knowledge">{{ t('nav.knowledge') }}</RouterLink>
          </nav>
          <div class="workspace-card">
            <span>{{ t('app.activeScope') }}</span><strong>{{ scopeName }}</strong><small>{{ t('context.vantage') }} · {{ operationalContext.vantage || '—' }}</small>
          </div>
        </aside>

        <main>
          <header class="topbar">
            <div class="workspace-heading"><span class="eyebrow">{{ t('app.workspace') }}</span><h1>{{ scopeName }}</h1></div>
            <div class="header-actions">
              <div class="context-controls" :aria-label="t('context.compact')">
                <label><span>{{ t('context.currentEntity') }}</span><ElInput v-model="operationalContext.from" :aria-label="t('context.currentEntity')" /></label>
                <label><span>{{ t('context.vantage') }}</span><ElInput v-model="operationalContext.vantage" :aria-label="t('context.vantage')" /></label>
              </div>
              <div class="preference-controls">
                <label><span>{{ t('settings.language') }}</span><select v-model="preferences.locale.value"><option value="zh-CN">简体中文</option><option value="en-US">English</option></select></label>
                <label><span>{{ t('settings.theme') }}</span><select v-model="preferences.themeMode.value"><option value="system">{{ t('settings.system') }}</option><option value="light">{{ t('settings.light') }}</option><option value="dark">{{ t('settings.dark') }}</option></select></label>
              </div>
              <span class="connection" :class="{ error: connectionState === 'error' }">{{ connectionText }}</span>
            </div>
          </header>

          <AsyncState :loading="contextQuery.isPending.value" :error="contextQuery.error.value">
            <RouterView />
          </AsyncState>
        </main>
      </div>
    </div>
  </ElConfigProvider>
</template>
