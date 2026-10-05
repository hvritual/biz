import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useModuleManagement } from './useModuleManagement'
import type { ModuleDTO } from '@/services/commercial/platformCommercial'

const mocks = vi.hoisted(() => ({
  ensureCurrentAuthorization: vi.fn(),
  currentAuthorizationAllows: vi.fn(),
  listPlatformModules: vi.fn(),
  createPlatformModule: vi.fn(),
  getPlatformModule: vi.fn(),
  listPlatformModuleDefinitions: vi.fn(),
  setPlatformModuleSalesStatus: vi.fn(),
  setPlatformModuleTechnicalStatus: vi.fn(),
  updatePlatformModule: vi.fn(),
  subscribe: vi.fn(() => vi.fn()),
}))

vi.mock('@/services/runtime/authorization', () => ({
  ensureCurrentAuthorization: mocks.ensureCurrentAuthorization,
  currentAuthorizationAllows: mocks.currentAuthorizationAllows,
}))

vi.mock('@/services/runtime/sessionCoordinator', () => ({
  subscribeSessionContextChange: mocks.subscribe,
}))

vi.mock('@/services/commercial/platformCommercial', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/services/commercial/platformCommercial')>()
  return {
    ...actual,
    listPlatformModules: mocks.listPlatformModules,
  }
})

vi.mock('@/services/commercial/platformModules', () => ({
  createPlatformModule: mocks.createPlatformModule,
  getPlatformModule: mocks.getPlatformModule,
  listPlatformModuleDefinitions: mocks.listPlatformModuleDefinitions,
  setPlatformModuleSalesStatus: mocks.setPlatformModuleSalesStatus,
  setPlatformModuleTechnicalStatus: mocks.setPlatformModuleTechnicalStatus,
  updatePlatformModule: mocks.updatePlatformModule,
}))

const initial: ModuleDTO = {
  moduleCode: 'access-management',
  name: '成员与权限',
  category: 'access',
  salesScope: ['default'],
  technicalStatus: 'MODULE_TECHNICAL_STATUS_READY',
  salesStatus: 'MODULE_SALES_STATUS_SELLABLE',
  capabilityCodes: ['tenant.lifecycle', 'tenant.member.lifecycle', 'tenant.role.permission'],
  quotaSchemaKeys: ['member.count'],
  fieldPolicySchemaKeys: [],
  dependencies: [],
  version: '9',
}

const definition = {
  moduleCode: 'device-operations',
  capabilityCodes: ['device.lifecycle', 'device.transfer'],
  quotaSchemaKeys: ['tenant.devices'],
  fieldPolicySchemaKeys: ['device.identity'],
  dependencies: [],
  implementationReady: true,
}

function mountManagement() {
  let vm!: ReturnType<typeof useModuleManagement>
  const wrapper = mount(defineComponent({
    setup() {
      vm = useModuleManagement()
      return () => h('div')
    },
  }))
  return { vm, wrapper }
}

function deferredAuthorization() {
  let release!: () => void
  const promise = new Promise<null>((resolve) => {
    release = () => resolve(null)
  })
  return { promise, release }
}

