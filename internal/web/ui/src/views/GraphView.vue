<script setup lang="ts">
import type { LocusContext } from '../api'

defineProps<{ context?: LocusContext }>()
</script>

<template>
  <section class="page-grid">
    <article class="hero-panel">
      <div>
        <span class="eyebrow">Declared view</span>
        <h2>Operational graph</h2>
        <p>Inspect the active scope, its imported environments, and the bindings that connect project roles to canonical entities.</p>
      </div>
      <div class="scope-orbit" aria-hidden="true">
        <span class="orbit-core"></span>
        <span v-for="item in context?.imports ?? []" :key="item.alias" class="orbit-node"></span>
      </div>
    </article>

    <article class="metric-card">
      <span>Imports</span>
      <strong>{{ context?.imports.length ?? 0 }}</strong>
      <small>Composed scopes</small>
    </article>
    <article class="metric-card">
      <span>Bindings</span>
      <strong>{{ Object.keys(context?.bindings ?? {}).length }}</strong>
      <small>Project roles</small>
    </article>

    <article class="detail-panel full-width">
      <div class="panel-heading">
        <div><span class="eyebrow">Current composition</span><h3>Scope context</h3></div>
      </div>
      <dl class="context-list">
        <template v-for="item in context?.imports ?? []" :key="item.alias">
          <dt>{{ item.alias }}</dt><dd>{{ item.scope_id }}</dd>
        </template>
        <template v-if="!context?.imports.length"><dt>Active</dt><dd>{{ context?.active_scope.id }}</dd></template>
      </dl>
    </article>
  </section>
</template>
