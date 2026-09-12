<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import {
  CommercialApiError,
  checkPlanEligibility,
  commercialRequestId,
  createPlanDraft,
  createPlanVersion,
  listPlanVersions,
  listPlatformModules,
  publishPlanVersion,
  retirePlanVersion,
  updatePlanDraft,
  type ModuleDTO,
  type PlanModule,
  type PlanTerms,
  type PlanVersionDTO,
} from '@/services/commercial/platformCommercial'

type LoadState = 'idle' | 'loading' | 'ready' | 'empty' | 'blocked' | 'error'
type EditorMode = 'create' | 'edit'

const planCodeInput = ref('')
const activePlanCode = ref('')
const versions = ref<PlanVersionDTO[]>([])
const nextAfterVersion = ref<string | number>('')
const selectedVersionKey = ref('')
const loadState = ref<LoadState>('idle')
const errorMessage = ref('')
const actionMessage = ref('')
const actionError = ref('')
const actionPending = ref(false)

const modules = ref<ModuleDTO[]>([])
const moduleLoadError = ref('')

const editorOpen = ref(false)
const editorMode = ref<EditorMode>('create')
const editor = reactive({
  planCode: '',
  name: '',
  reason: '',
  terms: emptyTerms(),
})

const eligibilityOpen = ref(false)
const eligibilityScope = ref('')
const eligibilityResult = ref<{ eligible: boolean; reason: string } | null>(null)

const displayedVersions = computed(() => [...versions.value].sort((left, right) => {
  const a = Number(left.version)
  const b = Number(right.version)
  if (Number.isFinite(a) && Number.isFinite(b)) return b - a
  return String(right.version).localeCompare(String(left.version))
}))

const selectedVersion = computed(() => displayedVersions.value.find((item) => versionKey(item) === selectedVersionKey.value) ?? displayedVersions.value[0])

const summary = computed(() => ({
  count: versions.value.length,
  drafts: versions.value.filter((item) => item.state === 'DRAFT').length,
  published: versions.value.filter((item) => item.state === 'PUBLISHED').length,
  retired: versions.value.filter((item) => item.state === 'RETIRED').length,
}))

function emptyTerms(): PlanTerms {
  return {
    modules: [],
    salesScope: [],
    validityMode: 'unlimited',
    validityDays: 0,
    priceRef: '',
  }
}

function normalizeTerms(terms?: Partial<PlanTerms>): PlanTerms {
  return {
    modules: Array.isArray(terms?.modules)
      ? terms.modules.map((item) => ({
        moduleCode: item.moduleCode ?? '',
        capabilityCodes: Array.isArray(item.capabilityCodes) ? [...item.capabilityCodes] : [],
        quotas: Array.isArray(item.quotas) ? item.quotas.map((quota) => ({ ...quota })) : [],
        fields: Array.isArray(item.fields) ? item.fields.map((field) => ({ ...field })) : [],
      }))
      : [],
    salesScope: Array.isArray(terms?.salesScope) ? [...terms.salesScope] : [],
    validityMode: terms?.validityMode || 'unlimited',
    validityDays: Number(terms?.validityDays || 0),
    priceRef: terms?.priceRef || '',
  }
}

function versionKey(version: PlanVersionDTO) {
  return `${version.planCode}:${version.version}`
}

function statusLabel(state: string) {
  const labels: Record<string, string> = { DRAFT: '草稿', PUBLISHED: '已发布', RETIRED: '已停售' }
  return labels[state] || state || '未知'
}

function statusTone(state: string): 'success' | 'warning' | 'neutral' {
  if (state === 'PUBLISHED') return 'success'
  if (state === 'DRAFT') return 'warning'
  return 'neutral'
}

function formatTime(value: string) {
  if (!value) return '—'
  const parsed = new Date(value)
  return Number.isNaN(parsed.valueOf()) ? value : parsed.toLocaleString('zh-CN', { hour12: false })
}

function validityLabel(terms?: PlanTerms) {
  if (!terms) return '—'
  if (terms.validityMode === 'fixed_days') return `${terms.validityDays} 天`
  return terms.validityMode === 'unlimited' ? '长期有效' : terms.validityMode || '—'
}

function selectedModule(moduleCode: string) {
  return modules.value.find((item) => item.moduleCode === moduleCode)
}

function resetMessages() {
  errorMessage.value = ''
  actionMessage.value = ''
  actionError.value = ''
}

async function loadModules() {
  moduleLoadError.value = ''
  try {
    modules.value = await listPlatformModules()
  } catch (error) {
    modules.value = []
    moduleLoadError.value = error instanceof Error ? error.message : '模块目录读取失败'
  }
}

