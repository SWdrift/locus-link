import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/graph' },
    { path: '/graph', component: () => import('./views/GraphView.vue') },
    { path: '/status', component: () => import('./views/StatusView.vue') },
    { path: '/knowledge', component: () => import('./views/KnowledgeView.vue') },
    { path: '/inspect', component: () => import('./views/InspectView.vue') },
  ],
})
