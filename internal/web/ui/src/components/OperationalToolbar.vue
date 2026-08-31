<script setup lang="ts">
import { ElInput, ElOption, ElSelect, ElTag } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useOperationalContext } from '../operational-context'
import { usePreferences } from '../preferences'

defineProps<{
  scopeName: string
  connectionState: 'error' | 'valid' | 'local'
  connectionText: string
}>()

const { t } = useI18n()
const context = useOperationalContext()
const preferences = usePreferences()
</script>

<template>
  <header class="topbar">
    <div class="workspace-heading">
      <span class="eyebrow">{{ t('app.workspace') }}</span>
      <strong>{{ scopeName }}</strong>
    </div>
    <div class="header-actions">
      <div class="context-controls" :aria-label="t('context.compact')">
        <label><span>{{ t('context.currentEntity') }}</span><ElInput v-model="context.from" class="context-input" :aria-label="t('context.currentEntity')" /></label>
        <label><span>{{ t('context.vantage') }}</span><ElInput v-model="context.vantage" class="context-input" :aria-label="t('context.vantage')" /></label>
      </div>
      <div class="preference-controls">
        <label><span>{{ t('settings.language') }}</span><ElSelect v-model="preferences.locale.value" :aria-label="t('settings.language')"><ElOption label="简体中文" value="zh-CN" /><ElOption label="English" value="en-US" /></ElSelect></label>
        <label><span>{{ t('settings.theme') }}</span><ElSelect v-model="preferences.themeMode.value" :aria-label="t('settings.theme')"><ElOption :label="t('settings.system')" value="system" /><ElOption :label="t('settings.light')" value="light" /><ElOption :label="t('settings.dark')" value="dark" /></ElSelect></label>
      </div>
      <ElTag class="connection" :type="connectionState === 'error' ? 'danger' : 'success'" effect="plain" round>{{ connectionText }}</ElTag>
    </div>
  </header>
</template>
