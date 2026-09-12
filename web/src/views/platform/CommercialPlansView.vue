<script setup lang="ts">
import AuthorityPicker from '@/components/platform/AuthorityPicker.vue'
import { computed, onMounted, ref } from 'vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import PlanEditorDialog from '@/components/platform/PlanEditorDialog.vue'
import PlanVersionDetail from '@/components/platform/PlanVersionDetail.vue'
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
  type PlanTerms,
  type PlanVersionDTO,
} from '@/services/commercial/platformCommercial'

type LoadState = 'idle' | 'loading' | 'ready' | 'empty' | 'blocked' | 'error'
type EditorPayload = { planCode: string; name: string; reason: string; terms: PlanTerms }

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
const editorMode = ref<'create' | 'edit'>('create')
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
  actionError.value = ''
  editorMode.value = 'create'
  editorOpen.value = true
}

function openEdit() {
  if (!selectedVersion.value || selectedVersion.value.state !== 'DRAFT') return
  actionError.value = ''
  editorMode.value = 'edit'
  editorOpen.value = true
}

async function saveEditor(payload: EditorPayload) {
  actionPending.value = true
  actionError.value = ''
  try {
    let saved: PlanVersionDTO
    if (editorMode.value === 'create') {
      saved = await createPlanDraft({
        requestId: commercialRequestId('ce13-plan-create'),
        ...payload,
        reason: payload.reason || '平台控制台创建套餐首稿',
      })
    } else {
      const selected = selectedVersion.value
      if (!selected) return
      saved = await updatePlanDraft(payload.planCode, selected.version, {
        requestId: commercialRequestId('ce13-plan-update'),
        expectedRevision: selected.revision,
        name: payload.name,
        terms: payload.terms,
        reason: payload.reason || '平台控制台修改套餐草稿',
      })
    }
    editorOpen.value = false
    planCodeInput.value = saved.planCode || payload.planCode
    await loadVersions(true)
    selectedVersionKey.value = versionKey(saved)
    actionMessage.value = editorMode.value === 'create' ? '套餐草稿已创建。' : '草稿已保存。'
  } catch (error) {
    await handleMutationError(error)
  } finally {
    actionPending.value = false
  }
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

async function cloneSelected() {
  const selected = selectedVersion.value
  if (!selected || selected.state === 'DRAFT') return
  await runMutation(() => createPlanVersion(selected.planCode, {
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
  await runMutation(() => publishPlanVersion(selected.planCode, selected.version, {
    requestId: commercialRequestId('ce13-plan-publish'),
    expectedRevision: selected.revision,
    reason: '平台控制台发布套餐版本',
  }), '套餐版本已发布；后续修订必须创建新版本。')
}

async function retireSelected() {
  const selected = selectedVersion.value
  if (!selected || selected.state !== 'PUBLISHED') return
  if (!window.confirm(`确认停售 ${selected.planCode} v${selected.version}？历史引用仍会保留。`)) return
  await runMutation(() => retirePlanVersion(selected.planCode, selected.version, {
    requestId: commercialRequestId('ce13-plan-retire'),
    expectedRevision: selected.revision,
    reason: '平台控制台停售套餐版本',
  }), '套餐版本已停售，历史内容与引用保持可读。')
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
  if (!selected || !scope || scope === '*') return
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
      <RouterLink class="commercial-tab" to="/platform/commercial/tenant-entitlements">租户权益</RouterLink>
    </section>

    <AuthorityPicker kind="plans" @select="(id) => { planCodeInput = id; loadVersions(true) }" />
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
        <input id="plan-code" v-model="planCodeInput" class="input" autocomplete="off" placeholder="例如 office-pro" />
        <button class="btn" type="submit" :disabled="loadState === 'loading'">读取版本</button>
      </form>
      <p class="scope-note">创建/查询/编辑/发布/停售均调用真实 commercial.plan.* API；浏览器只携带 HttpOnly 会话 Cookie。</p>
    </section>

    <section v-if="actionMessage" class="notice success" role="status">{{ actionMessage }}</section>
    <section v-if="actionError && !editorOpen" class="notice danger" role="alert">{{ actionError }}</section>

    <section v-if="loadState === 'idle'" class="card state-card"><strong>输入套餐代码或新建套餐</strong><p>不会自动展示本地保存过的套餐代码。</p></section>
    <section v-else-if="loadState === 'loading'" class="card state-card" aria-live="polite"><strong>正在读取 {{ activePlanCode }} 的真实版本记录</strong><p>请求 /v1/platform/plans/{plan_code}/versions。</p></section>
    <section v-else-if="loadState === 'blocked'" class="card state-card warning" role="alert"><strong>当前会话无套餐管理读取权限</strong><p>{{ errorMessage }}</p><button class="btn" type="button" @click="loadVersions(true)">重新检查</button></section>
    <section v-else-if="loadState === 'error'" class="card state-card danger" role="alert"><strong>套餐版本读取失败</strong><p>{{ errorMessage }}</p><button class="btn" type="button" @click="loadVersions(true)">重试</button></section>

    <template v-else>
      <section class="metric-grid">
        <article class="card metric"><span>已读取版本</span><strong>{{ summary.count }}</strong><small>{{ activePlanCode }}</small></article>
        <article class="card metric"><span>草稿</span><strong>{{ summary.drafts }}</strong><small>DRAFT</small></article>
        <article class="card metric"><span>已发布</span><strong>{{ summary.published }}</strong><small>PUBLISHED</small></article>
        <article class="card metric"><span>已停售</span><strong>{{ summary.retired }}</strong><small>RETIRED</small></article>
      </section>

      <section v-if="loadState === 'empty'" class="card state-card"><strong>没有找到 {{ activePlanCode }} 的版本记录</strong><p>若这是新套餐，可点击“新建套餐”；页面不会将 404/空结果替换为演示数据。</p></section>

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

        <PlanVersionDetail
          v-if="selectedVersion"
          :version="selectedVersion"
          :modules="modules"
          :pending="actionPending"
          @edit="openEdit"
          @publish="publishSelected"
          @retire="retireSelected"
          @clone="cloneSelected"
          @eligibility="openEligibility"
        />
      </div>
    </template>

    <PlanEditorDialog
      :open="editorOpen"
      :mode="editorMode"
      :modules="modules"
      :initial-version="editorMode === 'edit' ? selectedVersion : undefined"
      :default-plan-code="activePlanCode || planCodeInput"
      :pending="actionPending"
      :server-error="actionError"
      :module-error="moduleLoadError"
      @close="editorOpen = false"
      @submit="saveEditor"
    />

    <div v-if="eligibilityOpen" class="eligibility-backdrop" role="presentation" @click.self="eligibilityOpen = false">
      <section class="eligibility-dialog card" role="dialog" aria-modal="true" aria-labelledby="eligibility-title">
        <header class="section-header">
          <div><h2 id="eligibility-title">适用资格预检</h2><p>{{ selectedVersion?.planCode }} v{{ selectedVersion?.version }}；结果不是订阅回执或免检许可。</p></div>
          <button class="btn" type="button" @click="eligibilityOpen = false">关闭</button>
        </header>
        <div class="eligibility-body">
          <label><span>sales_scope</span><input v-model="eligibilityScope" class="input" placeholder="例如 default（不能填 *）" /></label>
          <button class="btn primary full" type="button" :disabled="!eligibilityScope.trim() || eligibilityScope.trim() === '*' || actionPending" @click="runEligibility">执行真实资格检查</button>
          <div v-if="eligibilityResult" class="eligibility-result" :class="eligibilityResult.eligible ? 'allowed' : 'denied'"><strong>{{ eligibilityResult.eligible ? '可适用' : '不可适用' }}</strong><p>{{ eligibilityResult.reason || '服务端未提供额外说明' }}</p></div>
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
.workspace-title, .section-header { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.workspace-title h2, .section-header h2 { margin: 0; font-size: 16px; }
.workspace-title p, .section-header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.lookup-row { display: grid; grid-template-columns: auto minmax(220px, 420px) auto; gap: 10px; align-items: center; margin-top: 18px; }
.lookup-row label, .eligibility-body label > span { font-size: 12px; color: var(--color-text-secondary); }
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
.versions-card { overflow: hidden; }
.section-header { padding: 18px 20px; border-bottom: 1px solid var(--color-border); }
.data-table tbody tr { cursor: pointer; }
.data-table tbody tr.selected { background: var(--color-primary-soft); }
.data-table tbody tr:focus { outline: 2px solid var(--color-primary); outline-offset: -2px; }
.load-more { padding: 12px; text-align: center; border-top: 1px solid var(--color-border); }
.btn.primary { background: var(--color-primary); border-color: var(--color-primary); color: white; }
.eligibility-backdrop { position: fixed; inset: 0; z-index: 80; background: rgb(17 24 39 / 42%); display: grid; place-items: center; padding: 24px; }
.eligibility-dialog { width: min(560px, 96vw); overflow: hidden; }
.eligibility-body { padding: 18px 20px; }
.eligibility-body label { display: grid; gap: 6px; }
.full { width: 100%; margin-top: 12px; }
.eligibility-result { margin-top: 14px; padding: 14px; border-radius: 8px; border: 1px solid var(--color-border); }
.eligibility-result.allowed { border-color: var(--color-success, #4b9a68); }
.eligibility-result.denied { border-color: var(--color-danger, #d14343); }
.eligibility-result p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
@media (max-width: 1100px) { .plan-layout { grid-template-columns: 1fr; } }
@media (max-width: 760px) {
  .commercial-tabs { overflow-x: auto; }
  .commercial-tab { white-space: nowrap; }
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .lookup-row { grid-template-columns: 1fr; }
  .workspace-title, .section-header { align-items: flex-start; flex-wrap: wrap; }
  .eligibility-backdrop { padding: 8px; }
}
</style>
