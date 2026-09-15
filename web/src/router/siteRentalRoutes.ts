import type { RouteRecordRaw } from 'vue-router'
export const siteRentalRoutes: RouteRecordRaw[] = [
  {
    path: '/sites',
    component: () => import('@/features/customer/pages/CustomerAreaView.vue'),
    meta: { module: 'sites' },
    children: [
      { path: '', component: () => import('@/features/site-rental/pages/SitesView.vue'), meta: { title: '点位租赁' } },
      {
        path: 'groups/:groupId?',
        component: () => import('@/features/site-rental/pages/GroupsView.vue'),
        meta: { title: '计费规则与共享组' },
      },
      {
        path: 'statements',
        component: () => import('@/features/site-rental/pages/StatementsView.vue'),
        meta: { title: '租赁对账' },
      },
      {
        path: ':id',
        component: () => import('@/features/site-rental/pages/SiteDetailView.vue'),
        meta: { title: '点位工作区' },
      },
    ],
  },
]
