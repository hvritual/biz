import type { RouteRecordRaw } from 'vue-router'

export const rentalWorkRoutes: RouteRecordRaw[] = [
  {
    path: '/rental',
    component: () => import('@/features/customer/pages/CustomerAreaView.vue'),
    meta: { module: 'rental' },
    children: [
      {
        path: '',
        redirect: '/rental/delivery',
      },
      {
        path: 'delivery',
        component: () => import('@/features/customer/pages/WorkItemsView.vue'),
        meta: { title: '投放交付', module: 'rental', workKind: 'delivery' },
      },
      {
        path: 'service',
        component: () => import('@/features/customer/pages/WorkItemsView.vue'),
        meta: { title: '服务恢复验证', module: 'rental', workKind: 'service' },
      },
      {
        path: 'payment',
        component: () => import('@/features/customer/pages/WorkItemsView.vue'),
        meta: { title: '回款跟进', module: 'rental', workKind: 'payment' },
      },
      {
        path: 'returns',
        component: () => import('@/features/customer/pages/WorkItemsView.vue'),
        meta: { title: '退租回收', module: 'rental', workKind: 'return' },
      },
    ],
  },
]
