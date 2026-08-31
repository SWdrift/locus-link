<script setup lang="ts">
import { ElAlert, ElButton, ElInput, ElMenu, ElMenuItem, ElSpace, ElTabPane, ElTabs } from 'element-plus'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvidenceStatus, GraphView, ProbeResult, ResolveResult, StatusView } from '../../api'
import StatusBadge from '../../components/StatusBadge.vue'

const props = defineProps<{
  graph: GraphView
  status?: StatusView
  operationalReady: boolean
  probing: boolean
  resolving: boolean
  probeResult?: ProbeResult
  resolveResult?: ResolveResult
  probeError?: Error | null
  resolveError?: Error | null
  statusError?: Error | null
}>()
const emit = defineEmits<{ probe: [subject: string]; resolve: []; relayout: [] }>()
const selectedRoute = defineModel<string>('selectedRoute', { required: true })
const selectedLink = defineModel<string>('selectedLink', { required: true })
const target = defineModel<string>('target', { required: true })
const capability = defineModel<string>('capability', { required: true })
const { t } = useI18n()
const inspectorTab = ref('routes')
const activeRoute = computed(() => props.graph.routes.find(route => route.canonical_id === selectedRoute.value))
const selectedLinkView = computed(() => props.graph.links.find(link => link.canonical_id === selectedLink.value))
const linkStatus = computed(() => new Map(props.status?.links.map(item => [item.link_id, item.status]) ?? []))
const routeStatus = (routeID: string): EvidenceStatus => props.status?.routes.find(item => item.route_id === routeID)?.evidence.status ?? 'unknown'
const shortID = (value: string) => value.split('::').at(-1) ?? value
</script>

<template>
  <aside class="inspector-panel">
    <section class="inspector-selection">
      <div class="inspector-heading"><span class="eyebrow">{{ t('graph.routeList') }} / {{ t('status.links') }}</span><ElButton text size="small" @click="emit('relayout')">{{ t('graph.relayout') }}</ElButton></div>
      <ElTabs v-model="inspectorTab" stretch>
        <ElTabPane :label="t('graph.routeList')" name="routes">
          <ElSpace direction="vertical" fill :size="8">
            <ElMenu :default-active="selectedRoute" @select="selectedRoute = $event">
              <ElMenuItem v-for="route in graph.routes" :key="route.canonical_id" :index="route.canonical_id">
                <span class="menu-item-content"><span><strong>{{ shortID(route.canonical_id) }}</strong><small>{{ route.steps.length }} {{ t('status.steps') }}</small></span><StatusBadge :status="routeStatus(route.canonical_id)" /></span>
              </ElMenuItem>
            </ElMenu>
            <ElButton v-if="activeRoute" type="primary" size="small" :loading="probing" :disabled="!operationalReady" @click="emit('probe', activeRoute.canonical_id)">{{ t('graph.probeRoute') }}</ElButton>
          </ElSpace>
        </ElTabPane>
        <ElTabPane :label="t('status.links')" name="links">
          <ElSpace direction="vertical" fill :size="8">
            <ElMenu :default-active="selectedLink" @select="selectedLink = $event">
              <ElMenuItem v-for="link in graph.links" :key="link.canonical_id" :index="link.canonical_id">
                <span class="menu-item-content"><span class="technical-id">{{ shortID(link.canonical_id) }}</span><StatusBadge :status="linkStatus.get(link.canonical_id) ?? 'unknown'" /></span>
              </ElMenuItem>
            </ElMenu>
            <div v-if="selectedLinkView" class="selected-detail">
              <strong class="technical-id">{{ selectedLinkView.canonical_id }}</strong><small>{{ selectedLinkView.provider }}</small>
              <ElButton type="primary" size="small" :loading="probing" :disabled="!operationalReady" @click="emit('probe', selectedLinkView.canonical_id)">{{ t('graph.probeLink') }}</ElButton>
            </div>
          </ElSpace>
        </ElTabPane>
      </ElTabs>
    </section>

    <section class="resolve-box">
      <span class="eyebrow">{{ t('graph.resolve') }}</span>
      <label><span>{{ t('graph.target') }}</span><ElInput v-model="target" size="small" /></label>
      <label><span>{{ t('graph.capability') }}</span><ElInput v-model="capability" size="small" /></label>
      <ElButton type="primary" size="small" :loading="resolving" :disabled="!operationalReady || !target || !capability" @click="emit('resolve')">{{ t('graph.resolveRoute') }}</ElButton>
      <ElAlert v-if="resolveResult" type="success" :closable="false" :title="t('graph.resolveStatus.' + resolveResult.status)"><span class="technical-id">{{ resolveResult.route?.canonical_id ?? t('graph.candidates', { count: resolveResult.candidates?.length ?? 0 }) }}</span></ElAlert>
      <ElAlert v-if="resolveError" type="error" :closable="false" :title="resolveError.message" />
    </section>
    <ElAlert v-if="probeResult" type="success" :closable="false" :title="t('graph.probeResult', { status: t('status.' + probeResult.status) })" :description="t('graph.observationsWritten', { count: probeResult.observations.length })" />
    <ElAlert v-if="probeError" type="error" :closable="false" :title="probeError.message" />
    <ElAlert v-if="statusError" type="error" :closable="false" :title="statusError.message" />
  </aside>
</template>
