import { siteRentalRoutes } from './siteRentalRoutes'
import { customerRoutes } from './customerRoutes'
import { rentalWorkRoutes } from './rentalWorkRoutes'
import { createRouter, createWebHashHistory } from 'vue-router'
import {
  authorizationApiMode,
  currentAuthorizationAllowsAny,
  currentAuthorizationState,
  ensureCurrentAuthorization,
  redirectToTrustedLogin,
} from '@/services/runtime/authorization'

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
      meta: { title: '业务设备', module: 'device-operations', surface: 'runtime', authorizationActions: ['device.list'] },
    },
    {
      path: '/workspace/:resource(members|roles)',
      component: () => import('@/features/runtime/pages/RuntimeConsoleView.vue'),
      meta: { title: '业务工作区', module: 'enterprise', surface: 'runtime', authorizationActions: ['tenant.member.list', 'tenant.role.list'] },
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
      path: '/enterprise/personal-profile',
      component: () => import('@/features/enterprise/pages/PersonalProfileView.vue'),
      meta: { title: '个人中心', module: 'enterprise', surface: 'tenant', pageTemplate: 'FormPage', authorizationActions: ['tenant.member.personal_profile.get'] },
    },
    {
      path: '/enterprise/members',
      component: () => import('@/features/enterprise/pages/MembersView.vue'),
      meta: { title: '成员管理', module: 'enterprise', surface: 'tenant', pageTemplate: 'ListPage', authorizationActions: ['tenant.member.list'] },
    },
    {
      path: '/enterprise/roles',
      component: () => import('@/features/enterprise/pages/RolesView.vue'),
      meta: { title: '角色权限', module: 'enterprise', surface: 'tenant', pageTemplate: 'ListPage', authorizationActions: ['tenant.role.list'] },
    },
    {
      path: '/enterprise/organization',
      component: () => import('@/features/enterprise/pages/OrganizationView.vue'),
      meta: { title: '组织架构', module: 'enterprise', surface: 'tenant', pageTemplate: 'WorkbenchPage', authorizationActions: ['tenant.department.list'] },
    },
    {
      path: '/enterprise/plan',
      component: () => import('@/features/enterprise/pages/PlansView.vue'),
      meta: { title: '套餐额度', module: 'enterprise', surface: 'tenant', pageTemplate: 'WorkbenchPage', authorizationActions: ['commercial.subscription.get_my'] },
    },
    {
      path: '/enterprise/company',
      component: () => import('@/features/enterprise/pages/CompanyView.vue'),
      meta: { title: '企业信息', module: 'enterprise', surface: 'tenant', pageTemplate: 'FormPage', authorizationActions: ['tenant.profile.get'] },
    },
    {
      path: '/enterprise/branding',
      component: () => import('@/features/enterprise/pages/BrandingView.vue'),
      meta: { title: '品牌与主题', module: 'enterprise', surface: 'tenant', pageTemplate: 'FormPage', authorizationActions: ['tenant.branding.get'] },
    },
    {
      path: '/enterprise/logs',
      component: () => import('@/features/enterprise/pages/AuditLogsView.vue'),
      meta: { title: '操作日志', module: 'enterprise', surface: 'tenant', pageTemplate: 'ListPage', authorizationActions: ['access.audit.list'] },
    },
    {
      path: '/system/:section(general|notifications|security|integrations|dictionary)',
      component: () => import('@/features/system/pages/SettingsView.vue'),
      meta: { title: '系统设置', module: 'system', surface: 'tenant', pageTemplate: 'FormPage' },
    },
    {
      path: '/member-appeal',
      component: () => import('@/features/enterprise/pages/MemberAppealView.vue'),
      meta: { title: '成员访问申诉', authorizationPublic: true },
    },
    {
      path: '/authorization-state',
      component: () => import('@/features/system/pages/AuthorizationStateView.vue'),
      meta: { title: '访问授权', authorizationPublic: true },
    },
    {
      path: '/:pathMatch(.*)*',
      component: () => import('@/features/system/pages/NotFoundView.vue'),
      meta: { title: '页面不存在' },
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  const accountSecurityRoute = to.meta.module === 'system' && to.params.section === 'security'
  if (!authorizationApiMode() || to.meta.authorizationPublic || accountSecurityRoute) return true
  const required = Array.isArray(to.meta.authorizationActions)
    ? to.meta.authorizationActions.filter((value): value is string => typeof value === 'string' && value.length > 0)
    : []
  const moduleCode = typeof to.meta.module === 'string' ? to.meta.module : ''
  const tenantModules = new Set(['customers', 'success', 'sites', 'rental', 'device-operations', 'enterprise', 'system'])
  if (!required.length && tenantModules.has(moduleCode)) {
    return { path: '/authorization-state', query: { reason: 'forbidden', from: to.fullPath } }
  }
  if (!required.length) return true

  await ensureCurrentAuthorization()
  if (currentAuthorizationState.status === 'unauthenticated') {
    redirectToTrustedLogin()
    return false
  }
  if (currentAuthorizationState.status === 'error') {
    return { path: '/authorization-state', query: { reason: 'unavailable', from: to.fullPath } }
  }
  if (currentAuthorizationState.status !== 'ready' || !currentAuthorizationAllowsAny(required)) {
    return { path: '/authorization-state', query: { reason: 'forbidden', from: to.fullPath } }
  }
  return true
})

router.afterEach((to) => {
  document.title = `${String(to.meta.title ?? '企业中心')} · CoffeeLink`
})