async function loadVersions(reset = true) {
  const code = (reset ? planCodeInput.value : activePlanCode.value).trim()
  if (!code) {
    loadState.value = 'idle'
    activePlanCode.value = ''
    versions.value = []
    return
  }

  resetMessages()
  if (reset) {
    loadState.value = 'loading'
    activePlanCode.value = code
    versions.value = []
    nextAfterVersion.value = ''
    selectedVersionKey.value = ''
  }

  try {
    const result = await listPlanVersions(code, {
      afterVersion: reset ? undefined : nextAfterVersion.value,
      pageSize: 50,
    })
    versions.value = reset ? result.versions : [...versions.value, ...result.versions]
    nextAfterVersion.value = result.nextAfterVersion
    loadState.value = versions.value.length ? 'ready' : 'empty'
    if (!selectedVersionKey.value && displayedVersions.value[0]) {
      selectedVersionKey.value = versionKey(displayedVersions.value[0])
    }
  } catch (error) {
    if (error instanceof CommercialApiError && error.status === 404) {
      loadState.value = 'empty'
      return
    }
    if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
      loadState.value = 'blocked'
      errorMessage.value = error.message
      return
    }
    loadState.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : '套餐版本读取失败'
  }
}

function openCreate() {
  editorMode.value = 'create'
  editor.planCode = activePlanCode.value || planCodeInput.value.trim()
  editor.name = ''
  editor.reason = '平台控制台创建套餐首稿'
  editor.terms = emptyTerms()
  editorOpen.value = true
}

function openEdit() {
  const selected = selectedVersion.value
  if (!selected || selected.state !== 'DRAFT') return
  editorMode.value = 'edit'
  editor.planCode = selected.planCode
  editor.name = selected.name
  editor.reason = '平台控制台修改套餐草稿'
  editor.terms = normalizeTerms(selected.terms)
  editorOpen.value = true
}

function addPlanModule() {
  editor.terms.modules.push({ moduleCode: '', capabilityCodes: [], quotas: [], fields: [] })
}

function removePlanModule(index: number) {
  editor.terms.modules.splice(index, 1)
}

function onModuleChanged(item: PlanModule) {
  item.capabilityCodes = []
  item.quotas = []
  item.fields = []
}

function toggleCapability(item: PlanModule, capability: string, event: Event) {
  const checked = (event.target as HTMLInputElement | null)?.checked ?? false
  item.capabilityCodes = checked
    ? Array.from(new Set([...item.capabilityCodes, capability]))
    : item.capabilityCodes.filter((value) => value !== capability)
}

function addQuota(item: PlanModule) {
  const catalog = selectedModule(item.moduleCode)
  const firstUnused = catalog?.quotaSchemaKeys.find((key) => !item.quotas.some((quota) => quota.key === key)) ?? ''
  item.quotas.push({ key: firstUnused, unlimited: false, value: 0 })
}

function removeQuota(item: PlanModule, index: number) {
  item.quotas.splice(index, 1)
}

function addField(item: PlanModule) {
  const catalog = selectedModule(item.moduleCode)
  const firstUnused = catalog?.fieldPolicySchemaKeys.find((key) => !item.fields.some((field) => field.key === key)) ?? ''
  item.fields.push({ key: firstUnused, action: 'read', mode: 'deny' })
}

function removeField(item: PlanModule, index: number) {
  item.fields.splice(index, 1)
}

function normalizedEditorTerms(): PlanTerms {
  return {
    modules: editor.terms.modules.map((item) => ({
      moduleCode: item.moduleCode.trim(),
      capabilityCodes: item.capabilityCodes.filter(Boolean),
      quotas: item.quotas.map((quota) => ({
        key: quota.key.trim(),
        unlimited: Boolean(quota.unlimited),
        value: quota.unlimited ? 0 : String(quota.value || 0),
      })),
      fields: item.fields.map((field) => ({
        key: field.key.trim(),
        action: field.action,
        mode: field.mode,
      })),
    })).filter((item) => item.moduleCode),
    salesScope: editor.terms.salesScope.map((scope) => scope.trim()).filter(Boolean),
    validityMode: editor.terms.validityMode,
    validityDays: editor.terms.validityMode === 'fixed_days' ? Number(editor.terms.validityDays || 0) : 0,
    priceRef: editor.terms.priceRef.trim(),
  }
}

