<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'

import { computed, onMounted, ref } from 'vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import {
  CommercialApiError,
  listPlatformModules,
  type ModuleDTO,
  type ModuleSalesStatus,
  type ModuleTechnicalStatus,
} from '@/services/commercial/platformCommercial'
import {
  setPlatformModuleSalesStatus,
  setPlatformModuleTechnicalStatus,
  updatePlatformModule,
} from '@/services/commercial/platformModules'

type LoadState = 'loading' | 'ready' | 'empty' | 'blocked' | 'error'

const modules = ref<ModuleDTO[]>([])
const loadState = ref<LoadState>('loading')
const errorMessage = ref('')
const actionMessage = ref('')
const actionError = ref('')
const actionPending = ref(false)
const detailOpen = ref(false)
const selectedCode = ref('')
const editName = ref('')
const editCategory = ref('')
const editSalesScope = ref('')
const technicalDraft = ref<ModuleTechnicalStatus>('MODULE_TECHNICAL_STATUS_NOT_READY')
const reason = ref('')
const keyword = ref('')
const technicalFilter = ref('')
const salesFilter = ref('')

const filteredModules = computed(() => {
  const query = keyword.value.trim().toLowerCase()
  return modules.value.filter((item) => {
    const matchesKeyword = !query || [item.moduleCode, item.name, item.category]
      .some((value) => String(value ?? '').toLowerCase().includes(query))
    const matchesTechnical = !technicalFilter.value || item.technicalStatus === technicalFilter.value
    const matchesSales = !salesFilter.value || item.salesStatus === salesFilter.value
    return matchesKeyword && matchesTechnical && matchesSales
  })
})

const selected = computed(() => modules.value.find((item) => item.moduleCode === selectedCode.value))
const summary = computed(() => ({
  total: modules.value.length,
  ready: modules.value.filter((item) => item.technicalStatus === 'MODULE_TECHNICAL_STATUS_READY').length,
  sellable: modules.value.filter((item) => item.salesStatus === 'MODULE_SALES_STATUS_SELLABLE').length,
}))

function technicalLabel(status: ModuleTechnicalStatus) {
  return {
    MODULE_TECHNICAL_STATUS_READY: '技术就绪',
    MODULE_TECHNICAL_STATUS_NOT_READY: '未就绪',
    MODULE_TECHNICAL_STATUS_DISABLED: '技术停用',
    MODULE_TECHNICAL_STATUS_UNSPECIFIED: '未声明',
  }[status]
}

function salesLabel(status: ModuleSalesStatus) {
  return {
    MODULE_SALES_STATUS_SELLABLE: '可销售',
    MODULE_SALES_STATUS_RETIRED: '已停售',
    MODULE_SALES_STATUS_UNSPECIFIED: '未声明',
  }[status]
}

function salesScopeValues() {
  return editSalesScope.value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function openDetail(item: ModuleDTO) {
  selectedCode.value = item.moduleCode
  editName.value = item.name
  editCategory.value = item.category
  editSalesScope.value = item.salesScope?.join(', ') ?? ''
  technicalDraft.value = item.technicalStatus
  reason.value = ''
  actionMessage.value = ''
  actionError.value = ''
  detailOpen.value = true
}

function replaceModule(updated: ModuleDTO) {
  modules.value = modules.value.map((item) => (item.moduleCode === updated.moduleCode ? updated : item))
  openDetail(updated)
  detailOpen.value = true
}

async function loadModules() {
  const previous = selectedCode.value
  loadState.value = 'loading'
  errorMessage.value = ''
  try {
    modules.value = await listPlatformModules()
    loadState.value = modules.value.length ? 'ready' : 'empty'
    if (previous && modules.value.some((item) => item.moduleCode === previous)) selectedCode.value = previous
  } catch (error) {
    if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
      loadState.value = 'blocked'
      errorMessage.value = error.message
      return
    }
    loadState.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : '无法读取模块目录'
  }
}

async function handleMutationError(error: unknown) {
  if (error instanceof CommercialApiError && error.code === 'conflict') {
    actionError.value = '模块版本已变化，已重新读取服务端最新状态；请核对后重新提交。'
    await loadModules()
    const fresh = selected.value
    if (fresh) openDetail(fresh)
    return
  }
  if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
    actionError.value = `当前平台会话无权执行该操作：${error.message}`
    return
  }
  actionError.value = error instanceof Error ? error.message : '模块操作失败'
}

