import { customerRoutes } from './customerRoutes'
import { createRouter, createWebHashHistory } from 'vue-router'
export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    ...customerRoutes,
    { path: '/', redirect: '/enterprise/members' },
    {
      path: '/dashboard',
      component: () => import('@/views/dashboard/DashboardView.vue'),
      meta: { title: '工作台', module: 'dashboard' },
    },
    {
      path: '/enterprise/members',
      component: () => import('@/views/enterprise/MembersView.vue'),
      meta: { title: '成员管理', module: 'enterprise' },
    },
    {
      path: '/enterprise/roles',
      component: () => import('@/views/enterprise/RolesView.vue'),
      meta: { title: '角色权限', module: 'enterprise' },
    },
    {
      path: '/enterprise/organization',
      component: () => import('@/views/enterprise/OrganizationView.vue'),
      meta: { title: '组织架构', module: 'enterprise' },
    },
    {
      path: '/enterprise/plan',
      component: () => import('@/views/enterprise/PlansView.vue'),
      meta: { title: '套餐信息', module: 'enterprise' },
    },
    {
      path: '/enterprise/company',
      component: () => import('@/views/enterprise/CompanyView.vue'),
      meta: { title: '企业信息', module: 'enterprise' },
    },
    {
      path: '/enterprise/logs',
      component: () => import('@/views/enterprise/AuditLogsView.vue'),
      meta: { title: '操作日志', module: 'enterprise' },
    },
    {
      path: '/system/:section(general|notifications|security|integrations|dictionary)',
      component: () => import('@/views/system/SettingsView.vue'),
      meta: { title: '系统设置', module: 'system' },
    },
    {
      path: '/:pathMatch(.*)*',
      component: () => import('@/views/NotFoundView.vue'),
      meta: { title: '页面不存在' },
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
})
router.afterEach((to) => {
  document.title = `${String(to.meta.title ?? '企业中心')} · CoffeeLink`
})