async function saveEditor() {
  actionError.value = ''
  const planCode = editor.planCode.trim()
  if (!planCode || !editor.name.trim()) {
    actionError.value = '套餐代码和套餐名称不能为空。'
    return
  }
  const terms = normalizedEditorTerms()
  if (!terms.salesScope.length) {
    actionError.value = '至少声明一个 sales_scope；需要全范围时请显式填写 *。'
    return
  }
  if (terms.validityMode === 'fixed_days' && (!Number.isInteger(terms.validityDays) || terms.validityDays < 1 || terms.validityDays > 36500)) {
    actionError.value = '固定有效期必须是 1～36500 天。'
    return
  }

  actionPending.value = true
  try {
    let saved: PlanVersionDTO
    if (editorMode.value === 'create') {
      saved = await createPlanDraft({
        requestId: commercialRequestId('ce13-plan-create'),
        planCode,
        name: editor.name.trim(),
        terms,
        reason: editor.reason.trim() || '平台控制台创建套餐首稿',
      })
    } else {
      const selected = selectedVersion.value
      if (!selected) return
      saved = await updatePlanDraft(planCode, selected.version, {
        requestId: commercialRequestId('ce13-plan-update'),
        expectedRevision: selected.revision,
        name: editor.name.trim(),
        terms,
        reason: editor.reason.trim() || '平台控制台修改套餐草稿',
      })
    }
    editorOpen.value = false
    planCodeInput.value = saved.planCode || planCode
    await loadVersions(true)
    selectedVersionKey.value = versionKey(saved)
    actionMessage.value = editorMode.value === 'create' ? '套餐草稿已创建。' : '草稿已保存。'
  } catch (error) {
    await handleMutationError(error)
  } finally {
    actionPending.value = false
  }
}

async function cloneSelected() {
  const selected = selectedVersion.value
  if (!selected || selected.state === 'DRAFT') return
  await runMutation(async () => createPlanVersion(selected.planCode, {
    requestId: commercialRequestId('ce13-plan-clone'),
    fromVersion: selected.version,
    expectedPlanRevision: selected.planRevision,
    reason: '平台控制台基于历史版本创建新草稿',
  }), '已创建新的不可变版本草稿。')
}

async function publishSelected() {
  const selected = selectedVersion.value
  if (!selected || selected.state !== 'DRAFT') return
  if (!window.confirm(`确认发布 ${selected.planCode} v${selected.version}？发布后内容不可覆盖。`)) return
  await runMutation(async () => publishPlanVersion(selected.planCode, selected.version, {
    requestId: commercialRequestId('ce13-plan-publish'),
    expectedRevision: selected.revision,
    reason: '平台控制台发布套餐版本',
  }), '套餐版本已发布；后续修订必须创建新版本。')
}

async function retireSelected() {
  const selected = selectedVersion.value
  if (!selected || selected.state !== 'PUBLISHED') return
  if (!window.confirm(`确认停售 ${selected.planCode} v${selected.version}？历史引用仍会保留。`)) return
  await runMutation(async () => retirePlanVersion(selected.planCode, selected.version, {
    requestId: commercialRequestId('ce13-plan-retire'),
    expectedRevision: selected.revision,
    reason: '平台控制台停售套餐版本',
  }), '套餐版本已停售，历史内容与引用保持可读。')
}

async function runMutation(operation: () => Promise<PlanVersionDTO>, success: string) {
  actionPending.value = true
  actionMessage.value = ''
  actionError.value = ''
  try {
    const result = await operation()
    await loadVersions(true)
    selectedVersionKey.value = versionKey(result)
    actionMessage.value = success
  } catch (error) {
    await handleMutationError(error)
  } finally {
    actionPending.value = false
  }
}

async function handleMutationError(error: unknown) {
  if (error instanceof CommercialApiError && error.code === 'conflict') {
    actionError.value = '版本已被其他操作修改，已重新读取最新状态；请核对后再提交。'
    await loadVersions(true)
    return
  }
  if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
    actionError.value = `当前可信平台会话无权执行该操作：${error.message}`
    return
  }
  actionError.value = error instanceof Error ? error.message : '套餐操作失败'
}

function openEligibility() {
  if (!selectedVersion.value) return
  eligibilityScope.value = ''
  eligibilityResult.value = null
  eligibilityOpen.value = true
}

async function runEligibility() {
  const selected = selectedVersion.value
  const scope = eligibilityScope.value.trim()
  if (!selected || !scope) return
  actionPending.value = true
  actionError.value = ''
  try {
    const result = await checkPlanEligibility(selected.planCode, selected.version, scope)
    eligibilityResult.value = { eligible: Boolean(result.eligible), reason: result.reason || '' }
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : '适用资格检查失败'
  } finally {
    actionPending.value = false
  }
}

function addSalesScope() {
  editor.terms.salesScope.push('')
}

function removeSalesScope(index: number) {
  editor.terms.salesScope.splice(index, 1)
}

onMounted(loadModules)
</script>