async function saveMetadata() {
  const current = selected.value
  if (!current || actionPending.value) return
  if (!editName.value.trim() || !editCategory.value.trim()) {
    actionError.value = '模块名称和分类不能为空。'
    return
  }
  if (!reason.value.trim()) {
    actionError.value = '请填写本次配置变更原因。'
    return
  }
  actionPending.value = true
  actionMessage.value = ''
  actionError.value = ''
  try {
    const updated = await updatePlatformModule(current, {
      name: editName.value,
      category: editCategory.value,
      salesScope: salesScopeValues(),
      reason: reason.value,
    })
    replaceModule(updated)
    actionMessage.value = '模块基础配置已由服务端确认更新。'
  } catch (error) {
    await handleMutationError(error)
  } finally {
    actionPending.value = false
  }
}

async function applySalesStatus() {
  const current = selected.value
  if (!current || actionPending.value) return
  if (!reason.value.trim()) {
    actionError.value = '请填写销售状态变更原因。'
    return
  }
  const next: ModuleSalesStatus = current.salesStatus === 'MODULE_SALES_STATUS_SELLABLE'
    ? 'MODULE_SALES_STATUS_RETIRED'
    : 'MODULE_SALES_STATUS_SELLABLE'
  actionPending.value = true
  actionMessage.value = ''
  actionError.value = ''
  try {
    const updated = await setPlatformModuleSalesStatus(current, next, reason.value)
    replaceModule(updated)
    actionMessage.value = next === 'MODULE_SALES_STATUS_SELLABLE'
      ? '模块已恢复可销售状态。'
      : '模块已停售；技术状态未被自动修改。'
  } catch (error) {
    await handleMutationError(error)
  } finally {
    actionPending.value = false
  }
}

async function applyTechnicalStatus() {
  const current = selected.value
  if (!current || actionPending.value || technicalDraft.value === current.technicalStatus) return
  if (!reason.value.trim()) {
    actionError.value = '请填写技术状态变更原因。'
    return
  }
  actionPending.value = true
  actionMessage.value = ''
  actionError.value = ''
  try {
    const updated = await setPlatformModuleTechnicalStatus(current, technicalDraft.value, reason.value)
    replaceModule(updated)
    actionMessage.value = '技术状态已由服务端确认更新；销售状态保持独立。'
  } catch (error) {
    await handleMutationError(error)
  } finally {
    actionPending.value = false
  }
}

onMounted(loadModules)
</script>

