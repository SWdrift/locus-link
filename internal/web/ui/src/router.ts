import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/locus/catalog' },
    { path: '/locus/catalog', component: () => import('./views/LocusView.vue') },
    { path: '/locus/dependencies', component: () => import('./views/LocusView.vue') },
    { path: '/scopes/:scopeId/graph', component: () => import('./views/GraphView.vue') },
    { path: '/scopes/:scopeId/status', component: () => import('./views/StatusView.vue') },
    { path: '/scopes/:scopeId/knowledge', component: () => import('./views/KnowledgeView.vue') },
    { path: '/scopes/:scopeId/inspect', component: () => import('./views/InspectView.vue') },
  ],
})
