import type { RouteRecordRaw } from 'vue-router'
export const customerRoutes: RouteRecordRaw[] = [
  {
    path: '/customers',
    component: () => import('@/views/customer/CustomerAreaView.vue'),
    meta: { module: 'customers' },
    children: [
      {
        path: '',
        component: () => import('@/views/customer/CustomersView.vue'),
        meta: { title: '客户总览' },
      },
      {
        path: 'accounts/:id',
        component: () => import('@/views/customer/WorkspaceView.vue'),
        meta: { title: '客户工作区' },
      },
      {
        path: 'contacts',
        component: () => import('@/views/customer/ContactsView.vue'),
        meta: { title: '联系人与协作' },
      },
      {
        path: 'work',
        component: () => import('@/views/customer/WorkItemsView.vue'),
        meta: { title: '客户事项' },
      },
      {
        path: 'work/:id',
        component: () => import('@/views/customer/WorkDetailView.vue'),
        meta: { title: '客户事项详情' },
      },
      {
        path: 'plans',
        component: () => import('@/views/customer/PlansView.vue'),
        meta: { title: '经营计划', module: 'success' },
      },
      {
        path: 'plans/:id',
        component: () => import('@/views/customer/PlanDetailView.vue'),
        meta: { title: '经营计划详情', module: 'success' },
      },
      {
        path: 'contracts',
        component: () => import('@/views/customer/ContractsView.vue'),
        meta: { title: '合同与续约', module: 'rental' },
      },
      {
        path: 'risks',
        component: () => import('@/views/customer/RisksView.vue'),
        meta: { title: '客户风险' },
      },
      {
        path: 'automation',
        component: () => import('@/views/customer/AutomationView.vue'),
        meta: { title: '自动化规则', module: 'success' },
      },
      {
        path: 'executions',
        component: () => import('@/views/customer/ExecutionsView.vue'),
        meta: { title: '自动化执行记录', module: 'success' },
      },
      {
        path: 'workflows',
        component: () => import('@/views/customer/WorkflowsView.vue'),
        meta: { title: '流程模板与版本', module: 'success' },
      },
      {
        path: 'sla',
        component: () => import('@/views/customer/SlaView.vue'),
        meta: { title: '服务时限', module: 'success' },
      },
      {
        path: 'sharing',
        component: () => import('@/views/customer/SharingView.vue'),
        meta: { title: '客户共享' },
      },
      {
        path: 'client/:id',
        component: () => import('@/views/customer/ClientPortalView.vue'),
        meta: { title: '客户侧交付确认' },
      },
      {
        path: 'reports',
        component: () => import('@/views/customer/ReportsView.vue'),
        meta: { title: '客户经营结果', module: 'success' },
      },
      {
        path: 'notifications',
        component: () => import('@/views/customer/NotificationsView.vue'),
        meta: { title: '通知与待决策' },
      },
      {
        path: 'restricted',
        component: () => import('@/views/customer/RestrictedView.vue'),
        meta: { title: '财务访问受限' },
      },
      {
        path: 'import',
        component: () => import('@/views/customer/ImportView.vue'),
        meta: { title: '客户导入' },
      },
    ],
  },
]
