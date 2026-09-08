import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/chat' },
    { path: '/chat', name: 'chat', component: () => import('@/views/Chat.vue') },
    { path: '/kb', name: 'kb', component: () => import('@/views/KnowledgeBases.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/Settings.vue') },
  ],
})

export default router