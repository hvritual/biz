import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { loadSnapshot, saveSnapshot } from '@/services/demo/repository'
import { applyStatusAction, memberActionError } from '@/services/memberPolicy'
import { departmentMoveAllowed } from '@/utils/organization'
import { timestamp } from '@/utils/format'
import type { Member, MemberAction, Role, AuditRecord, Company, Department } from '@/types/enterprise'
export const useEnterpriseStore = defineStore('enterprise', () => {
  const tenantId = ref('shanghai'),
    snapshot = ref(loadSnapshot('shanghai'))
  const members = computed(() => snapshot.value.members),
    roles = computed(() => snapshot.value.roles)
  const departments = computed(() => snapshot.value.departments),
    company = computed(() => snapshot.value.company)
  const logs = computed(() => snapshot.value.logs),
    settings = computed(() => snapshot.value.settings)
  const previewMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'demo'
  const departmentName = (id: string) => departments.value.find((d) => d.id === id)?.name ?? '未分配部门'
  const roleName = (id: string) => roles.value.find((r) => r.id === id)?.name ?? '未知角色'
  let lastPersisted = JSON.stringify(snapshot.value)
  function persist() {
    try {
      saveSnapshot(tenantId.value, snapshot.value)
      lastPersisted = JSON.stringify(snapshot.value)
    } catch {
      snapshot.value = JSON.parse(lastPersisted)
      throw new Error('本地存储不可用，本次变更未保存。请检查浏览器存储权限。')
    }
  }
  function switchTenant(id: string) {
    if (!['shanghai', 'hangzhou'].includes(id)) return
    tenantId.value = id
    snapshot.value = loadSnapshot(id)
    lastPersisted = JSON.stringify(snapshot.value)
  }
  function audit(
    module: string,
    action: string,
    target: string,
    before = '',
    after = '',
    reason = '',
    risk: AuditRecord['risk'] = 'low',
  ) {
    const id = crypto.randomUUID()
    snapshot.value.logs.unshift({
      id,
      time: timestamp(),
      actor: '张三',
      module,
      action,
      target,
      result: 'success',
      risk,
      requestId: `demo-${id}`,
      before,
      after,
      reason,
    })
    persist()
  }
  function saveMember(draft: Member, action: MemberAction, expectedVersion: number) {
    const current = members.value.find((m) => m.id === draft.id)
    if (current && current.version !== expectedVersion)
      throw new Error('成员资料已被其他操作修改，请重新打开后重试。')
    if (current) {
      const error = memberActionError(action, current, members.value, roles.value, draft.roleIds)
      if (error) throw new Error(error)
    }
    if (
      !draft.roleIds.length ||
      draft.roleIds.some((id) => !roles.value.some((r) => r.id === id && r.enabled))
    )
      throw new Error('请选择至少一个已启用的有效角色。')
    if (!departments.value.some((d) => d.id === draft.departmentId && d.enabled))
      throw new Error('请选择有效且启用的所属部门。')
    draft = { ...draft, email: draft.email.trim() }
    if (!draft.name.trim() || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(draft.email))
      throw new Error('请填写姓名和有效邮箱。')
    if (
      members.value.some(
        (m) =>
          m.id !== draft.id && m.email.toLowerCase() === draft.email.toLowerCase() && m.status !== 'removed',
      )
    )
      throw new Error('当前企业已存在该邮箱的成员。')
    if (!current && members.value.filter((m) => m.status !== 'removed').length >= 500)
      throw new Error('成员额度已用完，请先申请扩容。')
    const updated = {
      ...draft,
      name: draft.name.trim(),
      email: draft.email.trim(),
      version: expectedVersion + 1,
    }
    if (current) snapshot.value.members = snapshot.value.members.map((m) => (m.id === draft.id ? updated : m))
    else snapshot.value.members.unshift(updated)
    audit(
      '成员管理',
      action === 'role'
        ? '变更成员角色'
        : action === 'edit'
          ? '修改成员信息'
          : action === 'invite'
            ? '创建成员邀请'
            : '新增成员',
      draft.name,
      current
        ? JSON.stringify({
            name: current.name,
            department: departmentName(current.departmentId),
            roles: current.roleIds.map(roleName),
          })
        : '无',
      JSON.stringify({
        name: updated.name,
        department: departmentName(updated.departmentId),
        roles: updated.roleIds.map(roleName),
      }),
      '界面预览操作',
      action === 'role' ? 'high' : 'low',
    )
  }
  function changeStatus(
    id: string,
    action: 'activate' | 'suspend' | 'remove',
    version: number,
    reason: string,
  ) {
    const member = members.value.find((m) => m.id === id)
    if (!member) throw new Error('成员不存在。')
    if (member.version !== version) throw new Error('成员状态已变化，请刷新后重试。')
    const error = memberActionError(action, member, members.value, roles.value)
    if (error) throw new Error(error)
    if (!reason.trim()) throw new Error('请填写操作原因。')
    const updated = applyStatusAction(action, member)
    snapshot.value.members = snapshot.value.members.map((m) => (m.id === id ? updated : m))
    audit(
      '成员管理',
      action === 'activate' ? '启用成员' : action === 'suspend' ? '禁用成员' : '移除成员',
      member.name,
      member.status,
      updated.status,
      reason,
      'high',
    )
  }
  function saveRole(role: Role) {
    if (!role.name.trim()) throw new Error('请填写角色名称。')
    if (roles.value.some((r) => r.id !== role.id && r.name === role.name)) throw new Error('角色名称已存在。')
    const old = roles.value.find((r) => r.id === role.id)
    if (old?.builtin) throw new Error('内置角色不可直接修改，请复制为自定义角色。')
    const updated = { ...role, updatedAt: timestamp() }
    snapshot.value.roles = old
      ? roles.value.map((r) => (r.id === role.id ? updated : r))
      : [...roles.value, updated]
    audit(
      '角色权限',
      old ? '更新角色权限' : '新建角色',
      role.name,
      old?.permissions.join(', ') ?? '',
      role.permissions.join(', '),
      '界面预览操作',
      'high',
    )
  }
  function saveCompany(value: Company) {
    const before = JSON.stringify(company.value)
    snapshot.value.company = { ...value }
    audit('企业信息', '修改企业资料', value.name, before, JSON.stringify(value))
  }
  function saveDepartment(value: Department) {
    if (!departmentMoveAllowed(departments.value, value.id, value.parentId))
      throw new Error('部门不能移动到自身或下级部门。')
    if (!value.enabled && members.value.some((m) => m.departmentId === value.id && m.status !== 'removed'))
      throw new Error('请先转移部门成员，再停用部门。')
    if (!value.name.trim()) throw new Error('请填写部门名称。')
    const exists = departments.value.some((d) => d.id === value.id)
    snapshot.value.departments = exists
      ? departments.value.map((d) => (d.id === value.id ? { ...value } : d))
      : [...departments.value, { ...value }]
    audit('组织架构', exists ? '编辑部门' : '新建部门', value.name)
  }
  function saveSettings(values: Record<string, string | number | boolean>) {
    const before = JSON.stringify(settings.value)
    snapshot.value.settings = { ...settings.value, ...values }
    audit('系统设置', '更新系统设置', '当前企业', before, JSON.stringify(values), '界面预览操作', 'medium')
  }
  return {
    tenantId,
    members,
    roles,
    departments,
    company,
    logs,
    settings,
    previewMode,
    departmentName,
    roleName,
    switchTenant,
    audit,
    saveMember,
    changeStatus,
    saveRole,
    saveCompany,
    saveDepartment,
    saveSettings,
  }
})
