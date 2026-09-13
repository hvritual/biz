export interface NavigationItem {
  id: string
  label: string
  icon: string
  path?: string
  matches?: string[]
}

export function isPrimaryNavigationActive(item: NavigationItem, currentModule: unknown): boolean {
  if (typeof currentModule !== 'string') return false
  return item.id === currentModule || item.matches?.includes(currentModule) === true
}

export const primaryNavigation: NavigationItem[] = [
  { id: 'dashboard', label: '工作台', icon: 'home', path: '/dashboard' },
  {
    id: 'customer-operations',
    label: '客户经营',
    icon: 'customer',
    matches: ['customers', 'success'],
  },
  {
    id: 'rental-operations',
    label: '租赁运营',
    icon: 'file',
    matches: ['sites', 'rental'],
  },
  { id: 'device-operations', label: '设备运营', icon: 'device' },
  {
    id: 'business-operations',
    label: '经营管理',
    icon: 'chart',
    matches: ['orders', 'drinks', 'analytics'],
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
  { id: 'overview', label: '平台总览', icon: 'home', path: '/platform/overview' },
  { id: 'tenants', label: '租户管理', icon: 'company', path: '/platform/tenants' },
  { id: 'modules', label: '模块目录', icon: 'database', path: '/platform/commercial/modules' },
  { id: 'plans', label: '套餐版本', icon: 'crown', path: '/platform/commercial/plans' },
  {
    id: 'tenant-entitlements',
    label: '租户权益',
    icon: 'shield',
    path: '/platform/commercial/tenant-entitlements',
  },
]

export const platformCommercialQuickActions = [
  { label: '打开租户管理', icon: 'company', path: '/platform/tenants' },
  { label: '管理模块状态', icon: 'settings', path: '/platform/commercial/modules' },
  { label: '创建或发布套餐', icon: 'crown', path: '/platform/commercial/plans' },
  { label: '调整租户权益', icon: 'shield', path: '/platform/commercial/tenant-entitlements' },
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
