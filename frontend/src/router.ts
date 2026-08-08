import { createRouter, createWebHistory } from 'vue-router'
import { auth } from '@/auth'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    public?: boolean
    permission?: string
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { title: '登录', public: true } },
    { path: '/change-password', name: 'change-password', component: () => import('@/views/ChangePasswordView.vue'), meta: { title: '设置密码' } },
    { path: '/oauth/authorize', name: 'oauth-authorize', component: () => import('@/views/OAuthAuthorizeView.vue'), meta: { title: '应用授权' } },
    {
      path: '/',
      component: () => import('@/layouts/AppLayout.vue'),
      children: [
        { path: '', redirect: '/departures' },
        { path: 'dashboard', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: '人事概览', permission: 'people.dashboard:view' } },
        { path: 'employees', name: 'employees', component: () => import('@/views/EmployeesView.vue'), meta: { title: '员工管理', permission: 'people.employee:view' } },
        { path: 'departments', name: 'departments', component: () => import('@/views/DepartmentsView.vue'), meta: { title: '部门管理', permission: 'people.department:manage' } },
        { path: 'departures', name: 'departures', component: () => import('@/views/DeparturesView.vue'), meta: { title: '离职审批' } },
        { path: 'profile', name: 'profile', component: () => import('@/views/ProfileView.vue'), meta: { title: '个人资料' } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  document.title = to.meta.title ? `${to.meta.title} - People` : 'People'
  await auth.hydrate()
  if (to.meta.public) {
    if (auth.authenticated.value && to.name === 'login') return auth.state.user?.mustChangePassword ? '/change-password' : '/'
    return true
  }
  if (!auth.authenticated.value) return { name: 'login', query: { redirect: to.fullPath } }
  if (auth.state.user?.mustChangePassword && to.name !== 'change-password') return { name: 'change-password', query: { redirect: to.fullPath } }
  if (!auth.state.user?.mustChangePassword && to.name === 'change-password') return '/'
  if (to.meta.permission && !auth.can(to.meta.permission)) return '/departures'
  return true
})

export default router