<template>
  <div class="page-stack" data-testid="ce13-plan-management">
    <PageHeading
      title="平台商业管理"
      description="按真实 plan_code 管理套餐草稿、不可变版本、发布停售与适用资格；不从租户订阅或本地缓存反推套餐目录"
    />

    <section class="commercial-tabs" aria-label="平台商业管理导航">
      <RouterLink class="commercial-tab" to="/platform/commercial/modules">模块目录</RouterLink>
      <RouterLink class="commercial-tab active" to="/platform/commercial/plans">套餐版本</RouterLink>
      <span class="commercial-tab disabled" aria-disabled="true">租户权益 · 后续切片</span>
    </section>

    <section class="card workspace-card">
      <div class="workspace-title">
        <div>
          <h2>套餐代码工作台</h2>
          <p>CE-07 当前仅支持按 plan_code 查询版本；全量套餐目录接口缺口已记录为 Issue #62，本页不会用前端 seed 补齐。</p>
        </div>
        <button class="btn primary" type="button" @click="openCreate">新建套餐</button>
      </div>
      <form class="lookup-row" @submit.prevent="loadVersions(true)">
        <label for="plan-code">套餐代码</label>
        <input id="plan-code" v-model="planCodeInput" class="input code-input" autocomplete="off" placeholder="例如 office-pro" />
        <button class="btn" type="submit" :disabled="loadState === 'loading'">读取版本</button>
      </form>
      <p class="scope-note">服务端事实边界：创建/查询/编辑/发布/停售均调用真实 commercial.plan.* API；浏览器只携带 HttpOnly 会话 Cookie。</p>
    </section>

    <section v-if="actionMessage" class="notice success" role="status">{{ actionMessage }}</section>
    <section v-if="actionError" class="notice danger" role="alert">{{ actionError }}</section>

    <section v-if="loadState === 'idle'" class="card state-card">
      <strong>输入套餐代码或新建套餐</strong>
      <p>不会自动展示本地保存过的套餐代码。</p>
    </section>
    <section v-else-if="loadState === 'loading'" class="card state-card" aria-live="polite">
      <strong>正在读取 {{ activePlanCode }} 的真实版本记录</strong>
      <p>请求 /v1/platform/plans/{plan_code}/versions。</p>
    </section>
    <section v-else-if="loadState === 'blocked'" class="card state-card warning" role="alert">
      <strong>当前会话无套餐管理读取权限</strong>
      <p>{{ errorMessage }}</p>
      <button class="btn" type="button" @click="loadVersions(true)">重新检查</button>
    </section>
    <section v-else-if="loadState === 'error'" class="card state-card danger" role="alert">
      <strong>套餐版本读取失败</strong>
      <p>{{ errorMessage }}</p>
      <button class="btn" type="button" @click="loadVersions(true)">重试</button>
    </section>

    <template v-else>
      <section class="metric-grid">
        <article class="card metric"><span>已读取版本</span><strong>{{ summary.count }}</strong><small>{{ activePlanCode }}</small></article>
        <article class="card metric"><span>草稿</span><strong>{{ summary.drafts }}</strong><small>DRAFT</small></article>
        <article class="card metric"><span>已发布</span><strong>{{ summary.published }}</strong><small>PUBLISHED</small></article>
        <article class="card metric"><span>已停售</span><strong>{{ summary.retired }}</strong><small>RETIRED</small></article>
      </section>

      <section v-if="loadState === 'empty'" class="card state-card">
        <strong>没有找到 {{ activePlanCode }} 的版本记录</strong>
        <p>若这是新套餐，可点击“新建套餐”；页面不会将 404/空结果替换为演示数据。</p>
      </section>

      <div v-else class="plan-layout">
        <section class="card versions-card">
          <div class="section-header">
            <div><h2>版本历史</h2><p>发布版本内容不可覆盖；修订请克隆为新草稿。</p></div>
            <button class="btn" type="button" @click="loadVersions(true)">刷新</button>
          </div>
          <div class="table-scroll">
            <table class="data-table">
              <thead><tr><th>版本</th><th>状态</th><th>名称</th><th>销售范围</th><th>修订</th><th>发布时间</th></tr></thead>
              <tbody>
                <tr
                  v-for="version in displayedVersions"
                  :key="versionKey(version)"
                  :class="{ selected: versionKey(version) === selectedVersionKey }"
                  tabindex="0"
                  @click="selectedVersionKey = versionKey(version)"
                  @keydown.enter="selectedVersionKey = versionKey(version)"
                >
                  <td class="numeric">v{{ version.version }}</td>
                  <td><StatusBadge :text="statusLabel(version.state)" :tone="statusTone(version.state)" /></td>
                  <td><strong>{{ version.name || '—' }}</strong></td>
                  <td>{{ version.terms?.salesScope?.join('、') || '—' }}</td>
                  <td class="numeric">{{ version.revision }}</td>
                  <td>{{ formatTime(version.publishedAt) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="nextAfterVersion" class="load-more"><button class="btn" type="button" @click="loadVersions(false)">加载更多</button></div>
        </section>

        <section v-if="selectedVersion" class="card detail-card">
          <div class="section-header detail-heading">
            <div>
              <div class="detail-title-row"><h2>{{ selectedVersion.name || selectedVersion.planCode }}</h2><StatusBadge :text="statusLabel(selectedVersion.state)" :tone="statusTone(selectedVersion.state)" /></div>
              <p>{{ selectedVersion.planCode }} · v{{ selectedVersion.version }} · plan revision {{ selectedVersion.planRevision }}</p>
            </div>
            <div class="action-row">
              <button v-if="selectedVersion.state === 'DRAFT'" class="btn" type="button" @click="openEdit">编辑草稿</button>
              <button v-if="selectedVersion.state === 'DRAFT'" class="btn primary" type="button" :disabled="actionPending" @click="publishSelected">发布</button>
              <button v-if="selectedVersion.state === 'PUBLISHED'" class="btn" type="button" :disabled="actionPending" @click="retireSelected">停售</button>
              <button v-if="selectedVersion.state !== 'DRAFT'" class="btn" type="button" :disabled="actionPending" @click="cloneSelected">创建新版本</button>
              <button class="btn" type="button" @click="openEligibility">资格预检</button>
            </div>
          </div>

          <dl class="fact-grid">
            <div><dt>有效期</dt><dd>{{ validityLabel(selectedVersion.terms) }}</dd></div>
            <div><dt>价格引用</dt><dd>{{ selectedVersion.terms?.priceRef || '—' }}</dd></div>
            <div><dt>内容 SHA256</dt><dd class="mono break-all">{{ selectedVersion.contentSha256 || '草稿未固定' }}</dd></div>
            <div><dt>创建时间</dt><dd>{{ formatTime(selectedVersion.createdAt) }}</dd></div>
            <div><dt>发布/停售</dt><dd>{{ formatTime(selectedVersion.retiredAt || selectedVersion.publishedAt) }}</dd></div>
            <div><dt>最后原因</dt><dd>{{ selectedVersion.reason || '—' }}</dd></div>
          </dl>

          <div class="terms-section">
            <h3>销售范围</h3>
            <div class="chip-row"><span v-for="scope in selectedVersion.terms?.salesScope ?? []" :key="scope" class="chip">{{ scope }}</span><span v-if="!selectedVersion.terms?.salesScope?.length">—</span></div>
          </div>

          <div class="terms-section">
            <h3>能力矩阵</h3>
            <article v-for="module in selectedVersion.terms?.modules ?? []" :key="module.moduleCode" class="module-term">
              <div class="module-term-title"><strong>{{ selectedModule(module.moduleCode)?.name || module.moduleCode }}</strong><span class="mono">{{ module.moduleCode }}</span></div>
              <div class="term-columns">
                <div><small>能力</small><div class="chip-row"><span v-for="capability in module.capabilityCodes" :key="capability" class="chip">{{ capability }}</span><span v-if="!module.capabilityCodes?.length">—</span></div></div>
                <div><small>额度</small><ul><li v-for="quota in module.quotas" :key="quota.key"><span class="mono">{{ quota.key }}</span> = {{ quota.unlimited ? 'unlimited' : quota.value }}</li><li v-if="!module.quotas?.length">—</li></ul></div>
                <div><small>字段策略</small><ul><li v-for="field in module.fields" :key="`${field.key}:${field.action}`"><span class="mono">{{ field.key }}</span> · {{ field.action }} → {{ field.mode }}</li><li v-if="!module.fields?.length">—</li></ul></div>
              </div>
            </article>
            <p v-if="!selectedVersion.terms?.modules?.length" class="muted">当前版本没有模块条款。</p>
          </div>
        </section>
      </div>
    </template>

    <div v-if="editorOpen" class="modal-backdrop" role="presentation" @click.self="editorOpen = false">
      <section class="modal card" role="dialog" aria-modal="true" aria-labelledby="plan-editor-title">
        <div class="modal-header">
          <div><h2 id="plan-editor-title">{{ editorMode === 'create' ? '新建套餐首稿' : '编辑套餐草稿' }}</h2><p>发布后内容不可覆盖；服务端仍会执行 CE-07 完整校验。</p></div>
          <button class="btn" type="button" @click="editorOpen = false">关闭</button>
        </div>

        <div class="editor-grid">
          <label><span>套餐代码</span><input v-model="editor.planCode" class="input" :disabled="editorMode === 'edit'" autocomplete="off" /></label>
          <label><span>套餐名称</span><input v-model="editor.name" class="input" autocomplete="off" /></label>
          <label><span>有效期模式</span><select v-model="editor.terms.validityMode" class="input"><option value="unlimited">unlimited</option><option value="fixed_days">fixed_days</option></select></label>
          <label v-if="editor.terms.validityMode === 'fixed_days'"><span>有效天数</span><input v-model.number="editor.terms.validityDays" class="input" type="number" min="1" max="36500" /></label>
          <label><span>价格引用 price_ref</span><input v-model="editor.terms.priceRef" class="input" autocomplete="off" /></label>
          <label class="wide"><span>变更原因</span><input v-model="editor.reason" class="input" autocomplete="off" /></label>
        </div>

        <div class="editor-section">
          <div class="editor-section-head"><div><h3>销售范围 sales_scope</h3><p>使用 * 表示全范围；适用资格查询本身不能用 *。</p></div><button class="btn" type="button" @click="addSalesScope">添加范围</button></div>
          <div v-for="(_, index) in editor.terms.salesScope" :key="`scope-${index}`" class="inline-editor"><input v-model="editor.terms.salesScope[index]" class="input" placeholder="default" /><button class="btn danger-text" type="button" @click="removeSalesScope(index)">移除</button></div>
          <p v-if="!editor.terms.salesScope.length" class="muted">尚未声明销售范围。</p>
        </div>

        <div class="editor-section">
          <div class="editor-section-head"><div><h3>模块与权益</h3><p v-if="moduleLoadError" class="error-text">模块目录不可用：{{ moduleLoadError }}</p><p v-else>模块、能力、额度键与字段键均来自真实模块目录。</p></div><button class="btn" type="button" :disabled="!modules.length" @click="addPlanModule">添加模块</button></div>

          <article v-for="(item, moduleIndex) in editor.terms.modules" :key="`module-${moduleIndex}`" class="module-editor">
            <div class="module-editor-head">
              <label class="flex-1"><span>模块</span><select v-model="item.moduleCode" class="input" @change="onModuleChanged(item)"><option value="">请选择</option><option v-for="module in modules" :key="module.moduleCode" :value="module.moduleCode">{{ module.name || module.moduleCode }} · {{ module.moduleCode }}</option></select></label>
              <button class="btn danger-text" type="button" @click="removePlanModule(moduleIndex)">移除模块</button>
            </div>

            <div class="editor-subsection">
              <strong>能力</strong>
              <div class="checkbox-grid">
                <label v-for="capability in selectedModule(item.moduleCode)?.capabilityCodes ?? []" :key="capability" class="check-row"><input type="checkbox" :checked="item.capabilityCodes.includes(capability)" @change="toggleCapability(item, capability, $event)" />{{ capability }}</label>
                <span v-if="!selectedModule(item.moduleCode)?.capabilityCodes?.length" class="muted">当前模块没有可声明能力。</span>
              </div>
            </div>

            <div class="editor-subsection">
              <div class="subsection-head"><strong>额度</strong><button class="btn small" type="button" :disabled="!selectedModule(item.moduleCode)?.quotaSchemaKeys?.length" @click="addQuota(item)">添加额度</button></div>
              <div v-for="(quota, quotaIndex) in item.quotas" :key="`quota-${quotaIndex}`" class="quota-row">
                <select v-model="quota.key" class="input"><option value="">额度键</option><option v-for="key in selectedModule(item.moduleCode)?.quotaSchemaKeys ?? []" :key="key" :value="key">{{ key }}</option></select>
                <label class="check-row"><input v-model="quota.unlimited" type="checkbox" />unlimited</label>
                <input v-model="quota.value" class="input" type="number" min="0" :disabled="quota.unlimited" aria-label="额度值" />
                <button class="btn danger-text" type="button" @click="removeQuota(item, quotaIndex)">移除</button>
              </div>
            </div>

            <div class="editor-subsection">
              <div class="subsection-head"><strong>字段策略</strong><button class="btn small" type="button" :disabled="!selectedModule(item.moduleCode)?.fieldPolicySchemaKeys?.length" @click="addField(item)">添加字段</button></div>
              <div v-for="(field, fieldIndex) in item.fields" :key="`field-${fieldIndex}`" class="field-row">
                <select v-model="field.key" class="input"><option value="">字段键</option><option v-for="key in selectedModule(item.moduleCode)?.fieldPolicySchemaKeys ?? []" :key="key" :value="key">{{ key }}</option></select>
                <select v-model="field.action" class="input"><option value="read">read</option><option value="write">write</option><option value="export">export</option></select>
                <select v-model="field.mode" class="input"><option value="deny">deny</option><option value="masked">masked</option><option value="allow">allow</option></select>
                <button class="btn danger-text" type="button" @click="removeField(item, fieldIndex)">移除</button>
              </div>
            </div>
          </article>
          <p v-if="!editor.terms.modules.length" class="muted">尚未添加模块条款。</p>
        </div>

        <div v-if="actionError" class="notice danger" role="alert">{{ actionError }}</div>
        <div class="modal-footer"><button class="btn" type="button" @click="editorOpen = false">取消</button><button class="btn primary" type="button" :disabled="actionPending" @click="saveEditor">{{ actionPending ? '提交中…' : '提交到服务端' }}</button></div>
      </section>
    </div>

    <div v-if="eligibilityOpen" class="modal-backdrop" role="presentation" @click.self="eligibilityOpen = false">
      <section class="modal compact card" role="dialog" aria-modal="true" aria-labelledby="eligibility-title">
        <div class="modal-header"><div><h2 id="eligibility-title">适用资格预检</h2><p>{{ selectedVersion?.planCode }} v{{ selectedVersion?.version }}；该结果不是订阅回执或免检许可。</p></div><button class="btn" type="button" @click="eligibilityOpen = false">关闭</button></div>
        <label><span>sales_scope</span><input v-model="eligibilityScope" class="input" placeholder="例如 default（不能填 *）" /></label>
        <button class="btn primary full" type="button" :disabled="!eligibilityScope.trim() || eligibilityScope.trim() === '*' || actionPending" @click="runEligibility">执行真实资格检查</button>
        <div v-if="eligibilityResult" class="eligibility-result" :class="eligibilityResult.eligible ? 'allowed' : 'denied'">
          <strong>{{ eligibilityResult.eligible ? '可适用' : '不可适用' }}</strong><p>{{ eligibilityResult.reason || '服务端未提供额外说明' }}</p>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.commercial-tabs { display: flex; gap: 4px; border-bottom: 1px solid var(--color-border); }
.commercial-tab { padding: 10px 14px; font-size: 13px; color: var(--color-text-secondary); text-decoration: none; border-bottom: 2px solid transparent; }
.commercial-tab.active { color: var(--color-primary); border-bottom-color: var(--color-primary); font-weight: 600; }
.commercial-tab.disabled { color: var(--color-text-muted); cursor: not-allowed; }
.workspace-card { padding: 20px; }
.workspace-title, .section-header, .editor-section-head, .subsection-head, .modal-header, .modal-footer, .module-editor-head { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.workspace-title h2, .section-header h2, .modal-header h2 { margin: 0; font-size: 16px; }
.workspace-title p, .section-header p, .modal-header p, .editor-section-head p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.lookup-row { display: grid; grid-template-columns: auto minmax(220px, 420px) auto; gap: 10px; align-items: center; margin-top: 18px; }
.lookup-row label, label > span { font-size: 12px; color: var(--color-text-secondary); }
.input { width: 100%; min-height: 36px; border: 1px solid var(--color-border); border-radius: 7px; padding: 7px 10px; background: var(--color-surface); color: var(--color-text-primary); font: inherit; }
.input:focus { outline: 2px solid var(--color-primary-soft); border-color: var(--color-primary); }
.scope-note { margin: 10px 0 0; font-size: 11px; color: var(--color-text-muted); }
.notice { padding: 10px 13px; border: 1px solid var(--color-border); border-radius: 8px; font-size: 13px; }
.notice.success { border-color: var(--color-success, #4b9a68); }
.notice.danger { border-color: var(--color-danger, #d14343); }
.state-card { min-height: 104px; padding: 20px; display: flex; flex-direction: column; align-items: flex-start; justify-content: center; gap: 6px; }
.state-card p { margin: 0; color: var(--color-text-secondary); font-size: 13px; }
.state-card.warning { border-color: var(--color-warning, #d9a441); }
.state-card.danger { border-color: var(--color-danger, #d14343); }
.metric-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric { padding: 16px; }
.metric span, .metric small { display: block; color: var(--color-text-muted); font-size: 12px; }
.metric strong { display: block; margin: 6px 0 2px; font-size: 23px; }
.plan-layout { display: grid; grid-template-columns: minmax(520px, 1.1fr) minmax(420px, .9fr); gap: 14px; align-items: start; }
.versions-card, .detail-card { overflow: hidden; }
.section-header { padding: 18px 20px; border-bottom: 1px solid var(--color-border); }
.data-table tbody tr { cursor: pointer; }
.data-table tbody tr.selected { background: var(--color-primary-soft); }
.data-table tbody tr:focus { outline: 2px solid var(--color-primary); outline-offset: -2px; }
.load-more { padding: 12px; text-align: center; border-top: 1px solid var(--color-border); }
.detail-heading { align-items: flex-start; flex-wrap: wrap; }
.detail-title-row { display: flex; align-items: center; gap: 10px; }
.detail-title-row h2 { margin: 0; }
.action-row { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
.fact-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); margin: 0; padding: 16px 20px; gap: 14px 20px; border-bottom: 1px solid var(--color-border); }
.fact-grid div { min-width: 0; }
.fact-grid dt { font-size: 11px; color: var(--color-text-muted); }
.fact-grid dd { margin: 4px 0 0; font-size: 13px; }
.terms-section { padding: 16px 20px; border-bottom: 1px solid var(--color-border); }
.terms-section:last-child { border-bottom: 0; }
.terms-section h3, .editor-section h3 { margin: 0 0 10px; font-size: 13px; }
.chip-row { display: flex; flex-wrap: wrap; gap: 6px; }
.chip { display: inline-flex; padding: 3px 7px; border-radius: 999px; background: var(--color-primary-soft); font-size: 11px; }
.module-term { padding: 12px 0; border-top: 1px solid var(--color-border); }
.module-term:first-of-type { border-top: 0; padding-top: 0; }
.module-term-title { display: flex; justify-content: space-between; gap: 10px; margin-bottom: 10px; }
.term-columns { display: grid; grid-template-columns: 1fr 1fr 1.2fr; gap: 12px; font-size: 12px; }
.term-columns small { color: var(--color-text-muted); }
.term-columns ul { margin: 6px 0 0; padding-left: 16px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.break-all { overflow-wrap: anywhere; }
.muted { color: var(--color-text-muted); font-size: 12px; }
.error-text { color: var(--color-danger, #d14343) !important; }
.modal-backdrop { position: fixed; inset: 0; z-index: 80; background: rgb(17 24 39 / 42%); display: grid; place-items: center; padding: 24px; }
.modal { width: min(1080px, 96vw); max-height: 92vh; overflow: auto; padding: 20px; }
.modal.compact { width: min(560px, 96vw); }
.modal-header { align-items: flex-start; padding-bottom: 14px; border-bottom: 1px solid var(--color-border); }
.editor-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 16px 0; }
.editor-grid label, .modal.compact > label { display: grid; gap: 6px; }
.editor-grid .wide { grid-column: 1 / -1; }
.editor-section { padding: 16px 0; border-top: 1px solid var(--color-border); }
.editor-section-head { align-items: flex-start; margin-bottom: 10px; }
.inline-editor { display: grid; grid-template-columns: 1fr auto; gap: 8px; margin-bottom: 8px; }
.module-editor { border: 1px solid var(--color-border); border-radius: 9px; padding: 14px; margin-top: 10px; }
.module-editor-head { align-items: end; }
.flex-1 { flex: 1; }
.editor-subsection { padding: 12px 0 0; }
.checkbox-grid { display: flex; flex-wrap: wrap; gap: 8px 14px; margin-top: 8px; }
.check-row { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--color-text-secondary); }
.quota-row { display: grid; grid-template-columns: minmax(160px, 1fr) auto 120px auto; gap: 8px; align-items: center; margin-top: 8px; }
.field-row { display: grid; grid-template-columns: minmax(160px, 1fr) 110px 120px auto; gap: 8px; align-items: center; margin-top: 8px; }
.btn.small { min-height: 30px; padding: 4px 8px; font-size: 12px; }
.btn.primary { background: var(--color-primary); border-color: var(--color-primary); color: white; }
.btn.danger-text { color: var(--color-danger, #d14343); }
.modal-footer { justify-content: flex-end; padding-top: 16px; border-top: 1px solid var(--color-border); }
.full { width: 100%; margin-top: 12px; }
.eligibility-result { margin-top: 14px; padding: 14px; border-radius: 8px; border: 1px solid var(--color-border); }
.eligibility-result.allowed { border-color: var(--color-success, #4b9a68); }
.eligibility-result.denied { border-color: var(--color-danger, #d14343); }
.eligibility-result p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
@media (max-width: 1100px) {
  .plan-layout { grid-template-columns: 1fr; }
}
@media (max-width: 760px) {
  .commercial-tabs { overflow-x: auto; }
  .commercial-tab { white-space: nowrap; }
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .lookup-row, .editor-grid, .fact-grid, .term-columns { grid-template-columns: 1fr; }
  .editor-grid .wide { grid-column: auto; }
  .quota-row, .field-row { grid-template-columns: 1fr; }
  .workspace-title, .section-header, .editor-section-head, .module-editor-head { align-items: flex-start; flex-wrap: wrap; }
  .modal-backdrop { padding: 8px; }
}
</style>
