<script setup lang="ts">
import type { LocusContext } from '../api'

defineProps<{ context?: LocusContext }>()
</script>

<template>
  <section class="single-page">
    <div class="section-intro">
      <span class="eyebrow">Documentation</span>
      <h2>Knowledge</h2>
      <p>Documentation remains attached to its owning scope and operational declarations.</p>
    </div>
    <article class="detail-panel">
      <div class="empty-state"><span>Scope</span><strong>{{ context?.active_scope.id }}</strong><small>Documentation references will be indexed from the validated declared view.</small></div>
    </article>
  </section>
</template>
