import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  createEnterpriseDataSource,
  emptyEnterpriseSnapshot,
  projectMember,
  projectRole,
  type EnterpriseDomain,
  type EnterpriseSourceState,
} from '@/services/enterprise/dataSource'
import { applyStatusAction, memberActionError, prepareMemberStatusBatch } from '@/services/memberPolicy'
import { departmentMoveAllowed } from '@/utils/organization'
import { timestamp } from '@/utils/format'
import type { Member, MemberAction, Role, AuditRecord, Company, Department, DataScope } from '@/types/enterprise'
import {
  activateEnterpriseMember,
  createEnterpriseMember,
  getEnterpriseMember,
  listRemovedEnterpriseMembers,
  memberRequestId,
  queryEnterpriseMembers,
  memberRuntimeError,
  readEnterpriseMemberSession,
  removeEnterpriseMember,
  restoreEnterpriseMember,
  sameTrustedSession,
  suspendEnterpriseMember,
  updateEnterpriseMember,
  type EnterpriseMemberListQuery,
  type EnterpriseTenantMember,
} from '@/services/enterprise/memberRuntime'
import {
  createEnterpriseRole,
  deleteEnterpriseRole,
  disableEnterpriseRole,
  enableEnterpriseRole,
  getEnterpriseRole,
  listEnterpriseRoles,
  readEnterpriseRoleSession,
  roleRequestId,
  roleRuntimeError,
  setEnterpriseRolePermissions,
  updateEnterpriseRole,
  type EnterpriseTenantRole,
} from '@/services/enterprise/roleRuntime'
import {
  createEnterpriseDepartment,
  disableEnterpriseDepartment,
  enableEnterpriseDepartment,
  getEnterpriseDepartment,
  readEnterpriseDepartmentSession,
  departmentRequestId,
  departmentRuntimeError,
  updateEnterpriseDepartment,
  type EnterpriseDepartmentDraft,
} from '@/services/enterprise/departmentRuntime'
import {
  getEnterpriseTenantProfile,
  readEnterpriseTenantProfileSession,
  tenantProfileRequestId,
  tenantProfileRuntimeError,
  updateEnterpriseTenantProfile,
} from '@/services/enterprise/tenantProfileRuntime'
import {
  getEnterpriseTenantBranding,
  readEnterpriseTenantBrandingSession,
  tenantBrandingRequestId,
  tenantBrandingRuntimeError,
  updateEnterpriseTenantBranding,
  type EnterpriseTenantBranding,
  type EnterpriseTenantBrandingDraft,
} from '@/services/enterprise/tenantBrandingRuntime'
import { loginUrl, logoutSession, type PermissionGrant, type TrustedSession } from '@/services/runtime/api'
import { cancelTrustedSessionRequests } from '@/services/commercial/platformCommercial'
import { publishSessionContextChange, subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'
import {
  authorizationApiMode,
  currentAuthorizationAllows,
  currentAuthorizationState,
  ensureCurrentAuthorization,
  invalidateCurrentAuthorization,
} from '@/services/runtime/authorization'

function serverMemberStatus(status: Member['status']) {
  switch (status) {
    case 'active': return 'TENANT_MEMBER_STATUS_ACTIVE'
    case 'suspended': return 'TENANT_MEMBER_STATUS_SUSPENDED'
    case 'removed': return 'TENANT_MEMBER_STATUS_REMOVED'
    default: return 'TENANT_MEMBER_STATUS_INVITED'
  }
}

function grantScope(scope: DataScope) {
  if (scope === 'all') return 'all'
  if (scope === 'self') return 'self'
  return 'sites'
}

export const useEnterpriseStore = defineStore('enterprise', () => {
  const dataSource = createEnterpriseDataSource()
  const initial = dataSource.initial()
  const tenantId = ref(initial.tenantId)
  const snapshot = ref(initial.snapshot)
  const session = ref<TrustedSession | null>(initial.session)
  const loading = ref(false)
  const ready = ref(dataSource.kind === 'demo')
  const sourceError = ref('')

  const members = computed(() => snapshot.value.members)
  const roles = computed(() => snapshot.value.roles)
  const departments = computed(() => snapshot.value.departments)
  const company = computed(() => snapshot.value.company)
  const logs = computed(() => snapshot.value.logs)
  const settings = computed(() => snapshot.value.settings)
  const previewMode = dataSource.kind === 'demo'
  const memberTotal = ref(previewMode ? members.value.filter((member) => member.status !== 'removed').length : 0)
  const removedMembers = ref<Member[]>([])
  const removedMemberTotal = ref(0)
  const sourceKind = dataSource.kind
  const demoBrandingByTenant = new Map<string, EnterpriseTenantBranding>([
    ['shanghai', { tenantId: 'shanghai', preset: 'blue', primary: '', version: 1, canManage: true }],
    ['hangzhou', { tenantId: 'hangzhou', preset: 'emerald', primary: '', version: 1, canManage: true }],
  ])
  const branding = ref<EnterpriseTenantBranding | null>(previewMode ? { ...demoBrandingByTenant.get(initial.tenantId)! } : null)
  const brandingLoading = ref(false)
  const brandingReady = ref(previewMode)
  const brandingError = ref('')
  const canManageBranding = computed(() => Boolean(branding.value?.canManage))
  const authenticated = computed(() => previewMode || Boolean(session.value?.authenticated))
  const tenantOptions = computed(() =>
    previewMode
      ? [
          { id: 'shanghai', name: '上海咖啡科技有限公司' },
          { id: 'hangzhou', name: '杭州咖啡运营有限公司' },
        ]
      : (session.value?.tenants ?? []),
  )

  const departmentName = (id: string) => departments.value.find((d) => d.id === id)?.name ?? '未分配部门'
  const roleName = (id: string) => roles.value.find((r) => r.id === id)?.name ?? '未知角色'

  let lastPersisted = JSON.stringify(snapshot.value)
  let activeDomains: EnterpriseDomain[] = []
  let companyMutation: { tenantId: string; signature: string; key: string } | null = null
  let memberMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null
  let memberListQuery: EnterpriseMemberListQuery | null = null
  let memberListGeneration = 0
  let roleMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null
  let departmentMutation: { tenantId: string; signature: string; keys: Record<string, string> } | null = null
  let brandingMutation: { tenantId: string; signature: string; key: string } | null = null
  let sessionEpoch = 0
  let refreshGeneration = 0

  function clearRuntimeTenantState(clearSession = false) {
    cancelTrustedSessionRequests()
    refreshGeneration++
    snapshot.value = emptyEnterpriseSnapshot()
    if (clearSession) {
      session.value = null
      tenantId.value = ''
    }
    companyMutation = null
    memberMutation = null
    memberListQuery = null
    memberListGeneration++
    memberTotal.value = 0
    removedMembers.value = []
    removedMemberTotal.value = 0
    roleMutation = null
    departmentMutation = null
    branding.value = null
    brandingError.value = ''
    brandingReady.value = false
    brandingMutation = null
  }

  async function refreshBranding(expectedEpoch = sessionEpoch) {
    const targetTenant = tenantId.value
    brandingLoading.value = true
    brandingError.value = ''
    try {
      if (previewMode) {
        const current = demoBrandingByTenant.get(targetTenant) ?? { tenantId: targetTenant, preset: 'blue', primary: '', version: 1, canManage: true }
        demoBrandingByTenant.set(targetTenant, current)
        branding.value = { ...current }
        brandingReady.value = true
        return true
      }
      const trusted = session.value
      if (!trusted?.authenticated || !trusted.active_tenant_id || trusted.active_tenant_id !== targetTenant) {
        branding.value = null
        brandingReady.value = true
        return false
      }
      if (
        authorizationApiMode() &&
        (
          currentAuthorizationState.status !== 'ready' ||
          currentAuthorizationState.snapshot?.tenant_id !== targetTenant ||
          !currentAuthorizationAllows('tenant.branding.get')
        )
      ) {
        branding.value = null
        brandingReady.value = false
        return false
      }
      const current = await getEnterpriseTenantBranding(trusted)
      if (expectedEpoch !== sessionEpoch || tenantId.value !== targetTenant) return false
      branding.value = current
      brandingReady.value = true
      return true
    } catch (error) {
      if (expectedEpoch === sessionEpoch && tenantId.value === targetTenant) {
        branding.value = null
        brandingError.value = tenantBrandingRuntimeError(error)
        brandingReady.value = true
      }
      return false
    } finally {
      if (expectedEpoch === sessionEpoch && tenantId.value === targetTenant) brandingLoading.value = false
    }
  }

  async function stableBrandingSession() {
    const expected = session.value
    if (!expected?.authenticated || !expected.active_tenant_id) throw new Error('请先登录并选择可访问租户。')
    const current = await readEnterpriseTenantBrandingSession()
    if (!sameTrustedSession(expected, current)) {
      branding.value = null
      brandingMutation = null
      throw new Error('会话或当前租户已变化，请刷新后重新操作。')
    }
    return current
  }

  async function saveBranding(draft: EnterpriseTenantBrandingDraft) {
    const normalized: EnterpriseTenantBrandingDraft = {
      preset: draft.preset,
      primary: draft.preset === 'custom' ? draft.primary.trim().toLowerCase() : '',
    }
    if (previewMode) {
      const current = branding.value ?? { tenantId: tenantId.value, preset: 'blue', primary: '', version: 0, canManage: true }
      const next: EnterpriseTenantBranding = { ...current, ...normalized, tenantId: tenantId.value, version: Number(current.version) + 1, canManage: true }
      demoBrandingByTenant.set(tenantId.value, next)
      branding.value = { ...next }
      brandingReady.value = true
      return next
    }
    const current = branding.value
    if (!current) throw new Error('企业品牌主题尚未从服务端加载。')
    if (!current.canManage) throw new Error('当前账号没有维护企业品牌主题的权限。')
    const signature = JSON.stringify({ tenantId: tenantId.value, version: current.version, ...normalized })
    if (!brandingMutation || brandingMutation.tenantId !== tenantId.value || brandingMutation.signature !== signature) {
      brandingMutation = { tenantId: tenantId.value, signature, key: tenantBrandingRequestId(tenantId.value) }
    }
    try {
      const trusted = await stableBrandingSession()
      const updated = await updateEnterpriseTenantBranding(trusted, current, normalized, brandingMutation.key)
      if (updated.tenantId && updated.tenantId !== tenantId.value) throw new Error('企业品牌主题回读不属于当前租户。')
      branding.value = updated
      brandingError.value = ''
      brandingReady.value = true
      brandingMutation = null
      return updated
    } catch (error) {
      throw new Error(tenantBrandingRuntimeError(error))
    }
  }

  function resetBranding() {
    return saveBranding({ preset: 'blue', primary: '' })
  }

  function memberMutationSignature(draft: Member, action: MemberAction, expectedVersion: number) {
    return JSON.stringify({
      action,
      expectedVersion,
      id: draft.id,
      username: (draft.username ?? '').trim().toLowerCase(),
      activationMode: draft.activationMode ?? '',
      email: draft.email.trim().toLowerCase(),
      name: draft.name.trim(),
      phone: draft.phone.trim(),
      employeeId: draft.employeeId.trim(),
      departmentId: draft.departmentId,
      position: draft.position.trim(),
      roleIds: [...draft.roleIds].sort(),
      scope: draft.scope,
    })
  }

  function memberMutationKey(signature: string, slot: string, create: () => string) {
    if (!memberMutation || memberMutation.tenantId !== tenantId.value || memberMutation.signature !== signature) {
      memberMutation = { tenantId: tenantId.value, signature, keys: {} }
    }
    memberMutation.keys[slot] ??= create()
    return memberMutation.keys[slot]!
  }

  function roleMutationSignature(role: Role, current?: Role) {
    return JSON.stringify({
      id: role.id,
      name: role.name.trim(),
      description: role.description.trim(),
      enabled: role.enabled,
      scope: role.scope,
      permissions: [...role.permissions].sort(),
      version: current?.runtimeVersion ?? 0,
    })
  }

  function roleMutationKey(signature: string, slot: string, create: () => string) {
    if (!roleMutation || roleMutation.tenantId !== tenantId.value || roleMutation.signature !== signature) {
      roleMutation = { tenantId: tenantId.value, signature, keys: {} }
    }
    roleMutation.keys[slot] ??= create()
    return roleMutation.keys[slot]!
  }

  function departmentMutationSignature(value: Department, current?: Department) {
    return JSON.stringify({
      id: value.id,
      name: value.name.trim(),
      parentId: value.parentId ?? '',
      leaderId: value.leaderId,
      email: value.email ?? '',
      phone: value.phone ?? '',
      sort: value.sort ?? 0,
      enabled: value.enabled,
      version: current?.runtimeVersion ?? 0,
    })
  }

  function departmentMutationKey(signature: string, slot: string, create: () => string) {
    if (!departmentMutation || departmentMutation.tenantId !== tenantId.value || departmentMutation.signature !== signature) {
      departmentMutation = { tenantId: tenantId.value, signature, keys: {} }
    }
    departmentMutation.keys[slot] ??= create()
    return departmentMutation.keys[slot]!
  }

  function applySourceState(state: EnterpriseSourceState, domains = state.loadedDomains, replace = false) {
    tenantId.value = state.tenantId
    session.value = state.session
    if (previewMode || replace) {
      snapshot.value = state.snapshot
    } else {
      const current = snapshot.value
      snapshot.value = {
        ...current,
        members: domains.includes('members') ? state.snapshot.members : current.members,
        roles: domains.includes('roles') ? state.snapshot.roles : current.roles,
        departments: domains.includes('departments') ? state.snapshot.departments : current.departments,
        company: domains.includes('company') ? state.snapshot.company : current.company,
      }
    }
    if (previewMode || domains.includes('members')) {
      memberTotal.value = snapshot.value.members.filter((member) => member.status !== 'removed').length
    }
    lastPersisted = JSON.stringify(snapshot.value)
  }

  function errorMessage(error: unknown, domains: EnterpriseDomain[] = activeDomains) {
    const primary = domains[0]
    if (primary === 'members') return memberRuntimeError(error)
    if (primary === 'roles') return roleRuntimeError(error)
    if (primary === 'departments') return departmentRuntimeError(error)
    if (primary === 'company') return tenantProfileRuntimeError(error)
    return error instanceof Error ? error.message : '企业数据服务请求失败。'
  }

  async function refresh(domains: EnterpriseDomain[] = activeDomains) {
    activeDomains = [...new Set(domains)]
    const epoch = sessionEpoch
    const generation = ++refreshGeneration
    loading.value = true
    sourceError.value = ''
    try {
      const previousTenant = tenantId.value
      const state = await dataSource.load(tenantId.value || undefined, activeDomains)
      if (epoch !== sessionEpoch || generation !== refreshGeneration) return false
      applySourceState(state, activeDomains)
      if (state.tenantId !== previousTenant) {
        branding.value = null
        brandingReady.value = false
        brandingMutation = null
        await refreshBranding(epoch)
      }
      ready.value = true
      return true
    } catch (error) {
      if (epoch !== sessionEpoch || generation !== refreshGeneration) return false
      sourceError.value = errorMessage(error, activeDomains)
      ready.value = true
      throw error
    } finally {
      if (epoch === sessionEpoch && generation === refreshGeneration) loading.value = false
    }
  }

  async function ensureDomains(domains: EnterpriseDomain[]) {
    activeDomains = [...new Set(domains)]
    await refresh(activeDomains)
  }

  async function switchTenant(id: string) {
    if (!id || id === tenantId.value) return
    if (previewMode && !tenantOptions.value.some((tenant) => tenant.id === id)) return
    if (previewMode) {
      const state = await dataSource.switchTenant(id, activeDomains)
      applySourceState(state, activeDomains, true)
      clearRuntimeTenantState(false)
      applySourceState(state, activeDomains, true)
      await refreshBranding(sessionEpoch)
      ready.value = true
      return
    }

    const epoch = ++sessionEpoch
    const previousDomains = [...activeDomains]
    clearRuntimeTenantState(true)
    invalidateCurrentAuthorization()
    loading.value = true
    sourceError.value = ''
    try {
      const state = await dataSource.switchTenant(id, previousDomains)
      if (epoch !== sessionEpoch) return
      applySourceState(state, previousDomains, true)
      await ensureCurrentAuthorization(true)
      if (epoch !== sessionEpoch) return
      await refreshBranding(epoch)
      ready.value = true
    } catch (error) {
      if (epoch === sessionEpoch) {
        sourceError.value = errorMessage(error)
        try {
          const state = await dataSource.load(undefined, previousDomains)
          if (epoch === sessionEpoch) {
            applySourceState(state, previousDomains, true)
            await ensureCurrentAuthorization(true)
            if (epoch !== sessionEpoch) return
            await refreshBranding(epoch)
            ready.value = true
          }
        } catch {
          session.value = null
          tenantId.value = ''
          ready.value = true
        }
      }
      throw error
    } finally {
      if (epoch === sessionEpoch) loading.value = false
    }
  }

  async function synchronizeExternalSession() {
    if (previewMode) return
    const epoch = ++sessionEpoch
    const domains = [...activeDomains]
    clearRuntimeTenantState(true)
    loading.value = true
    sourceError.value = ''
    try {
      const state = await dataSource.load(undefined, domains)
      if (epoch !== sessionEpoch) return
      applySourceState(state, domains, true)
      await refreshBranding(epoch)
      ready.value = true
    } catch (error) {
      if (epoch !== sessionEpoch) return
      sourceError.value = errorMessage(error, domains)
      ready.value = true
    } finally {
      if (epoch === sessionEpoch) loading.value = false
    }
  }

  async function logout() {
    if (previewMode) return
    const epoch = ++sessionEpoch
    clearRuntimeTenantState(true)
    loading.value = true
    try {
      await logoutSession()
    } finally {
      if (epoch === sessionEpoch) {
        session.value = { authenticated: false, context_version: 0 }
        tenantId.value = ''
        ready.value = true
        loading.value = false
        publishSessionContextChange(0)
      }
    }
  }

  function persist() {
    if (!previewMode) return
    try {
      dataSource.persist(tenantId.value, snapshot.value)
      lastPersisted = JSON.stringify(snapshot.value)
    } catch {
      snapshot.value = JSON.parse(lastPersisted)
      throw new Error('本地存储不可用，本次变更未保存。请检查浏览器存储权限。')
    }
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
    if (!previewMode) return
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

  async function stableMemberSession() {
    const expected = session.value
    if (!expected?.authenticated || !expected.active_tenant_id) throw new Error('请先登录并选择可访问租户。')
    const current = await readEnterpriseMemberSession()
    if (!sameTrustedSession(expected, current)) {
      await refresh()
      throw new Error('会话或当前租户已变化，请刷新后重新操作。')
    }
    return current
  }


  async function applyMemberPage(trusted: TrustedSession, query: EnterpriseMemberListQuery) {
    const targetTenant = tenantId.value
    const generation = ++memberListGeneration
    const result = await queryEnterpriseMembers(trusted, query)
    if (generation !== memberListGeneration || tenantId.value !== targetTenant) return false
    snapshot.value = {
      ...snapshot.value,
      members: result.members.map(projectMember),
    }
    memberTotal.value = result.total
    memberListQuery = { ...query }
    return true
  }

  async function queryMembers(input: {
    query: string
    roleId: string
    departmentId: string
    status: Member['status'] | ''
    page: number
    pageSize: number
  }) {
    if (previewMode) {
      memberTotal.value = members.value.filter((member) => member.status !== 'removed').length
      return true
    }
    const query: EnterpriseMemberListQuery = {
      query: input.query.trim(),
      roleId: input.roleId,
      departmentId: input.departmentId,
      status: input.status ? serverMemberStatus(input.status) : '',
      page: Math.max(1, Math.trunc(input.page)),
      pageSize: Math.max(1, Math.min(100, Math.trunc(input.pageSize))),
    }
    loading.value = true
    sourceError.value = ''
    try {
      const trusted = await stableMemberSession()
      return await applyMemberPage(trusted, query)
    } catch (error) {
      sourceError.value = memberRuntimeError(error)
      throw new Error(sourceError.value)
    } finally {
      loading.value = false
    }
  }

  async function queryRemovedMembers(page = 1, pageSize = 50) {
    if (previewMode) {
      removedMembers.value = members.value.filter((member) => member.status === 'removed')
      removedMemberTotal.value = removedMembers.value.length
      return true
    }
    const trusted = await stableMemberSession()
    const result = await listRemovedEnterpriseMembers(trusted, page, pageSize)
    removedMembers.value = result.members.map(projectMember)
    removedMemberTotal.value = result.total
    return true
  }

  async function restoreMember(id: string, version: number, reason: string) {
    if (!reason.trim()) throw new Error('请填写恢复原因。')
    const member = removedMembers.value.find((value) => value.id === id)
    if (!member || member.status !== 'removed') throw new Error('该成员不在回收站中。')
    if (member.version !== version) throw new Error('成员状态已变化，请刷新回收站后重试。')
    if (previewMode) {
      const restored = { ...member, status: 'active' as const, version: version + 1, online: false }
      snapshot.value.members = snapshot.value.members.map((value) => value.id === id ? restored : value)
      removedMembers.value = removedMembers.value.filter((value) => value.id !== id)
      removedMemberTotal.value = removedMembers.value.length
      audit('成员管理', '恢复成员', member.name, 'removed', 'active', reason, 'high')
      return
    }
    try {
      const trusted = await stableMemberSession()
      const receipt = await restoreEnterpriseMember(
        trusted,
        asServerMember(member),
        memberRequestId('restore'),
        reason,
      )
      await getEnterpriseMember(trusted, receipt.userId)
      await Promise.all([refreshMemberQuery(), queryRemovedMembers()])
      return receipt
    } catch (error) {
      throw new Error(memberRuntimeError(error))
    }
  }

  async function refreshMemberQuery() {
    if (previewMode || !memberListQuery) {
      await refresh()
      return
    }
    const trusted = await stableMemberSession()
    try {
      await applyMemberPage(trusted, memberListQuery)
    } catch (error) {
      throw new Error(memberRuntimeError(error))
    }
  }

  function asServerMember(member: Member): EnterpriseTenantMember {
    return {
      userId: member.id,
      username: member.username ?? '',
      email: member.email,
      status: serverMemberStatus(member.status),
      version: member.runtimeVersion ?? member.version,
      name: member.name,
      phone: member.phone,
      employeeId: member.employeeId,
      position: member.position,
      departmentId: member.departmentId,
      roles: member.roleIds.map((roleId) => ({
        roleId,
        roleName: roleName(roleId),
        roleStatus: roles.value.find((role) => role.id === roleId)?.enabled
          ? 'TENANT_ROLE_STATUS_ACTIVE'
          : 'TENANT_ROLE_STATUS_DISABLED',
      })),
      derivedDataScope: member.scope === 'all' ? 'all' : member.scope === 'self' ? 'self' : 'sites',
    }
  }

  function validateMemberDraft(draft: Member, current?: Member) {
    if (
      !draft.roleIds.length ||
      draft.roleIds.some((id) => !roles.value.some((role) => role.id === id && role.enabled))
    ) throw new Error('请选择至少一个已启用的有效角色。')
    if (departments.value.length && !departments.value.some((department) => department.id === draft.departmentId && department.enabled)) {
      throw new Error('请选择有效且启用的所属部门。')
    }
    if (!draft.name.trim()) throw new Error('请填写成员姓名。')
    const email = draft.email.trim()
    const phone = draft.phone.trim()
    if (!email && !phone) throw new Error('手机号和邮箱至少填写一项。')
    if (email && !email.includes('*') && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      throw new Error('请填写有效邮箱。')
    }
    if (previewMode) {
      const normalizedEmail = email.toLowerCase()
      if (
        normalizedEmail &&
        members.value.some(
          (member) =>
            member.id !== draft.id &&
            member.status !== 'removed' &&
            member.email.trim().toLowerCase() === normalizedEmail,
        )
      ) throw new Error('当前企业已存在该邮箱的成员。')
      if (
        phone &&
        members.value.some(
          (member) =>
            member.id !== draft.id &&
            member.status !== 'removed' &&
            member.phone.trim() === phone,
        )
      ) throw new Error('当前企业已存在该手机号的成员。')
    }
    if (!current) {
      const username = (draft.username ?? '').trim().toLowerCase()
      if (username.length < 5 || username.length > 20 || /^\d+$/.test(username) || /\s/.test(username)) {
        throw new Error('账号需为 5～20 位且不能为纯数字。')
      }
      if (!draft.activationMode) throw new Error('请选择成员激活方式。')
      if (draft.activationMode === 'sms_initial_password' && !phone) {
        throw new Error('短信发送账号密码需要填写手机号。')
      }
    }
    if (!current && previewMode && members.value.filter((member) => member.status !== 'removed').length >= 500) {
      throw new Error('成员额度已用完，请先申请扩容。')
    }
  }

  async function saveMember(draft: Member, action: MemberAction, expectedVersion: number) {
    const current = members.value.find((member) => member.id === draft.id)
    if (current && current.version !== expectedVersion) throw new Error('成员资料已被其他操作修改，请重新打开后重试。')
    if (current) {
      const error = memberActionError(action, current, members.value, roles.value, draft.roleIds)
      if (error) throw new Error(error)
      if ((draft.username ?? '').trim().toLowerCase() !== (current.username ?? '').trim().toLowerCase()) {
        throw new Error('账号为全局唯一登录标识，创建后不可修改。')
      }
    }
    validateMemberDraft(draft, current)

    if (previewMode) {
      const updated = {
        ...draft,
        username: (draft.username ?? '').trim().toLowerCase(),
        name: draft.name.trim(),
        email: draft.email.trim(),
        status: current?.status ?? 'invited' as const,
        version: expectedVersion + 1,
      }
      if (current) snapshot.value.members = snapshot.value.members.map((member) => member.id === draft.id ? updated : member)
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
          ? JSON.stringify({ name: current.name, department: departmentName(current.departmentId), roles: current.roleIds.map(roleName) })
          : '无',
        JSON.stringify({ name: updated.name, department: departmentName(updated.departmentId), roles: updated.roleIds.map(roleName) }),
        '界面预览操作',
        action === 'role' ? 'high' : 'low',
      )
      return
    }

    const signature = memberMutationSignature(draft, action, expectedVersion)
    const key = (slot: string, create: () => string) => memberMutationKey(signature, slot, create)

    try {
      const trusted = await stableMemberSession()
      if (action === 'create' || action === 'invite') {
        const receipt = await createEnterpriseMember(
          trusted,
          {
            username: (draft.username ?? '').trim(),
            email: draft.email,
            phone: draft.phone,
            name: draft.name,
            employeeId: draft.employeeId,
            position: draft.position,
            departmentId: draft.departmentId,
            roleIds: draft.roleIds,
            activationMode: draft.activationMode === 'sms_initial_password' ? 'sms_initial_password' : 'activation_link',
          },
          key('create', () => memberRequestId('create')),
        )
        await getEnterpriseMember(trusted, receipt.member.userId)
        await refreshMemberQuery()
        memberMutation = null
        return receipt
      }

      if (action === 'edit' || action === 'role') {
        if (!current) throw new Error('成员不存在。')
        if (action === 'role' && draft.scope !== current.scope) {
          throw new Error('真实服务的数据范围由角色权限派生，当前不支持按成员单独覆盖数据范围。')
        }
        const receipt = await updateEnterpriseMember(
          trusted,
          asServerMember(current),
          {
            email: draft.email,
            phone: draft.phone,
            name: draft.name,
            employeeId: draft.employeeId,
            position: draft.position,
            departmentId: draft.departmentId,
            roleIds: draft.roleIds,
          },
          key('update', () => memberRequestId('update')),
        )
        await getEnterpriseMember(trusted, receipt.userId)
        await refreshMemberQuery()
        memberMutation = null
        return receipt
      }

      throw new Error('该成员操作不应通过资料保存入口执行。')
    } catch (error) {
      throw new Error(memberRuntimeError(error))
    }
  }

  async function changeStatus(
    id: string,
    action: 'activate' | 'suspend' | 'remove',
    version: number,
    reason: string,
  ) {
    const member = members.value.find((value) => value.id === id)
    if (!member) throw new Error('成员不存在。')
    if (member.version !== version) throw new Error('成员状态已变化，请刷新后重试。')
    const policy = memberActionError(action, member, members.value, roles.value)
    if (policy) throw new Error(policy)
    if (!reason.trim()) throw new Error('请填写操作原因。')

    if (previewMode) {
      const updated = applyStatusAction(action, member)
      snapshot.value.members = snapshot.value.members.map((value) => value.id === id ? updated : value)
      audit(
        '成员管理',
        action === 'activate' ? '启用成员' : action === 'suspend' ? '禁用成员' : '移除成员',
        member.name,
        member.status,
        updated.status,
        reason,
        'high',
      )
      return
    }

    const trusted = await stableMemberSession()
    const server = asServerMember(member)
    const key = memberRequestId(action)
    const receipt = action === 'activate'
      ? await activateEnterpriseMember(trusted, server, key, reason)
      : action === 'suspend'
        ? await suspendEnterpriseMember(trusted, server, key, reason)
        : await removeEnterpriseMember(trusted, server, key, reason)
    await getEnterpriseMember(trusted, receipt.userId)
    await refreshMemberQuery()
  }

  async function changeStatuses(
    targets: { id: string; version: number }[],
    action: 'activate' | 'suspend',
    reason: string,
  ) {
    if (!reason.trim()) throw new Error('请填写操作原因。')
    if (previewMode) {
      const updated = prepareMemberStatusBatch(members.value, roles.value, targets, action)
      const records = targets.map(({ id }) => {
        const before = members.value.find((member) => member.id === id)!
        const after = updated.find((member) => member.id === id)!
        const logId = crypto.randomUUID()
        return {
          id: logId,
          time: timestamp(),
          actor: '张三',
          module: '成员管理',
          action: action === 'activate' ? '批量启用成员' : '批量禁用成员',
          target: before.name,
          result: 'success' as const,
          risk: 'high' as const,
          requestId: `demo-${logId}`,
          before: before.status,
          after: after.status,
          reason,
        }
      })
      snapshot.value.members = updated
      snapshot.value.logs.unshift(...records)
      persist()
      return
    }

    prepareMemberStatusBatch(members.value, roles.value, targets, action)
    const trusted = await stableMemberSession()
    let failure: unknown = null
    try {
      for (const target of targets) {
        const member = members.value.find((value) => value.id === target.id)
        if (!member) throw new Error('批量操作中存在已不存在的成员，请刷新后重试。')
        const server = asServerMember(member)
        if (action === 'activate') await activateEnterpriseMember(trusted, server, memberRequestId('activate'), reason)
        else await suspendEnterpriseMember(trusted, server, memberRequestId('suspend'), reason)
      }
    } catch (error) {
      failure = error
    }
    await refreshMemberQuery()
    if (failure) throw failure
  }

  function asServerRole(role: Role): EnterpriseTenantRole {
    return {
      id: role.id,
      name: role.name,
      status: role.enabled ? 'TENANT_ROLE_STATUS_ACTIVE' : 'TENANT_ROLE_STATUS_DISABLED',
      version: role.runtimeVersion ?? 0,
      permissions: role.permissions.map((permission) => ({ permission, scope: grantScope(role.scope) })),
      description: role.description,
      roleCode: role.roleCode ?? '',
      systemRole: role.builtin,
      memberCount: role.memberCount ?? 0,
      protectedOwner: role.roleCode === 'tenant_owner',
      protectedSystem: role.builtin,
    }
  }

  async function stableRoleSession() {
    const expected = session.value
    if (!expected?.authenticated || !expected.active_tenant_id) throw new Error('请先登录并选择可访问租户。')
    const current = await readEnterpriseRoleSession()
    if (!sameTrustedSession(expected, current)) {
      await refresh()
      throw new Error('会话或当前租户已变化，请刷新后重新操作。')
    }
    return current
  }

  async function saveRole(role: Role) {
    if (!role.name.trim()) throw new Error('请填写角色名称。')
    if (role.description.trim().length > 120) throw new Error('角色说明不能超过 120 个字符。')
    if (roles.value.some((value) => value.id !== role.id && value.name === role.name)) throw new Error('角色名称已存在。')
    const old = roles.value.find((value) => value.id === role.id)
    if (old?.builtin) throw new Error('内置角色不可直接修改，请复制为自定义角色。')

    if (previewMode) {
      const updated = { ...role, updatedAt: timestamp() }
      snapshot.value.roles = old
        ? roles.value.map((value) => value.id === role.id ? updated : value)
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
      return
    }

    const signature = roleMutationSignature(role, old)
    const key = (slot: string, create: () => string) => roleMutationKey(signature, slot, create)
    try {
      const trusted = await stableRoleSession()
      let serverRole: EnterpriseTenantRole
      if (!old) {
        serverRole = await createEnterpriseRole(trusted, role.name, role.description, key('create', () => roleRequestId('create'))) as EnterpriseTenantRole
      } else {
        serverRole = asServerRole(old)
        if (old.name !== role.name) {
          serverRole = await updateEnterpriseRole(trusted, serverRole, role.name, role.description, key('update', () => roleRequestId('update')))
        }
      }
      const grants: PermissionGrant[] = role.permissions.map((permission) => ({
        permission,
        scope: grantScope(role.scope),
      }))
      serverRole = await setEnterpriseRolePermissions(
        trusted,
        serverRole,
        grants,
        key('permissions', () => roleRequestId('permissions')),
      )
      const active = serverRole.status === 'TENANT_ROLE_STATUS_ACTIVE'
      if (role.enabled !== active) {
        serverRole = role.enabled
          ? await enableEnterpriseRole(trusted, serverRole, key('enable', () => roleRequestId('enable')))
          : await disableEnterpriseRole(trusted, serverRole, key('disable', () => roleRequestId('disable')))
      }
      await getEnterpriseRole(trusted, serverRole.id)
      await refresh(['roles', 'members'])
      roleMutation = null
    } catch (error) {
      throw new Error(roleRuntimeError(error))
    }
  }

  async function queryRoles(input: { query: string; status: '' | 'active' | 'disabled' }) {
    if (previewMode) return true
    loading.value = true
    sourceError.value = ''
    try {
      const trusted = await stableRoleSession()
      const status = input.status === 'active'
        ? 'TENANT_ROLE_STATUS_ACTIVE'
        : input.status === 'disabled'
          ? 'TENANT_ROLE_STATUS_DISABLED'
          : ''
      const result = await listEnterpriseRoles(trusted, { query: input.query.trim(), status })
      snapshot.value = { ...snapshot.value, roles: result.map(projectRole) }
      return true
    } catch (error) {
      sourceError.value = roleRuntimeError(error)
      throw new Error(sourceError.value)
    } finally {
      loading.value = false
    }
  }

  async function deleteRole(role: Role) {
    if (role.builtin) throw new Error('系统内置角色不可删除。')
    if ((role.memberCount ?? members.value.filter((member) => member.roleIds.includes(role.id) && member.status !== 'removed').length) > 0) {
      throw new Error('该角色仍有关联成员，请先移除成员绑定。')
    }
    if (previewMode) {
      snapshot.value.roles = roles.value.filter((value) => value.id !== role.id)
      audit('角色权限', '删除角色', role.name, JSON.stringify(role), '已删除', '界面预览操作', 'high')
      return
    }
    const signature = JSON.stringify({ action: 'delete', id: role.id, version: role.runtimeVersion ?? 0 })
    const key = roleMutationKey(signature, 'delete', () => roleRequestId('delete'))
    try {
      const trusted = await stableRoleSession()
      await deleteEnterpriseRole(trusted, asServerRole(role), key)
      await refresh(['roles', 'members'])
      roleMutation = null
    } catch (error) {
      throw new Error(roleRuntimeError(error))
    }
  }

  async function stableDepartmentSession() {
    const expected = session.value
    if (!expected?.authenticated || !expected.active_tenant_id) throw new Error('请先登录并选择可访问租户。')
    const current = await readEnterpriseDepartmentSession()
    if (!sameTrustedSession(expected, current)) {
      await refresh()
      throw new Error('会话或当前租户已变化，请刷新后重新操作。')
    }
    return current
  }

  async function saveDepartment(value: Department) {
    if (!departmentMoveAllowed(departments.value, value.id, value.parentId)) throw new Error('部门不能移动到自身或下级部门。')
    if (!value.enabled && members.value.some((member) => member.departmentId === value.id && member.status !== 'removed')) {
      throw new Error('请先转移部门成员，再停用部门。')
    }
    if (!value.name.trim()) throw new Error('请填写部门名称。')
    const exists = departments.value.find((department) => department.id === value.id)

    if (previewMode) {
      snapshot.value.departments = exists
        ? departments.value.map((department) => department.id === value.id ? { ...value } : department)
        : [...departments.value, { ...value }]
      audit('组织架构', exists ? '编辑部门' : '新建部门', value.name)
      return
    }

    const signature = departmentMutationSignature(value, exists)
    const key = (slot: string, create: () => string) => departmentMutationKey(signature, slot, create)
    try {
      const trusted = await stableDepartmentSession()
      const draft: EnterpriseDepartmentDraft = {
        name: value.name,
        parentId: value.parentId ?? '',
        leaderUserId: value.leaderId,
        email: value.email ?? '',
        phone: value.phone ?? '',
        sort: value.sort ?? 0,
        enabled: value.enabled,
      }
      let receipt
      if (exists) {
        const current = await getEnterpriseDepartment(trusted, exists.id)
        receipt = await updateEnterpriseDepartment(
          trusted,
          current,
          draft,
          key('update', () => departmentRequestId('update')),
        )
        const active = receipt.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE'
        if (value.enabled !== active) {
          receipt = value.enabled
            ? await enableEnterpriseDepartment(trusted, receipt, key('enable', () => departmentRequestId('enable')))
            : await disableEnterpriseDepartment(trusted, receipt, key('disable', () => departmentRequestId('disable')))
        }
      } else {
        receipt = await createEnterpriseDepartment(trusted, draft, key('create', () => departmentRequestId('create')))
        if (!value.enabled && receipt.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE') {
          receipt = await disableEnterpriseDepartment(trusted, receipt, key('disable', () => departmentRequestId('disable')))
        }
      }
      await getEnterpriseDepartment(trusted, receipt.departmentId)
      await refresh(['departments', 'members', 'roles'])
      departmentMutation = null
    } catch (error) {
      throw new Error(departmentRuntimeError(error))
    }
  }

  async function stableProfileSession() {
    const expected = session.value
    if (!expected?.authenticated || !expected.active_tenant_id) throw new Error('请先登录并选择可访问租户。')
    const current = await readEnterpriseTenantProfileSession()
    if (!sameTrustedSession(expected, current)) {
      await refresh()
      throw new Error('会话或当前租户已变化，请刷新后重新操作。')
    }
    return current
  }

  async function saveCompany(value: Company) {
    if (previewMode) {
      const before = JSON.stringify(company.value)
      snapshot.value.company = { ...value }
      audit('企业信息', '修改企业资料', value.name, before, JSON.stringify(value))
      return
    }

    const signature = JSON.stringify({
      name: value.name.trim(),
      shortName: value.shortName.trim(),
      industry: value.industry.trim(),
      size: value.size.trim(),
      timezone: value.timezone.trim(),
      contact: value.contact.trim(),
      phone: value.phone.trim(),
      email: value.email.trim(),
      address: value.address.trim(),
      description: value.description.trim(),
      logoAssetRef: value.logoAssetRef ?? '',
    })
    if (!companyMutation || companyMutation.tenantId !== tenantId.value || companyMutation.signature !== signature) {
      companyMutation = {
        tenantId: tenantId.value,
        signature,
        key: tenantProfileRequestId(tenantId.value),
      }
    }

    try {
      const trusted = await stableProfileSession()
      const current = await getEnterpriseTenantProfile(trusted)
      await updateEnterpriseTenantProfile(
        trusted,
        current,
        {
          name: value.name,
          shortName: value.shortName,
          industry: value.industry,
          companySize: value.size,
          timezone: value.timezone,
          contactName: value.contact,
          phone: value.phone,
          email: value.email,
          address: value.address,
          description: value.description,
          logoAssetRef: value.logoAssetRef ?? current.logoAssetRef,
        },
        companyMutation.key,
      )
      await refresh(['company'])
      companyMutation = null
    } catch (error) {
      throw new Error(tenantProfileRuntimeError(error))
    }
  }

  async function requestPasswordReset(member: Member, reason: string) {
    if (!reason.trim()) throw new Error('请填写操作原因。')
    if (!previewMode) throw new Error('真实服务当前未提供管理员密码重置命令，不能制造伪成功记录。')
    audit('成员管理', '创建密码重置请求（预览）', member.name, '未请求', '待接入身份服务', reason, 'high')
  }

  function saveSettings(values: Record<string, string | number | boolean>) {
    if (!previewMode) throw new Error('真实系统设置尚未接入服务端，不允许回退到本地预览写入。')
    const before = JSON.stringify(settings.value)
    snapshot.value.settings = { ...settings.value, ...values }
    audit('系统设置', '更新系统设置', '当前企业', before, JSON.stringify(values), '界面预览操作', 'medium')
  }

  function loginHref() {
    return loginUrl()
  }

  const unsubscribeSessionContext = previewMode
    ? null
    : subscribeSessionContextChange(() => {
        void synchronizeExternalSession()
      })
  if (import.meta.hot && unsubscribeSessionContext) {
    import.meta.hot.dispose(() => unsubscribeSessionContext())
  }
  if (!previewMode) void refresh([]).catch(() => undefined)

  return {
    tenantId,
    members,
    memberTotal,
    removedMembers,
    removedMemberTotal,
    roles,
    departments,
    company,
    logs,
    settings,
    branding,
    brandingLoading,
    brandingReady,
    brandingError,
    canManageBranding,
    previewMode,
    sourceKind,
    sourceError,
    session,
    loading,
    ready,
    authenticated,
    tenantOptions,
    departmentName,
    roleName,
    loginHref,
    refresh,
    ensureDomains,
    switchTenant,
    synchronizeExternalSession,
    logout,
    audit,
    queryMembers,
    queryRemovedMembers,
    restoreMember,
    saveMember,
    changeStatus,
    changeStatuses,
    queryRoles,
    saveRole,
    deleteRole,
    saveCompany,
    saveDepartment,
    refreshBranding,
    saveBranding,
    resetBranding,
    requestPasswordReset,
    saveSettings,
  }
})
