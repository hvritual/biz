import type { RouteRecordRaw } from 'vue-router'
export const siteRentalRoutes: RouteRecordRaw[] = [
  {
    path: '/sites',
    component: () => import('@/views/customer/CustomerAreaView.vue'),
    meta: { module: 'sites' },
    children: [
      { path: '', component: () => import('@/views/siteRental/SitesView.vue'), meta: { title: '点位租赁' } },
      {
        path: 'groups/:groupId?',
        component: () => import('@/views/siteRental/GroupsView.vue'),
        meta: { title: '计费规则与共享组' },
      },
      {
        path: 'statements',
        component: () => import('@/views/siteRental/StatementsView.vue'),
        meta: { title: '租赁对账' },
      },
      {
        path: ':id',
        component: () => import('@/views/siteRental/SiteDetailView.vue'),
        meta: { title: '点位工作区' },
      },
    ],
  },
]
