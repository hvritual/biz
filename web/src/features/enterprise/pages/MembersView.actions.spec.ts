import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import type { Member } from '@/types/enterprise'

// These test the real page's UI projection with controlled store/identity
// doubles. The CE13 extension separately verifies real IdP/BFF/MySQL writes.
const fixture = vi.hoisted(() => ({
  codes: new Set<string>(),
  store: {
    previewMode: false, tenantId: 'test-tenant',
    session: { user_id: 'admin', context_version: 1 },
    members: [] as Member[], removedMembers: [] as Member[],
    memberTotal: 0, memberQueryLoading: false, memberQueryError: '',
    ensureDomains: vi.fn(async () => undefined),
    queryMembers: vi.fn(async () => true),
  },
}))
vi.mock('@/stores/enterprise', () => ({
  useEnterpriseStore: () => fixture.store,
  MemberStatusMutationError: class extends Error {},
  memberMutationNeedsInspection: () => false,
}))
vi.mock('@/stores/ui', () => ({ useUiStore: () => ({ module: 'members', toast: vi.fn() }) }))
vi.mock('@/services/runtime/authorization', () => ({
  currentAuthorizationAllows: (code: string) => fixture.codes.has(code),
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/enterprise/members', query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key, locale: ref('zh-CN') }),
}))
import MembersView from './MembersView.vue'
import MemberActionDialog from '../components/members/MemberActionDialog.vue'
import MemberDetailDrawer from '../components/members/MemberDetailDrawer.vue'
import MemberTable from '../components/members/MemberTable.vue'

let wrapper: VueWrapper | undefined
beforeEach(() => {
  fixture.codes = new Set()
  fixture.store.previewMode = false
  fixture.store.members = [{
    id: 'member-1', username: 'test.member', name: 'Test member',
    email: 'member@example.invalid', phone: '', employeeId: '', position: '',
    departmentId: '', roleIds: ['role-viewer'], scope: 'all', status: 'active',
    online: false, joinedAt: '', lastLogin: null, version: 1, mfa: false, note: '',
  }]
  fixture.store.memberTotal = 1
  vi.clearAllMocks()
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined })

async function page(codes: string[]) {
  fixture.codes = new Set(codes)
  wrapper = shallowMount(MembersView, {
    global: { stubs: { UiButton: { template: '<button><slot /></button>' } } },
  })
  await flushPromises()
  return wrapper
}
function button(key: string) {
  return wrapper!.findAll('button').find((item) => item.text() === key)
}

describe('member page canonical operation projection', () => {
  it('enables API-mode create and invite from the actual atomic create action alone', async () => {
    const view = await page(['tenant.member.create'])
    expect(button('members.add')).toBeTruthy()
    expect(button('members.invite')).toBeTruthy()
    await button('members.add')!.trigger('click')
    expect(view.findComponent(MemberActionDialog).props()).toMatchObject({ open: true, action: 'create' })
    view.findComponent(MemberActionDialog).vm.$emit('close')
    await flushPromises()
    await button('members.invite')!.trigger('click')
    expect(view.findComponent(MemberActionDialog).props()).toMatchObject({ open: true, action: 'invite' })
  })
  it('does not substitute old multi-step actions for atomic create or update', async () => {
    const view = await page([
      'tenant.member.invite', 'tenant.member.profile.update',
      'tenant.role.assign_member', 'tenant.role.revoke_member', 'tenant.member.activate',
    ])
    expect(button('members.add')).toBeUndefined()
    expect(button('members.invite')).toBeUndefined()
    expect(view.findComponent(MemberTable).props('canEdit')).toBe(false)
    expect(view.findComponent(MemberDetailDrawer).props('canChangeRoles')).toBe(false)
    // Even a stale child event cannot open an unauthorized edit form.
    view.findComponent(MemberTable).vm.$emit('edit', fixture.store.members[0])
    await flushPromises()
    expect(view.findComponent(MemberActionDialog).props('open')).toBe(false)
  })
  it('uses atomic member.update for edit and initial-role replacement', async () => {
    const view = await page(['tenant.member.update'])
    expect(button('members.add')).toBeUndefined()
    expect(view.findComponent(MemberTable).props('canEdit')).toBe(true)
    expect(view.findComponent(MemberDetailDrawer).props('canChangeRoles')).toBe(true)
    view.findComponent(MemberTable).vm.$emit('edit', fixture.store.members[0])
    await flushPromises()
    expect(view.findComponent(MemberActionDialog).props()).toMatchObject({ open: true, action: 'edit' })
    view.findComponent(MemberActionDialog).vm.$emit('close')
    await flushPromises()
    view.findComponent(MemberDetailDrawer).vm.$emit('action', 'role', fixture.store.members[0])
    await flushPromises()
    expect(view.findComponent(MemberActionDialog).props()).toMatchObject({ open: true, action: 'role' })
  })
  it('keeps a read-only projection non-mutating without hiding the members list', async () => {
    const view = await page(['tenant.member.list'])
    expect(view.findComponent(MemberTable).exists()).toBe(true)
    expect(button('members.add')).toBeUndefined()
    expect(button('members.invite')).toBeUndefined()
    expect(view.findComponent(MemberTable).props('canEdit')).toBe(false)
    expect(view.findComponent(MemberDetailDrawer).props('canChangeRoles')).toBe(false)
  })
})
