<script setup lang="ts">
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
  <div class="locus-toolbar">
    <div class="locus-toolbar__scope">
      <span>{{ t('app.activeScope') }}</span>
      <strong>{{ scopeName }}</strong>
    </div>

    <div class="locus-toolbar__controls">
      <label class="locus-toolbar__field">
        <span>{{ t('context.currentEntity') }}</span>
        <ElInput
          v-model="context.from"
          class="locus-toolbar__control locus-toolbar__control--entity"
          size="default"
          :aria-label="t('context.currentEntity')"
        />
      </label>
      <label class="locus-toolbar__field">
        <span>{{ t('context.vantage') }}</span>
        <ElInput
          v-model="context.vantage"
          class="locus-toolbar__control"
          size="default"
          :aria-label="t('context.vantage')"
        />
      </label>
      <label class="locus-toolbar__field">
        <span>{{ t('settings.language') }}</span>
        <ElSelect
          v-model="preferences.locale.value"
          class="locus-toolbar__preference"
          size="default"
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
          size="default"
          :aria-label="t('settings.theme')"
        >
          <ElOption :label="t('settings.system')" value="system" />
          <ElOption :label="t('settings.light')" value="light" />
          <ElOption :label="t('settings.dark')" value="dark" />
        </ElSelect>
      </label>
      <div class="locus-toolbar__field">
        <span>{{ t('app.service') }}</span>
        <ElTag
          class="locus-toolbar__service"
          size="default"
          :type="connectionState === 'error' ? 'danger' : 'success'"
          effect="plain"
        >
          {{ connectionText }}
        </ElTag>
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

.locus-toolbar__service {
  height: var(--locus-control-height);
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
  .locus-toolbar__service {
    width: 100%;
  }
}
</style>
