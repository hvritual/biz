import type { NavigationItem } from './navigation'

interface Domain {
  description: string
  links: NavigationItem[]
  actions: { label: string; icon: string; path: string }[]
}

const sitesDomain: Domain = {
  description: '管理点位履约、租赁规则与独立或共享计费',
  links: [
    { id: 'sites', label: '点位租赁', icon: 'site', path: '/sites', group: '投放与资产', groupId: 'deployment-assets' },
    {
      id: 'billing',
      label: '计费规则与共享组',
      icon: 'layers',
      path: '/sites/groups',
      group: '计费与结算', groupId: 'billing-settlement',
    },
    {
      id: 'statements',
      label: '租赁对账',
      icon: 'file',
      path: '/sites/statements',
      group: '计费与结算', groupId: 'billing-settlement',
    },
  ],
  actions: [
    { label: '新建点位', icon: 'plus', path: '/sites?action=new-site' },
    { label: '配置计费规则', icon: 'file', path: '/sites/groups?action=new-rule' },
    { label: '查看客户事项', icon: 'checks', path: '/customers/work' },
  ],
}

const customersDomain: Domain = {
  description: '以客户为中心，管理关系、风险与下一步行动',
  links: [
    { id: 'overview', label: '客户总览', icon: 'customer', path: '/customers', group: '客户管理', groupId: 'customer-management' },
    { id: 'contacts', label: '联系人与协作', icon: 'users', path: '/customers/contacts', group: '客户管理', groupId: 'customer-management' },
    { id: 'sharing', label: '客户共享', icon: 'shield', path: '/customers/sharing', group: '客户管理', groupId: 'customer-management' },
    { id: 'work', label: '客户事项', icon: 'checks', path: '/customers/work', group: '客户协同', groupId: 'customer-collaboration' },
    { id: 'risks', label: '客户风险', icon: 'warning', path: '/customers/risks', group: '客户协同', groupId: 'customer-collaboration' },
  ],
  actions: [
    { label: '新建客户', icon: 'plus', path: '/customers?action=create-customer' },
    { label: '新建事项', icon: 'checks', path: '/customers/work?action=create-work' },
    { label: '导入客户', icon: 'upload', path: '/customers/import' },
    { label: '查看通知', icon: 'bell', path: '/customers/notifications' },
  ],
}

const successDomain: Domain = {
  description: '围绕客户目标组织事项、规则与结果',
  links: [
    { id: 'plans', label: '经营计划', icon: 'calendar', path: '/customers/plans', group: '客户成功', groupId: 'customer-success' },
    { id: 'automation', label: '自动化规则', icon: 'activity', path: '/customers/automation', group: '客户成功', groupId: 'customer-success' },
    { id: 'executions', label: '执行记录', icon: 'file', path: '/customers/executions', group: '客户成功', groupId: 'customer-success' },
    { id: 'workflow', label: '流程与服务时限', icon: 'organization', path: '/customers/workflows', group: '客户成功', groupId: 'customer-success' },
    { id: 'reports', label: '经营结果', icon: 'chart', path: '/customers/reports', group: '客户成功', groupId: 'customer-success' },
  ],
  actions: [
    { label: '新建经营计划', icon: 'plus', path: '/customers/plans?action=create-plan' },
    { label: '新建自动化规则', icon: 'activity', path: '/customers/automation?action=create-rule' },
    { label: '调整服务时限', icon: 'clock', path: '/customers/sla?action=sla' },
    { label: '处理执行异常', icon: 'warning', path: '/customers/executions' },
  ],
}

const rentalDomain: Domain = {
  description: '推进交付、回款、续约与退租回收',
  links: [
    { id: 'contracts', label: '合同与续约', icon: 'file', path: '/customers/contracts', group: '投放与资产', groupId: 'deployment-assets' },
    { id: 'payment', label: '回款跟进', icon: 'order', path: '/rental/payment', group: '计费与结算', groupId: 'billing-settlement' },
    { id: 'delivery', label: '投放交付', icon: 'site', path: '/rental/delivery', group: '履约与服务', groupId: 'fulfillment-service' },
    { id: 'service', label: '服务恢复验证', icon: 'operations', path: '/rental/service', group: '履约与服务', groupId: 'fulfillment-service' },
    { id: 'return', label: '退租回收', icon: 'back', path: '/rental/returns', group: '履约与服务', groupId: 'fulfillment-service' },
  ],
  actions: [
    { label: '查看合同与续约', icon: 'refresh', path: '/customers/contracts' },
    { label: '查看投放交付', icon: 'checks', path: '/rental/delivery' },
    { label: '查看经营结果', icon: 'chart', path: '/customers/reports' },
  ],
}

export const customerDomains: Record<string, Domain> = {
  sites: sitesDomain,
  customers: customersDomain,
  success: successDomain,
  rental: rentalDomain,
  'customer-operations': {
    description: '聚合客户档案、协同事项、风险、经营计划与自动化执行',
    links: [...customersDomain.links, ...successDomain.links],
    actions: [
      { label: '新建客户', icon: 'plus', path: '/customers?action=create-customer' },
      { label: '新建事项', icon: 'checks', path: '/customers/work?action=create-work' },
      { label: '新建经营计划', icon: 'calendar', path: '/customers/plans?action=create-plan' },
      { label: '新建自动化规则', icon: 'activity', path: '/customers/automation?action=create-rule' },
      { label: '导入客户', icon: 'upload', path: '/customers/import' },
      { label: '查看通知', icon: 'bell', path: '/customers/notifications' },
    ],
  },
  'rental-operations': {
    description: '聚合点位、合同、计费结算、交付服务与退租流程',
    links: [...sitesDomain.links, ...rentalDomain.links],
    actions: [
      { label: '新建点位', icon: 'plus', path: '/sites?action=new-site' },
      { label: '配置计费规则', icon: 'file', path: '/sites/groups?action=new-rule' },
      { label: '查看合同与续约', icon: 'refresh', path: '/customers/contracts' },
      { label: '查看投放交付', icon: 'checks', path: '/rental/delivery' },
      { label: '查看经营结果', icon: 'chart', path: '/customers/reports' },
    ],
  },
  'device-operations': {
    description: '统一承载设备资产、设备配置、远程运维与故障处理入口',
    links: [
      { id: 'devices', label: '设备管理', icon: 'device', path: '/workspace/devices', group: '设备资产', groupId: 'device-assets' },
      { id: 'drinks', label: '饮品配置', icon: 'coffee', group: '设备配置', groupId: 'device-configuration' },
      { id: 'remote', label: '远程运维', icon: 'operations', group: '远程运维', groupId: 'remote-operations' },
      { id: 'tickets', label: '故障工单', icon: 'ticket', group: '服务维护', groupId: 'service-maintenance' },
    ],
    actions: [],
  },
  'business-operations': {
    description: '统一承载订单与经营数据分析入口',
    links: [
      { id: 'orders', label: '订单管理', icon: 'order', group: '交易管理', groupId: 'transaction-management' },
      { id: 'analytics', label: '数据分析', icon: 'chart', group: '经营分析', groupId: 'business-analytics' },
    ],
    actions: [],
  },
}