describe('module write authorization mutex', () => {
  beforeEach(() => {
    for (const mock of Object.values(mocks)) {
      if ('mockReset' in mock) mock.mockReset()
    }
    mocks.subscribe.mockReturnValue(vi.fn())
    mocks.currentAuthorizationAllows.mockReturnValue(true)
    mocks.ensureCurrentAuthorization.mockResolvedValue(null)
    mocks.listPlatformModules.mockResolvedValue([initial])
    mocks.listPlatformModuleDefinitions.mockResolvedValue([definition])
  })

  it('allows only one metadata write while live authorization revalidation is pending', async () => {
    const updated: ModuleDTO = {
      ...initial,
      name: '成员与权限（更新）',
      salesStatus: 'MODULE_SALES_STATUS_RETIRED',
      version: '10',
    }
    mocks.updatePlatformModule.mockResolvedValue(updated)
    mocks.getPlatformModule.mockResolvedValue(updated)

    const { vm, wrapper } = mountManagement()
    await nextTick()
    vm.openDetail(initial)
    await vm.startChange('metadata')
    vm.draft.name = updated.name
    vm.draft.reason = '双击提交回归'

    const auth = deferredAuthorization()
    mocks.ensureCurrentAuthorization.mockImplementationOnce(() => auth.promise)

    const first = vm.submit()
    const second = vm.submit()

    expect(vm.busy.value).toBe(true)
    expect(mocks.ensureCurrentAuthorization).toHaveBeenCalledTimes(2)
    expect(mocks.updatePlatformModule).not.toHaveBeenCalled()

    auth.release()
    await Promise.all([first, second])

    expect(mocks.updatePlatformModule).toHaveBeenCalledTimes(1)
    expect(vm.resultState.value).toBe('confirmed')
    wrapper.unmount()
  })

  it('does not open create after the session context changes during authorization', async () => {
    const auth = deferredAuthorization()
    mocks.ensureCurrentAuthorization.mockImplementationOnce(() => auth.promise)

    const { vm, wrapper } = mountManagement()
    await nextTick()
    const contextChanged = mocks.subscribe.mock.calls[0]?.[0] as (() => void) | undefined
    expect(contextChanged).toBeTypeOf('function')

    const opening = vm.openCreate()
    expect(vm.busy.value).toBe(true)
    contextChanged?.()
    auth.release()
    await opening
    await nextTick()

    expect(vm.dialogOpen.value).toBe(false)
    expect(vm.screen.value).toBe('detail')
    expect(vm.selected.value).toBeNull()
    wrapper.unmount()
  })

  it('does not continue an old module change after the session context changes during authorization', async () => {
    const { vm, wrapper } = mountManagement()
    await nextTick()
    vm.openDetail(initial)

    const auth = deferredAuthorization()
    mocks.ensureCurrentAuthorization.mockImplementationOnce(() => auth.promise)
    const contextChanged = mocks.subscribe.mock.calls[0]?.[0] as (() => void) | undefined
    expect(contextChanged).toBeTypeOf('function')

    const changing = vm.startChange('metadata')
    expect(vm.busy.value).toBe(true)
    contextChanged?.()
    auth.release()
    await expect(changing).resolves.toBeUndefined()
    await nextTick()

    expect(vm.dialogOpen.value).toBe(false)
    expect(vm.selected.value).toBeNull()
    expect(vm.before.value).toBeNull()
    expect(vm.screen.value).toBe('detail')
    wrapper.unmount()
  })

  it('allows only one create request while live authorization revalidation is pending', async () => {
    const created: ModuleDTO = {
      ...initial,
      moduleCode: definition.moduleCode,
      name: '设备运营',
      category: 'device',
      salesScope: ['default'],
      salesStatus: 'MODULE_SALES_STATUS_RETIRED',
      capabilityCodes: definition.capabilityCodes,
      quotaSchemaKeys: definition.quotaSchemaKeys,
      fieldPolicySchemaKeys: definition.fieldPolicySchemaKeys,
      dependencies: definition.dependencies,
      version: '1',
    }
    mocks.createPlatformModule.mockResolvedValue(created)
    mocks.getPlatformModule.mockResolvedValue(created)

    const { vm, wrapper } = mountManagement()
    await nextTick()
    vm.definitions.value = [definition]
    Object.assign(vm.createDraft, {
      moduleCode: definition.moduleCode,
      name: created.name,
      category: created.category,
      salesScope: 'default',
      reason: '双击创建回归',
    })
    vm.screen.value = 'createConfirm'

    const auth = deferredAuthorization()
    mocks.ensureCurrentAuthorization.mockImplementationOnce(() => auth.promise)

    const first = vm.submitCreate()
    const second = vm.submitCreate()

    expect(vm.busy.value).toBe(true)
    expect(mocks.ensureCurrentAuthorization).toHaveBeenCalledTimes(1)
    expect(mocks.createPlatformModule).not.toHaveBeenCalled()

    auth.release()
    await Promise.all([first, second])

    expect(mocks.createPlatformModule).toHaveBeenCalledTimes(1)
    expect(vm.resultState.value).toBe('confirmed')
    wrapper.unmount()
  })
})
