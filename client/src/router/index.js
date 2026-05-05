import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../pages/auth/LoginPage.vue'),
    meta: { guestOnly: true }
  },
  {
    path: '/register',
    name: 'register',
    component: () => import('../pages/auth/RegisterPage.vue'),
    meta: { guestOnly: true }
  },
  {
    path: '/',
    name: 'dashboard',
    component: () => import('../pages/DashboardPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/tenders',
    name: 'tenders',
    component: () => import('../pages/tenders/TenderListPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/tenders/:id',
    name: 'tender-details',
    component: () => import('../pages/tenders/TenderDetailsPage.vue'),
    props: true,
    meta: { requiresAuth: true }
  },
  {
    path: '/companies',
    name: 'companies',
    component: () => import('../pages/companies/CompanyListPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/companies/new',
    name: 'company-create',
    component: () => import('../pages/companies/CompanyFormPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/companies/:id/edit',
    name: 'company-edit',
    component: () => import('../pages/companies/CompanyFormPage.vue'),
    props: true,
    meta: { requiresAuth: true }
  },
  {
    path: '/analysis/new',
    name: 'analysis-prepare',
    component: () => import('../pages/analysis/AnalysisPreparePage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/analysis/history',
    name: 'analysis-history',
    component: () => import('../pages/analysis/AnalysisHistoryPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/analysis/results/:id',
    name: 'analysis-result',
    component: () => import('../pages/analysis/AnalysisResultPage.vue'),
    props: true,
    meta: { requiresAuth: true }
  },
  {
    path: '/integration',
    name: 'integration-log',
    component: () => import('../pages/integration/IntegrationLogPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/profile',
    name: 'profile',
    component: () => import('../pages/profile/ProfilePage.vue'),
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.initialized) {
    try {
      await auth.fetchMe()
    } catch {
      // ignore
    }
  }

  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  if (to.meta.guestOnly && auth.isAuthenticated) {
    return { name: 'dashboard' }
  }

  return true
})

export default router
