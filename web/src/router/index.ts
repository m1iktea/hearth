import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
    { path: '/nav', name: 'nav', component: () => import('../views/NavView.vue') },
    { path: '/nodes', name: 'nodes', component: () => import('../views/NodesView.vue') },
    { path: '/devices', name: 'devices', component: () => import('../views/DevicesView.vue') },
    { path: '/devices/:id', name: 'device-detail', component: () => import('../views/DeviceDetailView.vue') },
    { path: '/health', name: 'health', component: () => import('../views/HealthView.vue') },
  ],
})
