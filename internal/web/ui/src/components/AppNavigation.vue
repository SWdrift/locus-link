<script setup lang="ts">
import { DataAnalysis, Document, Expand, Fold, Share } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

defineProps<{ collapsed: boolean }>()
const emit = defineEmits<{ toggle: [] }>()

const route = useRoute()
const { t } = useI18n()
</script>

<template>
  <div class="locus-navigation" :class="{ 'locus-navigation--collapsed': collapsed }">
    <header class="locus-navigation__header">
      <strong class="locus-navigation__product">{{ t('app.name') }}</strong>
    </header>

    <ElMenu class="locus-navigation__menu" :collapse="collapsed" :default-active="route.path" router>
      <ElMenuItem class="locus-navigation__item" index="/graph" :title="collapsed ? t('nav.graph') : undefined">
        <ElIcon><Share /></ElIcon>
        <span class="locus-navigation__label">{{ t('nav.graph') }}</span>
      </ElMenuItem>
      <ElMenuItem class="locus-navigation__item" index="/status" :title="collapsed ? t('nav.status') : undefined">
        <ElIcon><DataAnalysis /></ElIcon>
        <span class="locus-navigation__label">{{ t('nav.status') }}</span>
      </ElMenuItem>
      <ElMenuItem class="locus-navigation__item" index="/knowledge" :title="collapsed ? t('nav.knowledge') : undefined">
        <ElIcon><Document /></ElIcon>
        <span class="locus-navigation__label">{{ t('nav.knowledge') }}</span>
      </ElMenuItem>
    </ElMenu>
    <footer class="locus-navigation__footer">
      <button
        class="locus-navigation__toggle"
        type="button"
        :aria-label="collapsed ? t('nav.expand') : t('nav.collapse')"
        @click="emit('toggle')"
      >
        <ElIcon><component :is="collapsed ? Expand : Fold" /></ElIcon>
      </button>
    </footer>
  </div>
</template>

<style scoped>
.locus-navigation {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: var(--locus-space-5) var(--locus-space-4);
}

.locus-navigation__header {
  min-height: var(--locus-control-height);
  display: flex;
  align-items: center;
  padding: 0 var(--locus-space-2) var(--locus-space-5) var(--locus-space-4);
}

.locus-navigation__product {
  overflow: hidden;
  font-size: var(--locus-font-size-lg);
  font-weight: var(--locus-font-weight-strong);
  letter-spacing: -0.01em;
  white-space: nowrap;
}

.locus-navigation__footer {
  min-height: var(--locus-touch-target);
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
  margin-top: auto;
  padding-top: var(--locus-space-4);
  border-top: 1px solid var(--border-subtle);
}

.locus-navigation__toggle {
  width: calc(var(--locus-control-height) - var(--locus-space-2));
  height: calc(var(--locus-control-height) - var(--locus-space-2));
  display: inline-grid;
  flex: 0 0 auto;
  place-items: center;
  padding: 0;
  border: 0;
  color: var(--text-muted);
  background: transparent;
  cursor: pointer;
  transition: color var(--locus-transition-fast);
}

.locus-navigation__toggle:hover {
  color: var(--text-secondary);
}

.locus-navigation__toggle .el-icon {
  width: var(--el-menu-icon-width);
  font-size: calc(var(--locus-icon-size-md) - var(--locus-space-1));
}

.locus-navigation__toggle:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}


.locus-navigation__menu {
  --el-menu-bg-color: transparent;
  --el-menu-hover-bg-color: var(--surface-hover);
  --el-menu-active-color: var(--accent);
  --el-menu-text-color: var(--text-secondary);
  border: 0;
}

.locus-navigation__item {
  height: var(--locus-navigation-item-height);
  margin-bottom: var(--locus-space-1);
  padding: 0 var(--locus-space-5);
  border-radius: var(--locus-radius-md);
  line-height: var(--locus-navigation-item-height);
}

.locus-navigation__item.is-active {
  background: var(--accent-muted);
  font-weight: var(--locus-font-weight-semibold);
}

.locus-navigation__label {
  margin-left: var(--locus-space-4);
}

.locus-navigation--collapsed .locus-navigation__header {
  display: none;
}

.locus-navigation--collapsed .locus-navigation__product,
.locus-navigation--collapsed .locus-navigation__label {
  display: none;
}

.locus-navigation--collapsed .locus-navigation__menu {
  width: 100%;
}

.locus-navigation--collapsed .locus-navigation__item {
  width: 100%;
  min-width: 0;
  justify-content: center;
  padding: 0;
}

.locus-navigation--collapsed .locus-navigation__item :deep(.el-icon) {
  margin-right: 0;
}

.locus-navigation--collapsed .locus-navigation__footer {
  justify-content: center;
}


@media (max-width: 800px) {
  .locus-navigation {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    gap: var(--locus-space-6);
    padding: var(--locus-space-3) var(--locus-space-5);
  }

  .locus-navigation__header,
  .locus-navigation--collapsed .locus-navigation__header {
    min-height: var(--locus-control-height);
    display: flex;
    padding: 0;
  }

  .locus-navigation__footer {
    display: none;
  }

  .locus-navigation__menu {
    display: flex;
    justify-content: flex-end;
    min-width: 0;
  }

  .locus-navigation__item,
  .locus-navigation--collapsed .locus-navigation__item {
    min-width: 88px;
    height: var(--locus-touch-target);
    margin: 0 var(--locus-space-1);
    padding: 0 var(--locus-space-5);
    line-height: var(--locus-touch-target);
    justify-content: center;
  }

  .locus-navigation--collapsed .locus-navigation__product,
  .locus-navigation--collapsed .locus-navigation__label {
    display: inline;
  }

}

@media (max-width: 480px) {
  .locus-navigation {
    grid-template-columns: 1fr;
    gap: var(--locus-space-2);
  }

  .locus-navigation__header {
    display: none;
  }

  .locus-navigation__menu {
    justify-content: stretch;
  }

  .locus-navigation__item,
  .locus-navigation--collapsed .locus-navigation__item {
    flex: 1;
    min-width: 0;
    padding: 0 var(--locus-space-3);
  }
}
</style>