<template>
  <div class="page-stack" data-testid="ce13-module-catalog" data-ui-template="ListPage">
    <div data-ui-region="page-heading">
      <PageHeading
        title="模块目录"
        description="治理平台可售模块、启用状态与能力边界，保持模块定义与销售配置一致"
      />
    </div>

    <section v-if="loadState === 'loading'" class="card state-card" aria-live="polite">
      <span class="state-icon"><AppIcon name="refresh" :size="20" /></span>
      <div><strong>正在读取真实模块目录</strong><p>请求 /v1/platform/modules，不加载本地 seed。</p></div>
    </section>

    <section v-else-if="loadState === 'blocked'" class="card state-card warning" role="alert">
      <span class="state-icon"><AppIcon name="shield" :size="20" /></span>
      <div class="flex-1">
        <strong>当前会话无平台商业访问权限</strong>
        <p>{{ errorMessage || '请使用已授权的平台 Web Session；浏览器不会降级使用平台 API Key。' }}</p>
      </div>
      <UiButton class="btn" type="button" @click="loadModules">重新检查</UiButton>
    </section>

    <section v-else-if="loadState === 'error'" class="card state-card danger" role="alert">
      <span class="state-icon"><AppIcon name="help" :size="20" /></span>
      <div class="flex-1"><strong>模块目录读取失败</strong><p>{{ errorMessage }}</p></div>
      <UiButton class="btn" type="button" @click="loadModules">重试</UiButton>
    </section>

    <template v-else>
      <section class="metric-grid" aria-label="模块目录摘要" data-ui-region="metrics">
        <article class="card metric"><span>模块总数</span><strong>{{ summary.total }}</strong><small>当前模块目录</small></article>
        <article class="card metric"><span>技术就绪</span><strong>{{ summary.ready }}</strong><small>READY</small></article>
        <article class="card metric"><span>可销售</span><strong>{{ summary.sellable }}</strong><small>SELLABLE</small></article>
      </section>

      <section class="card query-panel" data-ui-region="query" aria-label="模块目录查询">
        <div class="query-grid">
          <label class="field"><span>关键词</span><UiInput v-model.trim="keyword" class="input" placeholder="模块名称 / 代码 / 分类" /></label>
          <label class="field"><span>技术状态</span>
            <UiSelect v-model="technicalFilter" class="select">
              <UiOption value="">全部技术状态</UiOption>
              <UiOption value="MODULE_TECHNICAL_STATUS_READY">技术就绪</UiOption>
              <UiOption value="MODULE_TECHNICAL_STATUS_NOT_READY">未就绪</UiOption>
              <UiOption value="MODULE_TECHNICAL_STATUS_DISABLED">技术停用</UiOption>
            </UiSelect>
          </label>
          <label class="field"><span>销售状态</span>
            <UiSelect v-model="salesFilter" class="select">
              <UiOption value="">全部销售状态</UiOption>
              <UiOption value="MODULE_SALES_STATUS_SELLABLE">可销售</UiOption>
              <UiOption value="MODULE_SALES_STATUS_RETIRED">已停售</UiOption>
            </UiSelect>
          </label>
          <UiButton class="btn query-reset" type="button" @click="keyword = ''; technicalFilter = ''; salesFilter = ''">重置</UiButton>
        </div>
        <p class="query-summary">当前显示 {{ filteredModules.length }} / {{ modules.length }} 个模块；筛选只影响当前列表的展示结果。</p>
      </section>

      <section v-if="loadState === 'empty'" class="card state-card">
        <span class="state-icon"><AppIcon name="database" :size="20" /></span>
        <div><strong>模块目录为空</strong><p>当前没有平台模块记录。</p></div>
      </section>

      <section v-else class="card catalog-card" data-ui-region="data">
        <div class="catalog-header">
          <div>
            <h2>模块目录</h2>
            <p>启用状态与销售状态独立管理；模块能力、额度规则和依赖关系统一维护。</p>
          </div>
          <UiButton class="btn" type="button" @click="loadModules"><AppIcon name="refresh" :size="15" />刷新</UiButton>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>模块</th><th>分类</th><th>技术状态</th><th>销售状态</th><th>能力治理</th><th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!filteredModules.length">
                <td colspan="6" class="empty-row">没有符合当前筛选条件的模块。</td>
              </tr>
              <tr v-for="item in filteredModules" :key="item.moduleCode">
                <td><strong>{{ item.name || item.moduleCode }}</strong><small class="module-code">{{ item.moduleCode }} · v{{ item.version }}</small></td>
                <td>{{ item.category || '—' }}</td>
                <td><StatusBadge :text="technicalLabel(item.technicalStatus)" :tone="item.technicalStatus === 'MODULE_TECHNICAL_STATUS_READY' ? 'success' : 'warning'" /></td>
                <td><StatusBadge :text="salesLabel(item.salesStatus)" :tone="item.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? 'success' : 'neutral'" /></td>
                <td><strong class="governance-count">{{ item.capabilityCodes?.length ?? 0 }} 项能力</strong><small class="cell-note">{{ item.dependencies?.length ?? 0 }} 项依赖</small></td>
                <td><UiButton class="btn table-action" type="button" @click="openDetail(item)">查看详情</UiButton></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <UiDialog :open="detailOpen" :title="selected ? `模块详情 · ${selected.name || selected.moduleCode}` : '模块详情'" @close="detailOpen = false">
      <div v-if="selected" class="detail-stack">
        <div class="detail-summary">
          <div><span>模块代码</span><strong class="mono">{{ selected.moduleCode }}</strong></div>
          <div><span>当前版本</span><strong>{{ selected.version }}</strong></div>
          <div><span>技术状态</span><StatusBadge :text="technicalLabel(selected.technicalStatus)" /></div>
          <div><span>销售状态</span><StatusBadge :text="salesLabel(selected.salesStatus)" /></div>
        </div>

        <section class="detail-section">
          <div class="section-heading"><div><h3>基础配置</h3><p>仅维护商业元数据；技术能力代码仍由服务端 Registry 管理。</p></div></div>
          <div class="form-grid">
            <label class="field"><span>模块名称</span><UiInput v-model="editName" class="input" /></label>
            <label class="field"><span>分类</span><UiInput v-model="editCategory" class="input" /></label>
            <label class="field full"><span>销售范围</span><UiInput v-model="editSalesScope" class="input" placeholder="default, enterprise" /></label>
          </div>
        </section>

        <section class="detail-section">
          <div class="section-heading"><div><h3>技术与销售状态</h3><p>两类状态独立变更，不通过停售隐式关闭技术能力。</p></div></div>
          <div class="status-controls">
            <label class="field"><span>技术状态</span>
              <UiSelect v-model="technicalDraft" class="select">
                <UiOption value="MODULE_TECHNICAL_STATUS_NOT_READY">未就绪</UiOption>
                <UiOption value="MODULE_TECHNICAL_STATUS_READY">技术就绪</UiOption>
                <UiOption value="MODULE_TECHNICAL_STATUS_DISABLED">技术停用</UiOption>
              </UiSelect>
            </label>
            <div class="field"><span>销售状态</span><strong>{{ salesLabel(selected.salesStatus) }}</strong></div>
          </div>
        </section>

        <section class="detail-section facts-grid">
          <div><h3>能力代码</h3><p v-if="!selected.capabilityCodes?.length" class="muted">无</p><div v-else class="tag-list"><span v-for="item in selected.capabilityCodes" :key="item" class="fact-tag">{{ item }}</span></div></div>
          <div><h3>模块依赖</h3><p v-if="!selected.dependencies?.length" class="muted">无</p><div v-else class="tag-list"><span v-for="item in selected.dependencies" :key="item" class="fact-tag">{{ item }}</span></div></div>
          <div><h3>额度模板</h3><p v-if="!selected.quotaSchemaKeys?.length" class="muted">无</p><div v-else class="tag-list"><span v-for="item in selected.quotaSchemaKeys" :key="item" class="fact-tag">{{ item }}</span></div></div>
          <div><h3>字段策略</h3><p v-if="!selected.fieldPolicySchemaKeys?.length" class="muted">无</p><div v-else class="tag-list"><span v-for="item in selected.fieldPolicySchemaKeys" :key="item" class="fact-tag">{{ item }}</span></div></div>
        </section>

        <label class="field"><span>变更原因</span><UiTextarea v-model="reason" class="textarea" maxlength="500" placeholder="说明本次模块配置或状态调整原因" /></label>
        <div v-if="actionError" class="notice-box error" role="alert">{{ actionError }}</div>
        <div v-if="actionMessage" class="notice-box" role="status">{{ actionMessage }}</div>
      </div>
      <template #footer>
        <UiButton class="btn" type="button" @click="detailOpen = false">关闭</UiButton>
        <UiButton class="btn" type="button" :disabled="actionPending || !selected" @click="applyTechnicalStatus">应用技术状态</UiButton>
        <UiButton class="btn" type="button" :disabled="actionPending || !selected" @click="applySalesStatus">
          {{ selected?.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? '停售销售' : '恢复销售' }}
        </UiButton>
        <UiButton class="btn btn-primary" type="button" :disabled="actionPending || !selected" @click="saveMetadata">保存基础配置</UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.query-panel { padding: 18px 20px; }
