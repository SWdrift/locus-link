<script setup lang="ts">
import type { LocusContext } from '../api'

defineProps<{ context?: LocusContext }>()
</script>

<template>
  <section class="single-page">
    <div class="section-intro">
      <span class="eyebrow">Observation</span>
      <h2>Status</h2>
      <p>Evidence is interpreted for the selected vantage and remains separate from declarations.</p>
    </div>
    <article class="detail-panel">
      <div class="status-line"><span class="status-dot unknown"></span><div><strong>{{ context?.runtime.vantage }}</strong><small>Current observation vantage</small></div></div>
    </article>
  </section>
</template>
