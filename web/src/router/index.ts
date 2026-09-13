import { siteRentalRoutes } from './siteRentalRoutes'
import { customerRoutes } from './customerRoutes'
import { rentalWorkRoutes } from './rentalWorkRoutes'
import { createRouter, createWebHashHistory } from 'vue-router'
export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    ...customerRoutes,
    ...siteRentalRoutes,
    ...rentalWorkRoutes,
    {
      path: '/platform/overview',
      component: () => import('@/views/platform/PlatformOverviewView.vue'),
      meta: {
        title: '平台管理',
        module: 'platform-commercial',
        surface: 'platform',
        pageTemplate: 'WorkbenchPage',
      },
    },
    {
      path: '/platform/tenants',
      component: () => import('@/views/platform/PlatformTenantsView.vue'),
      meta: {
        title: '租户管理',
        module: 'platform-commercial',
        surface: 'platform',
        pageTemplate: 'ListPage',
      },
    },
    {
      path: '/workspace/:resource(devices)',
      component: () => import('@/views/runtime/RuntimeConsoleView.vue'),
      meta: { title: '业务设备', module: 'device-operations', surface: 'runtime' },
    },
    {
      path: '/workspace/:resource(members|roles)',
      component: () => import('@/views/runtime/RuntimeConsoleView.vue'),
      meta: { title: '业务工作区', module: 'enterprise', surface: 'runtime' },
    },
    { path: '/', redirect: '/enterprise/members' },
    {
      path: '/dashboard',
      component: () => import('@/views/dashboard/DashboardView.vue'),
      meta: { title: '工作台', module: 'dashboard' },
    },
    {
      path: '/platform/commercial/modules',
      component: () => import('@/views/platform/CommercialModulesView.vue'),
      meta: { title: '模块目录', module: 'platform-commercial', surface: 'platform', pageTemplate: 'ListPage' },
    },
    {
      path: '/platform/commercial/plans',
      component: () => import('@/views/platform/CommercialPlansView.vue'),
      meta: { title: '套餐版本', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage' },
    },
    {
      path: '/platform/commercial/tenant-entitlements',
      component: () => import('@/views/platform/CommercialTenantEntitlementsView.vue'),
      meta: { title: '租户权益', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage' },
    },
    {
      path: '/enterprise/members',
      component: () => import('@/views/enterprise/MembersEntryView.vue'),
      meta: { title: '成员管理', module: 'enterprise' },
    },
    {
      path: '/enterprise/roles',
      component: () => import('@/views/enterprise/RolesEntryView.vue'),
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
      meta: { title: '套餐额度', module: 'enterprise' },
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
      meta: { title: '系统设置', module: 'system', surface: 'tenant', pageTemplate: 'FormPage' },
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
