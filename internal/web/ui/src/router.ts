import { createRouter, createWebHistory } from 'vue-router'
import GraphView from './views/GraphView.vue'
import KnowledgeView from './views/KnowledgeView.vue'
import StatusView from './views/StatusView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/graph' },
    { path: '/graph', component: GraphView },
    { path: '/status', component: StatusView },
    { path: '/knowledge', component: KnowledgeView },
  ],
})
