<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useOperationalContext } from '../operational-context'
import { usePreferences } from '../preferences'
import { scopePath, useScopeID } from '../scope-context'

const props = defineProps<{
  scopeName: string
  connectionState: 'error' | 'valid' | 'partial' | 'loading'
  connectionText: string
}>()

const { t } = useI18n()
const context = useOperationalContext()
const scopeID = useScopeID()
const preferences = usePreferences()
const declarationStatusTagType = computed(() => {
  if (props.connectionState === 'error') return 'danger'
  if (props.connectionState === 'partial') return 'warning'
  if (props.connectionState === 'loading') return 'info'
  return 'success'
})
</script>

<template>
  <div class="locus-toolbar">
    <div class="locus-toolbar__scope">
      <span>{{ t('app.activeScope') }}</span>
      <strong>{{ scopeName }}</strong>
    </div>

    <div class="locus-toolbar__controls">
      <label v-if="scopeID" class="locus-toolbar__field">
        <span>{{ t('context.currentEntity') }}</span>
        <ElInput
          v-model="context.from"
          class="locus-toolbar__control locus-toolbar__control--entity"
          size="small"
          :aria-label="t('context.currentEntity')"
        />
      </label>
      <label v-if="scopeID" class="locus-toolbar__field">
        <span>{{ t('context.vantage') }}</span>
        <ElInput
          v-model="context.vantage"
          class="locus-toolbar__control"
          size="small"
          :aria-label="t('context.vantage')"
        />
      </label>
      <label class="locus-toolbar__field">
        <span>{{ t('settings.language') }}</span>
        <ElSelect
          v-model="preferences.locale.value"
          class="locus-toolbar__preference"
          size="small"
          :aria-label="t('settings.language')"
        >
          <ElOption label="简体中文" value="zh-CN" />
          <ElOption label="English" value="en-US" />
        </ElSelect>
      </label>
      <label class="locus-toolbar__field">
        <span>{{ t('settings.theme') }}</span>
        <ElSelect
          v-model="preferences.themeMode.value"
          class="locus-toolbar__preference"
          size="small"
          :aria-label="t('settings.theme')"
        >
          <ElOption :label="t('settings.system')" value="system" />
          <ElOption :label="t('settings.light')" value="light" />
          <ElOption :label="t('settings.dark')" value="dark" />
        </ElSelect>
      </label>
      <div v-if="scopeID" class="locus-toolbar__field">
        <span>{{ t('app.declarationStatus') }}</span>
        <RouterLink
          class="locus-toolbar__status-link"
          :to="{ path: scopePath(scopeID, 'inspect'), query: { tab: 'validation' } }"
        >
          <ElTag class="locus-toolbar__status" size="small" :type="declarationStatusTagType" effect="plain">
            {{ connectionText }}
          </ElTag>
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.locus-toolbar {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--locus-space-8);
}

.locus-toolbar__scope {
  min-width: 148px;
  display: grid;
  gap: var(--locus-space-1);
}

.locus-toolbar__scope span {
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.locus-toolbar__scope strong {
  overflow: hidden;
  font-size: var(--locus-font-size-base);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.locus-toolbar__controls {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: flex-end;
  gap: var(--locus-space-3);
}

.locus-toolbar__field {
  min-width: 0;
  display: grid;
  gap: var(--locus-space-1);
  color: var(--text-muted);
  font-size: var(--locus-font-size-sm);
}

.locus-toolbar__control {
  width: 160px;
}

.locus-toolbar__control--entity {
  width: 216px;
}

.locus-toolbar__preference {
  width: 104px;
}

.locus-toolbar__status-link {
  display: inline-flex;
  text-decoration: none;
}

.locus-toolbar__status {
  height: var(--el-component-size-small);
}

@media (max-width: 1180px) {
  .locus-toolbar {
    align-items: flex-start;
    flex-direction: column;
    gap: var(--locus-space-3);
  }

  .locus-toolbar__controls {
    width: 100%;
    justify-content: flex-start;
  }
}

@media (max-width: 680px) {
  .locus-toolbar__controls {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .locus-toolbar__control,
  .locus-toolbar__control--entity,
  .locus-toolbar__preference,
  .locus-toolbar__status {
    width: 100%;
  }
}
</style>
