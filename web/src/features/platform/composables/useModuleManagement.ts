import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { backendErrorFallback } from '@/i18n/backend-terms'
import { CommercialApiError, listPlatformModules, type ModuleDTO, type ModuleTechnicalStatus, type ModuleSalesStatus } from '@/services/commercial/platformCommercial'
import {
  createPlatformModule,
  getPlatformModule,
  listPlatformModuleDefinitions,
  setPlatformModuleSalesStatus,
  setPlatformModuleTechnicalStatus,
  updatePlatformModule,
  type PlatformModuleDefinitionDTO,
} from '@/services/commercial/platformModules'
import { moduleChangeOperation, moduleReadbackMatches, type ModuleChangeKind } from '@/services/commercial/moduleAccess'
import { currentAuthorizationAllows, ensureCurrentAuthorization } from '@/services/runtime/authorization'
import { subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'

export type ModuleScreen = 'detail' | 'create' | 'createConfirm' | 'createResult' | 'metadata' | 'sales' | 'technical' | 'result' | 'conflict' | 'discard'
export type ModuleResultState = 'confirmed' | 'denied' | 'unknown'
export type ExistingModuleChangeKind = Exclude<ModuleChangeKind, 'create'>
export interface ModuleDraft { name: string; category: string; salesScope: string; technicalStatus: ModuleTechnicalStatus; reason: string }
export interface ModuleCreateDraft { moduleCode: string; name: string; category: string; salesScope: string; reason: string }

const copy = (module: ModuleDTO): ModuleDTO => ({
  ...module,
  salesScope: [...(module.salesScope ?? [])], capabilityCodes: [...(module.capabilityCodes ?? [])],
  quotaSchemaKeys: [...(module.quotaSchemaKeys ?? [])], fieldPolicySchemaKeys: [...(module.fieldPolicySchemaKeys ?? [])],
  dependencies: [...(module.dependencies ?? [])],
})
type UnresolvedWrite = { before: ModuleDTO; receipt: ModuleDTO | null; kind: ExistingModuleChangeKind; message: string; reason: string }

function asBigInt(value: string | number) {
  try { return BigInt(String(value)) } catch { return null }
}

export function useModuleManagement() {
  const modules = ref<ModuleDTO[]>([])
  const definitions = ref<PlatformModuleDefinitionDTO[]>([])
  const loadState = ref<'loading' | 'ready' | 'empty' | 'blocked' | 'error'>('loading')
  const definitionState = ref<'idle' | 'loading' | 'ready' | 'error'>('idle')
  const errorMessage = ref('')
  const definitionError = ref('')
  const readAt = ref('')
  const keyword = ref('')
  const technicalFilter = ref('')
  const salesFilter = ref('')
  const selected = ref<ModuleDTO | null>(null)
  const before = ref<ModuleDTO | null>(null)
  const receipt = ref<ModuleDTO | null>(null)
  const fresh = ref<ModuleDTO | null>(null)
  const created = ref<ModuleDTO | null>(null)
  const dialogOpen = ref(false)
  const screen = ref<ModuleScreen>('detail')
  const tab = ref('overview')
  const kind = ref<ExistingModuleChangeKind>('metadata')
  const pending = ref(false)
  const readPending = ref(false)
  const actionError = ref('')
  const resultState = ref<ModuleResultState>('unknown')
  const resultMessage = ref('')
  const createUnresolved = ref(false)
  const draft = reactive<ModuleDraft>({ name: '', category: '', salesScope: '', technicalStatus: 'MODULE_TECHNICAL_STATUS_NOT_READY', reason: '' })
  const createDraft = reactive<ModuleCreateDraft>({ moduleCode: '', name: '', category: '', salesScope: '', reason: '' })
  const unresolved = reactive(new Map<string, UnresolvedWrite>())
  let epoch = 0
  let loadSequence = 0
  let definitionSequence = 0
  let priorScreen: ModuleScreen = 'detail'

  const filteredModules = computed(() => modules.value.filter((module) => {
    const query = keyword.value.trim().toLowerCase()
    return (!query || [module.name, module.moduleCode, module.category].some((value) => String(value ?? '').toLowerCase().includes(query)))
      && (!technicalFilter.value || module.technicalStatus === technicalFilter.value)
      && (!salesFilter.value || module.salesStatus === salesFilter.value)
  }))
  const summary = computed(() => ({
    total: modules.value.length,
    ready: modules.value.filter((item) => item.technicalStatus === 'MODULE_TECHNICAL_STATUS_READY').length,
    sellable: modules.value.filter((item) => item.salesStatus === 'MODULE_SALES_STATUS_SELLABLE').length,
    notReady: modules.value.filter((item) => item.technicalStatus !== 'MODULE_TECHNICAL_STATUS_READY').length,
  }))
  const existingCodes = computed(() => new Set(modules.value.map((item) => item.moduleCode)))
  const creatableDefinitions = computed(() => definitions.value.filter((item) => !existingCodes.value.has(item.moduleCode)))
  const selectedCreateDefinition = computed(() => creatableDefinitions.value.find((item) => item.moduleCode === createDraft.moduleCode) ?? null)
  const missingCreateDependencies = computed(() => (selectedCreateDefinition.value?.dependencies ?? []).filter((code) => !existingCodes.value.has(code)))
  const nextSales = ref<ModuleSalesStatus>('MODULE_SALES_STATUS_RETIRED')
  const busy = computed(() => pending.value || readPending.value)
  const writeUnresolved = computed(() => selected.value ? unresolved.has(selected.value.moduleCode) : false)
  const scopes = () => [...new Set(draft.salesScope.split(/[,，\n]/).map((value) => value.trim()).filter(Boolean))]
  const createScopes = () => [...new Set(createDraft.salesScope.split(/[,，\n]/).map((value) => value.trim()).filter(Boolean))]

  async function operationAllowed(operation: string, label: string) {
    await ensureCurrentAuthorization(true)
    if (currentAuthorizationAllows(operation)) return true
    actionError.value = `当前平台授权已变化，已阻止${label}。草稿仍保留；请联系平台管理员核对授权。`
    return false
  }
  function resetFilters() {
    keyword.value = ''; technicalFilter.value = ''; salesFilter.value = ''
  }
  function replace(module: ModuleDTO) {
    modules.value = modules.value.map((item) => item.moduleCode === module.moduleCode ? module : item)
    selected.value = copy(module)
  }
  function upsert(module: ModuleDTO) {
    const exists = modules.value.some((item) => item.moduleCode === module.moduleCode)
    modules.value = exists
      ? modules.value.map((item) => item.moduleCode === module.moduleCode ? module : item)
      : [...modules.value, module]
  }
  function resetCreate() {
    Object.assign(createDraft, { moduleCode: '', name: '', category: '', salesScope: '', reason: '' })
    created.value = null
    receipt.value = null
    fresh.value = null
    createUnresolved.value = false
    resultState.value = 'unknown'
    resultMessage.value = ''
    actionError.value = ''
  }

  async function loadModules() {
    const sequence = ++loadSequence
    const generation = epoch
    loadState.value = 'loading'
    errorMessage.value = ''
    try {
      const items = await listPlatformModules()
      if (sequence !== loadSequence || generation !== epoch) return
      modules.value = items
      loadState.value = items.length ? 'ready' : 'empty'
      readAt.value = new Date().toLocaleString('zh-CN', { hour12: false })
    } catch (error) {
      if (sequence !== loadSequence || generation !== epoch) return
      modules.value = []
      loadState.value = error instanceof CommercialApiError && [401, 403].includes(error.status) ? 'blocked' : 'error'
      errorMessage.value = loadState.value === 'blocked' ? '请使用具有平台模块读取权限的账号。' : '读取失败，没有使用示例数据替代。请重试。'
    }
  }
  async function loadDefinitions() {
    const sequence = ++definitionSequence
    const generation = epoch
    definitionState.value = 'loading'
    definitionError.value = ''
    try {
      const items = await listPlatformModuleDefinitions()
      if (sequence !== definitionSequence || generation !== epoch) return
      definitions.value = items
      definitionState.value = 'ready'
    } catch {
      if (sequence !== definitionSequence || generation !== epoch) return
      definitions.value = []
      definitionState.value = 'error'
      definitionError.value = '无法读取可新增模块定义。请确认当前账号具备平台模块读取权限后重试。'
    }
  }
  async function openCreate() {
    actionError.value = ''
    if (!await operationAllowed(moduleChangeOperation.create, '新增模块')) return
    resetCreate()
    selected.value = null
    before.value = null
    screen.value = 'create'
    dialogOpen.value = true
    void loadDefinitions()
  }
  function continueCreate() {
    actionError.value = ''
    if (definitionState.value !== 'ready') { actionError.value = '请等待模块定义读取完成。'; return }
    if (!selectedCreateDefinition.value) { actionError.value = '请选择一个尚未创建的模块定义。'; return }
    if (missingCreateDependencies.value.length) { actionError.value = '请先创建该模块所依赖的模块。'; return }
    if (!createDraft.name.trim() || !createDraft.category.trim()) { actionError.value = '请填写模块名称和分类。'; return }
    if (!createDraft.reason.trim()) { actionError.value = '请填写创建原因。'; return }
    screen.value = 'createConfirm'
  }
  async function refreshCreatedModule() {
    if (!createDraft.moduleCode || busy.value) return
    const generation = epoch
    readPending.value = true
    actionError.value = ''
    try {
      const value = await getPlatformModule(createDraft.moduleCode)
      if (generation !== epoch || value.moduleCode !== createDraft.moduleCode) return
      created.value = copy(value)
      upsert(value)
      if (receipt.value && moduleReadbackMatches(receipt.value, value, 'create')) {
        createUnresolved.value = false
        resultState.value = 'confirmed'
        resultMessage.value = '模块目录记录已创建并核对。初始销售状态保持停售。'
      } else {
        resultState.value = 'unknown'
        resultMessage.value = '已读取到同代码模块，但没有可靠回执可以证明它由本次请求创建。请保留操作信息，不要重复提交。'
      }
      screen.value = 'createResult'
    } catch {
      if (generation !== epoch) return
      resultState.value = 'unknown'
      resultMessage.value = '尚未取得可确认创建结果的最新状态。创建请求可能已经执行，请不要重复提交。'
      screen.value = 'createResult'
    } finally {
      if (generation === epoch) readPending.value = false
    }
  }
  async function submitCreate() {
    if (busy.value || createUnresolved.value || screen.value !== 'createConfirm' || !selectedCreateDefinition.value) return
    actionError.value = ''
    if (!await operationAllowed(moduleChangeOperation.create, '创建提交')) return
    if (missingCreateDependencies.value.length) { actionError.value = '模块依赖已变化，请返回重新核对。'; return }
    const generation = epoch
    pending.value = true
    try {
      const value = await createPlatformModule({
        moduleCode: createDraft.moduleCode,
        name: createDraft.name,
        category: createDraft.category,
        salesScope: createScopes(),
        reason: createDraft.reason,
      })
      if (generation !== epoch) return
      receipt.value = copy(value)
      created.value = copy(value)
      createUnresolved.value = true
      resultState.value = 'unknown'
      resultMessage.value = '已收到创建回执，正在核对最新状态；尚未确认完成。'
      screen.value = 'createResult'
      pending.value = false
      await refreshCreatedModule()
    } catch (error) {
      if (generation !== epoch) return
      if (error instanceof CommercialApiError && [401, 403].includes(error.status)) {
        createUnresolved.value = false
        resultState.value = 'denied'
        resultMessage.value = '当前账号没有执行创建操作的权限或会话已失效。没有显示为成功。'
        screen.value = 'createResult'
      } else if (error instanceof CommercialApiError && error.status >= 400 && error.status < 500 && error.code !== 'invalid-response') {
        createUnresolved.value = false
        actionError.value = backendErrorFallback('commercial')
      } else {
        createUnresolved.value = true
        resultState.value = 'unknown'
        resultMessage.value = '未取得可靠的创建回执，操作可能已经执行。请先读取最新状态，结果明确前不要重复提交。'
        screen.value = 'createResult'
      }
    } finally {
      if (generation === epoch) pending.value = false
    }
  }

  function openDetail(module: ModuleDTO) {
    selected.value = copy(module)
    before.value = null
    fresh.value = null
    receipt.value = null
    tab.value = 'overview'
    screen.value = 'detail'
    actionError.value = ''
    dialogOpen.value = true
    const unresolvedWrite = unresolved.get(module.moduleCode)
    if (unresolvedWrite) {
      before.value = copy(unresolvedWrite.before)
      receipt.value = unresolvedWrite.receipt ? copy(unresolvedWrite.receipt) : null
      kind.value = unresolvedWrite.kind
      draft.reason = unresolvedWrite.reason
      resultState.value = 'unknown'
      resultMessage.value = unresolvedWrite.message
      screen.value = 'result'
    }
  }
  async function startChange(value: ExistingModuleChangeKind) {
    if (!selected.value || busy.value || writeUnresolved.value) return
    actionError.value = ''
    if (!await operationAllowed(moduleChangeOperation[value], '打开该变更')) return
    kind.value = value
    nextSales.value = selected.value.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? 'MODULE_SALES_STATUS_RETIRED' : 'MODULE_SALES_STATUS_SELLABLE'
    before.value = copy(selected.value)
    receipt.value = null
    fresh.value = null
    Object.assign(draft, {
      name: selected.value.name, category: selected.value.category,
      salesScope: (selected.value.salesScope ?? []).join(', '),
      technicalStatus: selected.value.technicalStatus, reason: '',
    })
    actionError.value = ''
    screen.value = value
  }
  function requestClose() {
    if (busy.value) return
    if (['create', 'createConfirm', 'metadata', 'sales', 'technical'].includes(screen.value)) {
      priorScreen = screen.value
      screen.value = 'discard'
      return
    }
    dialogOpen.value = false
  }
  function discard() {
    actionError.value = ''
    if (priorScreen.startsWith('create')) {
      resetCreate()
      dialogOpen.value = false
      screen.value = 'detail'
      return
    }
    screen.value = 'detail'
  }
  function continueEditing() { screen.value = priorScreen }
  function backToDetail() {
    if (busy.value) return
    actionError.value = ''
    if (screen.value === 'createResult' && created.value && resultState.value === 'confirmed') {
      selected.value = copy(created.value)
    }
    screen.value = 'detail'
  }
  function markUnknown(message: string) {
    resultState.value = 'unknown'
    resultMessage.value = message
    if (before.value) unresolved.set(before.value.moduleCode, {
      before: copy(before.value), receipt: receipt.value && intendedReceipt(receipt.value) ? copy(receipt.value) : null,
      kind: kind.value, message, reason: draft.reason,
    })
    screen.value = 'result'
  }
  function intendedReceipt(value: ModuleDTO) {
    const base = before.value
    if (!base || value.moduleCode !== base.moduleCode) return false
    const baseVersion = asBigInt(base.version)
    const responseVersion = asBigInt(value.version)
    if (baseVersion === null || responseVersion === null) return false
    if (kind.value === 'sales') {
      return responseVersion === baseVersion && value.salesStatus === nextSales.value && value.technicalStatus === base.technicalStatus
    }
    if (responseVersion !== baseVersion + 1n) return false
    if (kind.value === 'technical') {
      return value.technicalStatus === draft.technicalStatus && value.salesStatus === 'MODULE_SALES_STATUS_RETIRED'
    }
    return value.name === draft.name.trim() && value.category === draft.category.trim()
      && value.technicalStatus === base.technicalStatus && value.salesStatus === 'MODULE_SALES_STATUS_RETIRED'
      && [...(value.salesScope ?? [])].sort().join('\n') === scopes().sort().join('\n')
  }
  async function refreshModule() {
    if (!selected.value || busy.value) return
    const generation = epoch
    const code = selected.value.moduleCode
    readPending.value = true
    actionError.value = ''
    fresh.value = null
    try {
      const value = await getPlatformModule(code)
      if (generation !== epoch || selected.value?.moduleCode !== code) return
      if (value.moduleCode !== code) throw new Error('Unexpected module response')
      fresh.value = value
      replace(value)
      const unresolvedWrite = unresolved.get(code)
      if (unresolvedWrite?.receipt && moduleReadbackMatches(unresolvedWrite.receipt, value, unresolvedWrite.kind)) {
        unresolved.delete(code)
        resultState.value = 'confirmed'
        resultMessage.value = kind.value === 'metadata' ? '模块基础配置已更新；新目录版本自动停售，最新状态已确认。'
          : kind.value === 'sales' ? (value.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? '模块已恢复可销售状态；目录版本未改变。' : '模块已停售；目录版本与技术状态未改变。')
            : '技术状态已更新；新目录版本自动停售的结果已确认。'
        screen.value = 'result'
      } else if (screen.value === 'result' && resultState.value === 'unknown') {
        resultMessage.value = '最新模块状态已读取，但无法确认原提交的最终结果。请保留操作信息并联系平台支持，不要重复提交。'
      }
    } catch {
      if (generation !== epoch) return
      actionError.value = '最新状态读取失败。当前草稿和原版本已保留；请重新读取，不要重复提交。'
    } finally {
      if (generation === epoch) readPending.value = false
    }
  }
  async function submit() {
    const base = before.value
    if (!base || busy.value || writeUnresolved.value || !['metadata', 'sales', 'technical'].includes(screen.value)) return
    actionError.value = ''
    if (!await operationAllowed(moduleChangeOperation[kind.value], '变更提交')) return
    if (!draft.reason.trim()) { actionError.value = '请填写本次变更原因。'; return }
    if (kind.value === 'metadata' && (!draft.name.trim() || !draft.category.trim())) {
      actionError.value = '模块名称和分类不能为空。'; return
    }
    if (kind.value === 'sales' && nextSales.value === base.salesStatus) {
      actionError.value = '最新状态已与目标相同；请返回详情核对，无需重复提交。'; return
    }
    if (kind.value === 'technical' && draft.technicalStatus === base.technicalStatus) {
      actionError.value = '请选择不同的目标技术状态。'; return
    }
    const generation = epoch
    pending.value = true
    try {
      const value = kind.value === 'metadata'
        ? await updatePlatformModule(base, { name: draft.name, category: draft.category, salesScope: scopes(), reason: draft.reason })
        : kind.value === 'sales' ? await setPlatformModuleSalesStatus(base, nextSales.value, draft.reason)
          : await setPlatformModuleTechnicalStatus(base, draft.technicalStatus, draft.reason)
      if (generation !== epoch) return
      if (!intendedReceipt(value)) {
        actionError.value = '系统返回的状态与本次操作预期不一致。没有显示为成功，请重新读取最新状态。'
        receipt.value = copy(value)
        markUnknown('写入回执与预期目录规则不一致。请先读取最新状态，结果明确前不要重复提交。')
        return
      }
      receipt.value = copy(value)
      markUnknown('已收到写入回执，正在核对最新状态；尚未确认完成。')
      pending.value = false
      await refreshModule()
    } catch (error) {
      if (generation !== epoch) return
      if (error instanceof CommercialApiError && error.code === 'conflict') {
        screen.value = 'conflict'
        actionError.value = '模块版本已变化。原草稿仍保留；读取最新状态并核对差异后，再决定是否提交。'
        pending.value = false
        await refreshModule()
      } else if (error instanceof CommercialApiError && [401, 403].includes(error.status)) {
        screen.value = 'result'
        resultState.value = 'denied'
        resultMessage.value = '当前账号没有执行该操作的权限或会话已失效。没有显示为成功；请联系平台管理员核对授权。'
      } else if (error instanceof CommercialApiError && error.status >= 400 && error.status < 500 && error.code !== 'invalid-response') {
        actionError.value = backendErrorFallback('commercial')
      } else {
        markUnknown('未取得可靠的完成回执，操作可能已经执行。请先读取最新状态，结果明确前不要重复提交。')
      }
    } finally {
      if (generation === epoch) pending.value = false
    }
  }
  function acceptFreshVersion() {
    if (!fresh.value || busy.value) return
    before.value = copy(fresh.value)
    selected.value = copy(fresh.value)
    actionError.value = ''
    screen.value = kind.value
  }
  const unsubscribe = subscribeSessionContextChange(() => {
    epoch += 1
    loadSequence += 1
    definitionSequence += 1
    dialogOpen.value = false
    selected.value = null
    before.value = null
    receipt.value = null
    fresh.value = null
    created.value = null
    pending.value = false
    readPending.value = false
    definitions.value = []
    definitionState.value = 'idle'
    unresolved.clear()
    resetCreate()
    Object.assign(draft, { name: '', category: '', salesScope: '', reason: '' })
    resetFilters()
    void loadModules()
  })
  onMounted(loadModules)
  onBeforeUnmount(() => { epoch += 1; unsubscribe() })
  return {
    modules, definitions, loadState, definitionState, errorMessage, definitionError, readAt, keyword, technicalFilter, salesFilter, filteredModules, summary,
    selected, before, receipt, fresh, created, dialogOpen, screen, tab, kind, draft, createDraft, pending, readPending, busy,
    actionError, resultState, resultMessage, nextSales, writeUnresolved, createUnresolved, creatableDefinitions, selectedCreateDefinition, missingCreateDependencies,
    resetFilters, loadModules, loadDefinitions, openCreate, continueCreate, submitCreate, refreshCreatedModule, openDetail, startChange, requestClose, discard, continueEditing,
    backToDetail, submit, refreshModule, acceptFreshVersion,
  }
}
