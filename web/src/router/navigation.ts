export interface NavigationItem {
  id: string
  label: string
  icon: string
  path?: string
  matches?: string[]
  selectorId?: string
  group?: string
  groupId?: string
}

export function isPrimaryNavigationActive(item: NavigationItem, currentModule: unknown): boolean {
  if (typeof currentModule !== 'string') return false
  return item.id === currentModule || item.matches?.includes(currentModule) === true
}

export const primaryNavigation: NavigationItem[] = [
  { id: 'dashboard', label: '工作台', icon: 'home', path: '/dashboard' },
  {
    id: 'customer-operations',
    selectorId: 'customers',
    label: '客户经营',
    icon: 'customer',
    matches: ['customers', 'success'],
  },
  {
    id: 'rental-operations',
    selectorId: 'sites',
    label: '租赁运营',
    icon: 'file',
    matches: ['sites', 'rental'],
  },
  { id: 'device-operations', selectorId: 'devices', label: '设备运营', icon: 'device' },
  {
    id: 'business-operations',
    selectorId: 'orders',
    label: '经营管理',
    icon: 'chart',
    matches: ['orders', 'analytics'],
  },
  { id: 'enterprise', label: '企业中心', icon: 'company' },
  { id: 'platform-commercial', label: '平台管理', icon: 'crown' },
  { id: 'system', label: '系统设置', icon: 'settings' },
]

export const enterpriseNavigation: NavigationItem[] = [
  { id: 'members', label: '成员管理', icon: 'users', path: '/enterprise/members' },
  { id: 'roles', label: '角色权限', icon: 'shield', path: '/enterprise/roles' },
  { id: 'organization', label: '组织架构', icon: 'organization', path: '/enterprise/organization' },
  { id: 'plan', label: '套餐额度', icon: 'crown', path: '/enterprise/plan' },
  { id: 'company', label: '企业信息', icon: 'company', path: '/enterprise/company' },
  { id: 'logs', label: '操作日志', icon: 'file', path: '/enterprise/logs' },
]

export const platformCommercialNavigation: NavigationItem[] = [
  { id: 'overview', label: '平台总览', icon: 'home', path: '/platform/overview', group: '总览' },
  { id: 'tenants', label: '租户管理', icon: 'company', path: '/platform/tenants', group: '租户生命周期' },
  { id: 'subscriptions', label: '租户订阅', icon: 'file', path: '/platform/commercial/subscriptions', group: '租户生命周期' },
  { id: 'changes', label: '套餐变更', icon: 'refresh', path: '/platform/commercial/changes', group: '租户生命周期' },
  { id: 'expiry', label: '到期与宽限', icon: 'clock', path: '/platform/commercial/expiry', group: '租户生命周期' },
  { id: 'modules', label: '模块目录', icon: 'database', path: '/platform/commercial/modules', group: '产品与定价' },
  { id: 'features', label: '商业功能', icon: 'layers', path: '/platform/commercial/features', group: '产品与定价' },
  { id: 'plans', label: '套餐版本', icon: 'crown', path: '/platform/commercial/plans', group: '产品与定价' },
  { id: 'add-ons', label: '增购项', icon: 'plus', path: '/platform/commercial/add-ons', group: '产品与定价' },
  {
    id: 'tenant-entitlements',
    label: '租户权益',
    icon: 'shield',
    path: '/platform/commercial/tenant-entitlements',
    group: '权益与授权',
  },
  { id: 'authorization', label: '授权诊断', icon: 'checks', path: '/platform/commercial/authorization', group: '权益与授权' },
  { id: 'quotas', label: '额度管理', icon: 'database', path: '/platform/commercial/quotas', group: '权益与授权' },
  { id: 'overrides', label: '专项授权', icon: 'key', path: '/platform/commercial/overrides', group: '权益与授权' },
  { id: 'usage-billing', label: '用量计费', icon: 'chart', path: '/platform/commercial/usage-billing', group: '计量与治理' },
  { id: 'audit', label: '商业审计', icon: 'file', path: '/platform/commercial/audit', group: '计量与治理' },
]

export const platformCommercialQuickActions = [
  { label: '打开租户管理', icon: 'company', path: '/platform/tenants' },
  { label: '创建或发布套餐', icon: 'crown', path: '/platform/commercial/plans' },
  { label: '调整租户权益', icon: 'shield', path: '/platform/commercial/tenant-entitlements' },
  { label: '查看用量计费', icon: 'chart', path: '/platform/commercial/usage-billing' },
]

export const systemNavigation: NavigationItem[] = [
  { id: 'general', label: '基础设置', icon: 'settings', path: '/system/general' },
  { id: 'security', label: '安全设置', icon: 'shield', path: '/system/security' },
  { id: 'notifications', label: '通知设置', icon: 'bell', path: '/system/notifications' },
  { id: 'integrations', label: '接口与集成', icon: 'link', path: '/system/integrations' },
  { id: 'dictionary', label: '数据字典', icon: 'database', path: '/system/dictionary' },
]

export const systemQuickActions = [
  { label: '打开基础设置', icon: 'settings', path: '/system/general' },
  { label: '打开安全设置', icon: 'shield', path: '/system/security' },
  { label: '打开通知设置', icon: 'bell', path: '/system/notifications' },
  { label: '打开接口与集成', icon: 'link', path: '/system/integrations' },
]

export const quickActions = [
  { label: '新增成员', icon: 'plus', path: '/enterprise/members?action=create' },
  { label: '邀请成员', icon: 'invite', path: '/enterprise/members?action=invite' },
  { label: '新建角色', icon: 'shield', path: '/enterprise/roles?action=create' },
  { label: '调整套餐', icon: 'crown', path: '/enterprise/plan?action=upgrade' },
  { label: '编辑企业信息', icon: 'edit', path: '/enterprise/company' },
  { label: '查看操作日志', icon: 'file', path: '/enterprise/logs' },
]