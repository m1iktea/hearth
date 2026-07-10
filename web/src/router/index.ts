import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
    { path: '/nav', name: 'nav', component: () => import('../views/NavView.vue') },
    { path: '/nodes', name: 'nodes', component: () => import('../views/NodesView.vue') },
  ],
})
