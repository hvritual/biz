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
      component: () => import('@/features/platform/pages/PlatformOverviewView.vue'),
      meta: {
        title: '平台管理',
        module: 'platform-commercial',
        surface: 'platform',
        pageTemplate: 'WorkbenchPage',
      },
    },
    {
      path: '/platform/tenants',
      component: () => import('@/features/platform/pages/PlatformTenantsView.vue'),
      meta: {
        title: '租户管理',
        module: 'platform-commercial',
        surface: 'platform',
        pageTemplate: 'ListPage',
      },
    },
    {
      path: '/workspace/:resource(devices)',
      component: () => import('@/features/runtime/pages/RuntimeConsoleView.vue'),
      meta: { title: '业务设备', module: 'device-operations', surface: 'runtime' },
    },
    {
      path: '/workspace/:resource(members|roles)',
      component: () => import('@/features/runtime/pages/RuntimeConsoleView.vue'),
      meta: { title: '业务工作区', module: 'enterprise', surface: 'runtime' },
    },
    { path: '/', redirect: '/enterprise/members' },
    {
      path: '/dashboard',
      component: () => import('@/features/dashboard/pages/DashboardView.vue'),
      meta: { title: '工作台', module: 'dashboard' },
    },
    {
      path: '/platform/commercial/modules',
      component: () => import('@/features/platform/pages/CommercialModulesView.vue'),
      meta: { title: '模块目录', module: 'platform-commercial', surface: 'platform', pageTemplate: 'ListPage' },
    },
    {
      path: '/platform/commercial/plans',
      component: () => import('@/features/platform/pages/CommercialPlansView.vue'),
      meta: { title: '套餐版本', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage' },
    },
    {
      path: '/platform/commercial/tenant-entitlements',
      component: () => import('@/features/platform/pages/CommercialTenantEntitlementsView.vue'),
      meta: { title: '租户权益', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage' },
    },
    {
      path: '/platform/commercial/features',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '商业功能', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'features' },
    },
    {
      path: '/platform/commercial/add-ons',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '增购项', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'add-ons' },
    },
    {
      path: '/platform/commercial/subscriptions',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '租户订阅', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'subscriptions' },
    },
    {
      path: '/platform/commercial/changes',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '套餐变更', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'changes' },
    },
    {
      path: '/platform/commercial/expiry',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '到期与宽限', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'expiry' },
    },
    {
      path: '/platform/commercial/authorization',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '授权诊断', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'authorization' },
    },
    {
      path: '/platform/commercial/quotas',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '额度管理', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'quotas' },
    },
    {
      path: '/platform/commercial/overrides',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '专项授权', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'overrides' },
    },
    {
      path: '/platform/commercial/usage-billing',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '用量计费', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'usage-billing' },
    },
    {
      path: '/platform/commercial/audit',
      component: () => import('@/features/platform/pages/LifecycleManagementView.vue'),
      meta: { title: '商业审计', module: 'platform-commercial', surface: 'platform', pageTemplate: 'WorkbenchPage', lifecycleKey: 'audit' },
    },
    {
      path: '/enterprise/members',
      component: () => import('@/features/enterprise/pages/MembersView.vue'),
      meta: { title: '成员管理', module: 'enterprise', surface: 'tenant', pageTemplate: 'ListPage' },
    },
    {
      path: '/enterprise/roles',
      component: () => import('@/features/enterprise/pages/RolesView.vue'),
      meta: { title: '角色权限', module: 'enterprise', surface: 'tenant', pageTemplate: 'ListPage' },
    },
    {
      path: '/enterprise/organization',
      component: () => import('@/features/enterprise/pages/OrganizationView.vue'),
      meta: { title: '组织架构', module: 'enterprise', surface: 'tenant', pageTemplate: 'WorkbenchPage' },
    },
    {
      path: '/enterprise/plan',
      component: () => import('@/features/enterprise/pages/PlansView.vue'),
      meta: { title: '套餐额度', module: 'enterprise', surface: 'tenant', pageTemplate: 'WorkbenchPage' },
    },
    {
      path: '/enterprise/company',
      component: () => import('@/features/enterprise/pages/CompanyView.vue'),
      meta: { title: '企业信息', module: 'enterprise', surface: 'tenant', pageTemplate: 'FormPage' },
    },
    {
      path: '/enterprise/logs',
      component: () => import('@/features/enterprise/pages/AuditLogsView.vue'),
      meta: { title: '操作日志', module: 'enterprise', surface: 'tenant' },
    },
    {
      path: '/system/:section(general|notifications|security|integrations|dictionary)',
      component: () => import('@/features/system/pages/SettingsView.vue'),
      meta: { title: '系统设置', module: 'system', surface: 'tenant', pageTemplate: 'FormPage' },
    },
    {
      path: '/:pathMatch(.*)*',
      component: () => import('@/features/system/pages/NotFoundView.vue'),
      meta: { title: '页面不存在' },
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.afterEach((to) => {
  document.title = `${String(to.meta.title ?? '企业中心')} · CoffeeLink`
})
