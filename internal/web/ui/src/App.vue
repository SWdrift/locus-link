<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { useI18n } from 'vue-i18n'
import { getContext, getValidation } from './api'
import AppNavigation from './components/AppNavigation.vue'
import AsyncState from './components/AsyncState.vue'
import OperationalToolbar from './components/OperationalToolbar.vue'
import { provideOperationalContext } from './operational-context'
import { usePreferences } from './preferences'

const { t } = useI18n()
const preferences = usePreferences()
const contextQuery = useQuery({ queryKey: ['context'], queryFn: getContext })
const validationQuery = useQuery({ queryKey: ['validation'], queryFn: getValidation })
const operationalContext = reactive({ from: '', vantage: '' })
const sidebarCollapsed = ref(false)

provideOperationalContext(operationalContext)

watch(
  () => contextQuery.data.value,
  value => {
    if (!value) return
    if (!operationalContext.from) operationalContext.from = value.runtime.current_entity ?? ''
    if (!operationalContext.vantage) operationalContext.vantage = value.runtime.vantage
  },
  { immediate: true },
)

const scopeName = computed(() => contextQuery.data.value?.active_scope.id ?? t('common.loading'))
const connectionState = computed<'error' | 'valid' | 'local'>(() => {
  if (contextQuery.isError.value || validationQuery.isError.value) return 'error'
  return validationQuery.data.value?.valid ? 'valid' : 'local'
})
const connectionText = computed(() => {
  if (connectionState.value === 'error') return t('app.unavailable')
  return connectionState.value === 'valid' ? t('app.validated') : t('app.localService')
})
</script>

<template>
  <ElConfigProvider :locale="preferences.componentLocale.value" size="small">
    <div class="locus-app" :class="preferences.dark.value ? 'locus-theme--dark' : 'locus-theme--light'">
      <ElContainer class="locus-app__shell">
        <ElAside
          class="locus-app__sidebar"
          :width="sidebarCollapsed ? 'var(--locus-sidebar-collapsed)' : 'var(--locus-sidebar-expanded)'"
        >
          <AppNavigation :collapsed="sidebarCollapsed" @toggle="sidebarCollapsed = !sidebarCollapsed" />
        </ElAside>

        <ElContainer class="locus-app__stage">
          <ElHeader class="locus-app__toolbar" height="auto">
            <OperationalToolbar
              :scope-name="scopeName"
              :connection-state="connectionState"
              :connection-text="connectionText"
            />
          </ElHeader>

          <ElMain class="locus-app__content">
            <AsyncState
              :loading="contextQuery.isPending.value"
              :error="contextQuery.error.value"
              :retrying="contextQuery.isFetching.value"
              retryable
              skeleton="app"
              @retry="contextQuery.refetch()"
            >
              <RouterView />
            </AsyncState>
          </ElMain>
        </ElContainer>
      </ElContainer>
    </div>
  </ElConfigProvider>
</template>

<style>
.locus-app {
  --status-success: #2e7d57;
  --status-failure: #c2413a;
  --status-stale: #a86d16;
  --status-unknown: #6f7b88;
  --el-color-primary: var(--accent);
  --el-bg-color: var(--surface-panel);
  --el-bg-color-overlay: var(--surface-raised);
  --el-fill-color-blank: var(--surface-panel);
  --el-fill-color-light: var(--surface-subtle);
  --el-fill-color-lighter: var(--surface-hover);
  --el-border-color: var(--border-subtle);
  --el-border-color-light: var(--border-subtle);
  --el-text-color-primary: var(--text-primary);
  --el-text-color-regular: var(--text-secondary);
  --el-text-color-secondary: var(--text-muted);
  --el-border-radius-base: var(--locus-radius-md);
  min-height: 100vh;
  color: var(--text-primary);
  background: var(--surface-page);
  font-size: var(--locus-font-size-lg);
}

.locus-theme--light {
  --surface-page: #f4f6f8;
  --surface-sidebar: #f8f9fb;
  --surface-panel: #ffffff;
  --surface-subtle: #f6f7f9;
  --surface-hover: #edf0f3;
  --surface-raised: #ffffff;
  --border-subtle: #dfe3e8;
  --border-strong: #c8ced6;
  --text-primary: #20252b;
  --text-secondary: #535d68;
  --text-muted: #76808c;
  --accent: #2f67a2;
  --accent-muted: #e6eef7;
}

.locus-theme--dark {
  --surface-page: #111419;
  --surface-sidebar: #15191f;
  --surface-panel: #1a1f26;
  --surface-subtle: #1e242c;
  --surface-hover: #252c35;
  --surface-raised: #20262e;
  --border-subtle: #303741;
  --border-strong: #424b57;
  --text-primary: #e8ebef;
  --text-secondary: #adb5bf;
  --text-muted: #858f9b;
  --accent: #73a7dc;
  --accent-muted: #22384f;
}

.locus-app__shell {
  height: 100vh;
  overflow: hidden;
}

.locus-app__sidebar {
  height: 100vh;
  flex: 0 0 auto;
  overflow: hidden;
  border-right: 1px solid var(--border-subtle);
  background: var(--surface-sidebar);
  transition: width var(--locus-transition-fast);
}

.locus-app__stage {
  min-width: 0;
  height: 100vh;
  flex-direction: column;
  overflow: hidden;
}

.locus-app__toolbar {
  z-index: 20;
  min-width: 0;
  flex: 0 0 auto;
  padding: var(--locus-space-3) var(--locus-space-8);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-panel);
}

.locus-app__content {
  min-width: 0;
  min-height: 0;
  flex: 1 1 auto;
  padding: var(--locus-space-6) var(--locus-space-8) var(--locus-space-8);
  overflow: auto;
}

.technical-id {
  overflow-wrap: anywhere;
  font-family: var(--locus-font-family-mono);
  font-size: 0.92em;
}

@media (max-width: 800px) {
  .locus-app__shell {
    height: auto;
    min-height: 100vh;
    display: block;
    overflow: visible;
  }

  .locus-app__sidebar {
    position: sticky;
    top: 0;
    z-index: 30;
    width: 100% !important;
    height: auto;
    border-right: 0;
    border-bottom: 1px solid var(--border-subtle);
  }

  .locus-app__stage {
    height: auto;
    min-height: 0;
    overflow: visible;
  }

  .locus-app__toolbar {
    padding: var(--locus-space-3) var(--locus-space-6);
  }

  .locus-app__content {
    padding: var(--locus-space-5) var(--locus-space-6) var(--locus-space-6);
    overflow: visible;
  }
}
</style>
