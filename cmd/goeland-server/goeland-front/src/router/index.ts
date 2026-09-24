/**
 * router/index.ts — case, document and actor routes for the Goéland POC.
 */
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/cases' },
    {
      path: '/documents',
      name: 'documents',
      component: () => import('@/pages/documents/DocumentListPage.vue'),
    },
    {
      path: '/documents/new',
      name: 'document-create',
      component: () => import('@/pages/documents/DocumentCreatePage.vue'),
    },
    {
      path: '/documents/:id',
      name: 'document-detail',
      component: () => import('@/pages/documents/DocumentDetailPage.vue'),
    },
    {
      path: '/cases',
      name: 'cases',
      component: () => import('@/pages/cases/CaseListPage.vue'),
    },
    {
      path: '/cases/new',
      name: 'case-create',
      component: () => import('@/pages/cases/CaseCreatePage.vue'),
    },
    {
      path: '/cases/:id',
      name: 'case-detail',
      component: () => import('@/pages/cases/CaseDetailPage.vue'),
    },
    {
      path: '/actors',
      name: 'actors',
      component: () => import('@/pages/actors/ActorListPage.vue'),
    },
    {
      path: '/actors/new',
      name: 'actor-create',
      component: () => import('@/pages/actors/ActorCreatePage.vue'),
    },
    {
      path: '/actors/:id',
      name: 'actor-detail',
      component: () => import('@/pages/actors/ActorDetailPage.vue'),
    },
  ],
})

export default router
