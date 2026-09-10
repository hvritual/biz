import type { TenantSnapshot, Member, Role } from '@/types/enterprise'
export const permissionCatalog = [
  { id: 'tenant.member', name: '成员管理', actions: ['read', 'manage'] },
  { id: 'tenant.role', name: '角色权限', actions: ['read', 'manage', 'assign'] },
  { id: 'device', name: '设备管理', actions: ['read', 'manage', 'export'] },
  { id: 'site', name: '点位管理', actions: ['read', 'manage'] },
  { id: 'customer', name: '客户管理', actions: ['read', 'manage', 'export'] },
  { id: 'workorder', name: '故障工单', actions: ['read', 'manage', 'assign'] },
  { id: 'analytics', name: '数据分析', actions: ['read', 'export'] },
  { id: 'audit', name: '操作日志', actions: ['read', 'export'] },
]
const allPermissions = permissionCatalog.flatMap((p) => p.actions.map((a) => `${p.id}.${a}`))
const roleNames = [
  '超级管理员',
  '运营管理员',
  '普通成员',
  '运维工程师',
  '销售经理',
  '客服专员',
  '财务专员',
  '数据分析师',
  '区域经理',
  '门店管理员',
  '审计专员',
  '访客',
]
export function createSeed(tenant: string): TenantSnapshot {
  const alternate = tenant !== 'shanghai'
  const roles: Role[] = roleNames.map((name, i) => ({
    id: i === 0 ? 'owner' : `role-${i}`,
    name,
    description:
      ['拥有企业管理权限，受最后一位所有者保护', '负责设备、点位与客户日常运营', '查看已授权的企业业务数据'][
        i
      ] ?? '根据岗位职责授予必要权限',
    builtin: [0, 2, 3, 10, 11].includes(i),
    enabled: true,
    scope: i === 0 ? 'all' : 'department',
    permissions:
      i === 0
        ? [...allPermissions]
        : i === 1
          ? allPermissions.filter((p) => !p.startsWith('tenant.role'))
          : allPermissions.filter((p) => p.endsWith('.read')),
    updatedAt: '2026-09-08 10:32',
  }))
  const departments = [
    { id: 'product', name: '产品研发部', parentId: null },
    { id: 'operations', name: '运营管理部', parentId: null },
    { id: 'market', name: '市场部', parentId: null },
    { id: 'success', name: '客户成功部', parentId: 'operations' },
    { id: 'support', name: '技术支持部', parentId: null },
    { id: 'finance', name: '财务部', parentId: null },
    { id: 'hr', name: '人事部', parentId: null },
    { id: 'service', name: '售后服务部', parentId: 'support' },
    { id: 'east', name: '华东运营组', parentId: 'operations' },
    { id: 'south', name: '华南运营组', parentId: 'operations' },
    { id: 'development', name: '平台开发组', parentId: 'product' },
    { id: 'quality', name: '质量保障组', parentId: 'product' },
  ].map((d, i) => ({
    ...d,
    leaderId: `member-${i + 1}`,
    code: `DEP${String(i + 1).padStart(3, '0')}`,
    description: '负责本部门的业务协同、资源管理与服务支持。',
    enabled: true,
  }))
  const names = [
    '张三',
    '李四',
    '王五',
    '刘佳',
    '陈晨',
    '赵六',
    '孙七',
    '周八',
    '林悦',
    '徐明',
    '吴桐',
    '郑青',
    '何静',
    '高远',
    '唐宁',
    '沈涵',
  ]
  const emails = ['zhangsan', 'lisi', 'wangwu', 'liujia', 'chenchen', 'zhaoliu', 'sunqi', 'zhouba']
  const count = alternate ? 24 : 368
  const members: Member[] = Array.from({ length: count }, (_, i) => ({
    id: `member-${i + 1}`,
    name: names[i % names.length]! + (i >= names.length ? ` ${i + 1}` : ''),
    email: `${emails[i] ?? `member${i + 1}`}@example.com`,
    phone: `138${String(10000000 + i)}`,
    employeeId: `CL${String(20260001 + i)}`,
    departmentId: departments[i % departments.length]!.id,
    position: i === 0 ? '企业负责人' : i === 1 ? '运营主管' : '业务专员',
    roleIds: i === 0 ? ['owner'] : i === 1 || i % 5 === 3 ? ['role-1'] : ['role-2'],
    scope: i === 0 ? 'all' : 'department',
    status:
      i >= count - 18 && !alternate ? 'invited' : i >= count - 30 && !alternate ? 'suspended' : 'active',
    online: i < (alternate ? 18 : 315) && ![2, 4, 6].includes(i),
    joinedAt: '2026-02-18',
    lastLogin:
      i >= count - 18 && !alternate
        ? null
        : `2026-09-0${8 - (i % 3)} ${['14:05', '11:23', '18:40', '09:12', '16:30', '08:21', '15:20', '10:11'][i % 8]}`,
    version: 1,
    mfa: i < 2,
    note: '',
  }))
  const company = {
    name: alternate ? '杭州咖啡运营有限公司' : '上海咖啡科技有限公司',
    shortName: 'CoffeeLink',
    email: 'service@example.com',
    phone: '021-8000-0000',
    contact: '张三',
    industry: '咖啡设备与运营服务',
    size: '50–200 人',
    timezone: 'Asia/Shanghai',
    description: '连接每一台咖啡机，让每一杯咖啡更智能。为设备、点位与团队提供统一的数字化运营管理。',
    address: alternate ? '浙江省杭州市' : '上海市浦东新区',
  }
  const actions = [
    ['角色权限', '分配成员角色', '王五', 'medium'],
    ['成员管理', '邀请成员', '林悦', 'low'],
    ['组织架构', '调整所属部门', '客户成功部', 'medium'],
    ['企业信息', '修改企业资料', company.name, 'low'],
    ['成员管理', '禁用成员', '赵六', 'high'],
    ['系统设置', '更新安全策略', '登录与认证', 'high'],
    ['操作日志', '导出操作记录', '近 7 天操作记录', 'medium'],
    ['角色权限', '更新数据范围', '运营管理员', 'high'],
  ]
  const logs = actions.map((a, i) => ({
    id: `audit-${i + 1}`,
    time: `2026-09-08 ${['10:32:15', '10:21:08', '10:10:42', '09:58:12', '09:42:06', '09:15:22', '08:55:10', '08:30:19'][i]}`,
    actor: ['张三', '李四'][i % 2]!,
    module: a[0]!,
    action: a[1]!,
    target: a[2]!,
    result: 'success' as const,
    risk: a[3] as 'low' | 'medium' | 'high',
    requestId: `demo-20260908-${String(i + 1).padStart(6, '0')}`,
    before: i === 0 ? '普通成员' : '变更前配置',
    after: i === 0 ? '运营管理员' : '变更后配置',
    reason: '业务调整（界面演示记录）',
  }))
  return {
    members,
    roles,
    departments,
    company,
    logs,
    settings: {
      platformName: 'CoffeeLink 咖啡机物联云平台',
      language: '简体中文',
      timezone: 'Asia/Shanghai',
      mfa: true,
      sessionMinutes: 30,
      loginLock: true,
      attempts: 5,
      notificationEmail: true,
      notificationInApp: true,
      alert: true,
      workorder: true,
      member: true,
      digest: false,
      apiEnabled: false,
      webhook: '',
    },
  }
}
