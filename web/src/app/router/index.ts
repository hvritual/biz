import { createRouter, createWebHashHistory } from "vue-router";
const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: "/", redirect: "/enterprise/members" },
    {
      path: "/workbench",
      component: () => import("@/features/workbench/pages/WorkbenchPage.vue"),
      meta: { section: "home" },
    },
    {
      path: "/enterprise/members",
      component: () => import("@/features/members/pages/MemberListPage.vue"),
      meta: { section: "enterprise" },
    },
    {
      path: "/enterprise/members/:id",
      component: () => import("@/features/members/pages/MemberDetailPage.vue"),
      meta: { section: "enterprise" },
    },
    {
      path: "/enterprise/roles",
      component: () => import("@/features/enterprise/pages/RolesPage.vue"),
      meta: { section: "enterprise" },
    },
    {
      path: "/enterprise/organization",
      component: () => import("@/features/enterprise/pages/OrganizationPage.vue"),
      meta: { section: "enterprise" },
    },
    {
      path: "/enterprise/plan",
      component: () => import("@/features/enterprise/pages/PlanPage.vue"),
      meta: { section: "enterprise" },
    },
    {
      path: "/enterprise/profile",
      component: () => import("@/features/enterprise/pages/CompanyPage.vue"),
      meta: { section: "enterprise" },
    },
    {
      path: "/enterprise/audit",
      component: () => import("@/features/enterprise/pages/AuditPage.vue"),
      meta: { section: "enterprise" },
    },
    {
      path: "/settings/:tab",
      component: () => import("@/features/settings/pages/SettingsPage.vue"),
      meta: { section: "settings" },
    },
    { path: "/:pathMatch(.*)*", redirect: "/enterprise/members" },
  ],
  scrollBehavior: () => ({ top: 0 }),
});
export default router;