.query-grid { display: grid; grid-template-columns: minmax(220px, 1.4fr) minmax(170px, .8fr) minmax(170px, .8fr) auto; gap: 12px; align-items: end; }
.query-grid .field { margin: 0; }
.query-reset { min-height: 36px; }
.query-summary { margin: 10px 0 0; color: var(--color-text-muted); font-size: 11px; }
.state-card {
  min-height: 112px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 22px;
}
.state-card p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.state-card.warning { border-color: var(--color-warning, var(--color-fixed-b79dd458)); }
.state-card.danger { border-color: var(--color-danger, var(--color-fixed-ecee5ee9)); }
.state-icon {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  flex: 0 0 auto;
}
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.metric { padding: 18px; }
.metric span, .metric small { display: block; color: var(--color-text-muted); font-size: 12px; }
.metric strong { display: block; margin: 7px 0 3px; font-size: 25px; }
.catalog-card { overflow: hidden; }
.catalog-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 20px; border-bottom: 1px solid var(--color-border); }
.catalog-header h2 { margin: 0; font-size: 16px; }
.catalog-header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.module-code { display: block; margin-top: 4px; color: var(--color-text-muted); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.governance-count { display: block; font-size: 12px; }
.cell-note { display: block; margin-top: 4px; color: var(--color-text-muted); font-size: 11px; }
.empty-row { padding: 28px 16px !important; text-align: center; color: var(--color-text-muted); }
.table-action { padding: 6px 10px; min-height: 32px; }
.flex-1 { flex: 1; }
.detail-stack { display: grid; gap: 18px; }
.detail-summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.detail-summary > div { padding: 12px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface-soft); }
.detail-summary span { display: block; margin-bottom: 6px; color: var(--color-text-muted); font-size: 11px; }
.detail-summary strong { font-size: 13px; }
.detail-section { border-top: 1px solid var(--color-border); padding-top: 16px; }
.section-heading { display: flex; justify-content: space-between; margin-bottom: 12px; }
.section-heading h3, .facts-grid h3 { font-size: 14px; margin: 0; }
.section-heading p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 11px; }
.form-grid, .status-controls { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.form-grid .full { grid-column: 1 / -1; }
.facts-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.tag-list { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.fact-tag { padding: 4px 7px; border-radius: 6px; background: var(--color-primary-soft); color: var(--color-primary); font-size: 11px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.notice-box.error { border-color: var(--color-danger); color: var(--color-danger); }
@media (max-width: 760px) {
  .metric-grid, .detail-summary, .form-grid, .status-controls, .facts-grid { grid-template-columns: 1fr; }
  .form-grid .full { grid-column: auto; }
  .query-grid { grid-template-columns: 1fr; }
  .state-card, .catalog-header { align-items: flex-start; flex-wrap: wrap; }
}
</style>
