import type { NavigationItem } from './navigation'
interface Domain {
  description: string
  links: NavigationItem[]
  actions: { label: string; icon: string; path: string }[]
}
export const customerDomains: Record<string, Domain> = {
  sites: {
    description: '管理点位履约、租赁规则与独立或共享计费',
    links: [
      { id: 'sites', label: '点位租赁', icon: 'site', path: '/sites' },
      { id: 'billing', label: '计费规则与共享组', icon: 'layers', path: '/sites/groups' },
      { id: 'statements', label: '租赁对账', icon: 'file', path: '/sites/statements' },
    ],
    actions: [
      { label: '新建点位', icon: 'plus', path: '/sites?action=new-site' },
      { label: '配置计费规则', icon: 'file', path: '/sites/groups?action=new-rule' },
      { label: '查看客户事项', icon: 'checks', path: '/customers/work' },
    ],
  },
  customers: {
    description: '以客户为中心，管理关系、风险与下一步行动',
    links: [
      { id: 'overview', label: '客户总览', icon: 'customer', path: '/customers' },
      { id: 'workspace', label: '客户工作区', icon: 'company', path: '/customers/accounts/CUS-0186' },
      { id: 'work', label: '客户事项', icon: 'checks', path: '/customers/work' },
      { id: 'contacts', label: '联系人与协作', icon: 'users', path: '/customers/contacts' },
      { id: 'risks', label: '客户风险', icon: 'warning', path: '/customers/risks' },
      { id: 'sharing', label: '客户共享', icon: 'shield', path: '/customers/sharing' },
    ],
    actions: [
      { label: '新建客户', icon: 'plus', path: '/customers?action=create-customer' },
      { label: '新建事项', icon: 'checks', path: '/customers/work?action=create-work' },
      { label: '记录回访', icon: 'phone', path: '/customers/work/CS-107?action=visit' },
      { label: '导入客户', icon: 'upload', path: '/customers/import' },
      { label: '查看通知', icon: 'bell', path: '/customers/notifications' },
    ],
  },
  success: {
    description: '围绕客户目标组织事项、规则与结果',
    links: [
      { id: 'plans', label: '经营计划', icon: 'calendar', path: '/customers/plans' },
      { id: 'board', label: '事项看板', icon: 'layers', path: '/customers/work?view=board' },
      { id: 'automation', label: '自动化规则', icon: 'activity', path: '/customers/automation' },
      { id: 'executions', label: '执行记录', icon: 'file', path: '/customers/executions' },
      { id: 'workflow', label: '流程与服务时限', icon: 'organization', path: '/customers/workflows' },
      { id: 'reports', label: '经营结果', icon: 'chart', path: '/customers/reports' },
    ],
    actions: [
      { label: '新建经营计划', icon: 'plus', path: '/customers/plans?action=create-plan' },
      { label: '新建自动化规则', icon: 'activity', path: '/customers/automation?action=create-rule' },
      { label: '调整服务时限', icon: 'clock', path: '/customers/sla?action=sla' },
      { label: '处理执行异常', icon: 'warning', path: '/customers/executions' },
    ],
  },
  rental: {
    description: '推进交付、回款、续约与退租回收',
    links: [
      { id: 'contracts', label: '合同与续约', icon: 'file', path: '/customers/contracts' },
      { id: 'site-billing', label: '点位计费与共享组', icon: 'site', path: '/sites/groups' },
      { id: 'delivery', label: '投放交付', icon: 'site', path: '/customers/work/CS-104' },
      { id: 'service', label: '服务恢复验证', icon: 'operations', path: '/customers/work/CS-103' },
      { id: 'payment', label: '回款跟进', icon: 'order', path: '/customers/work/CS-105' },
      { id: 'return', label: '退租回收', icon: 'back', path: '/customers/work/CS-106' },
    ],
    actions: [
      { label: '查看续约事项', icon: 'refresh', path: '/customers/work/CS-102' },
      { label: '查看客户验收', icon: 'checks', path: '/customers/client/SH-031' },
      { label: '查看经营结果', icon: 'chart', path: '/customers/reports' },
    ],
  },
}
