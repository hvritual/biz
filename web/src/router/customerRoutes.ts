import type { RouteRecordRaw } from 'vue-router'
export const customerRoutes: RouteRecordRaw[] = [
  {
    path: '/customers',
    component: () => import('@/features/customer/pages/CustomerAreaView.vue'),
    meta: { module: 'customers' },
    children: [
      {
        path: '',
        component: () => import('@/features/customer/pages/CustomersView.vue'),
        meta: { title: '客户总览' },
      },
      {
        path: 'accounts/:id',
        component: () => import('@/features/customer/pages/WorkspaceView.vue'),
        meta: { title: '客户工作区' },
      },
      {
        path: 'contacts',
        component: () => import('@/features/customer/pages/ContactsView.vue'),
        meta: { title: '联系人与协作' },
      },
      {
        path: 'work',
        component: () => import('@/features/customer/pages/WorkItemsView.vue'),
        meta: { title: '客户事项' },
      },
      {
        path: 'work/:id',
        component: () => import('@/features/customer/pages/WorkDetailView.vue'),
        meta: { title: '客户事项详情' },
      },
      {
        path: 'plans',
        component: () => import('@/features/customer/pages/PlansView.vue'),
        meta: { title: '经营计划', module: 'success' },
      },
      {
        path: 'plans/:id',
        component: () => import('@/features/customer/pages/PlanDetailView.vue'),
        meta: { title: '经营计划详情', module: 'success' },
      },
      {
        path: 'contracts',
        component: () => import('@/features/customer/pages/ContractsView.vue'),
        meta: { title: '合同与续约', module: 'rental' },
      },
      {
        path: 'risks',
        component: () => import('@/features/customer/pages/RisksView.vue'),
        meta: { title: '客户风险' },
      },
      {
        path: 'automation',
        component: () => import('@/features/customer/pages/AutomationView.vue'),
        meta: { title: '自动化规则', module: 'success' },
      },
      {
        path: 'executions',
        component: () => import('@/features/customer/pages/ExecutionsView.vue'),
        meta: { title: '自动化执行记录', module: 'success' },
      },
      {
        path: 'workflows',
        component: () => import('@/features/customer/pages/WorkflowsView.vue'),
        meta: { title: '流程模板与版本', module: 'success' },
      },
      {
        path: 'sla',
        component: () => import('@/features/customer/pages/SlaView.vue'),
        meta: { title: '服务时限', module: 'success' },
      },
      {
        path: 'sharing',
        component: () => import('@/features/customer/pages/SharingView.vue'),
        meta: { title: '客户共享' },
      },
      {
        path: 'client/:id',
        component: () => import('@/features/customer/pages/ClientPortalView.vue'),
        meta: { title: '客户侧交付确认' },
      },
      {
        path: 'reports',
        component: () => import('@/features/customer/pages/ReportsView.vue'),
        meta: { title: '客户经营结果', module: 'success' },
      },
      {
        path: 'notifications',
        component: () => import('@/features/customer/pages/NotificationsView.vue'),
        meta: { title: '通知与待决策' },
      },
      {
        path: 'restricted',
        component: () => import('@/features/customer/pages/RestrictedView.vue'),
        meta: { title: '财务访问受限' },
      },
      {
        path: 'import',
        component: () => import('@/features/customer/pages/ImportView.vue'),
        meta: { title: '客户导入' },
      },
    ],
  },
]
