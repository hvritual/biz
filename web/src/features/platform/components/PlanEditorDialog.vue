<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import type { ModuleDTO, PlanModule, PlanTerms, PlanVersionDTO } from '@/services/commercial/platformCommercial'

const props = defineProps<{
  open: boolean
  mode: 'create' | 'edit'
  modules: ModuleDTO[]
  initialVersion?: PlanVersionDTO
  defaultPlanCode?: string
  pending: boolean
  serverError?: string
  moduleError?: string
}>()

const emit = defineEmits<{
  close: []
  submit: [payload: { planCode: string; name: string; reason: string; terms: PlanTerms }]
}>()

const localError = ref('')
const editor = reactive({ planCode: '', name: '', reason: '', terms: emptyTerms() })

function emptyTerms(): PlanTerms {
  return { modules: [], salesScope: [], validityMode: 'unlimited', validityDays: 0, priceRef: '' }
}

function copyTerms(terms?: Partial<PlanTerms>): PlanTerms {
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

function reset() {
  localError.value = ''
  if (props.mode === 'edit' && props.initialVersion) {
    editor.planCode = props.initialVersion.planCode
    editor.name = props.initialVersion.name
    editor.reason = '平台控制台修改套餐草稿'
    editor.terms = copyTerms(props.initialVersion.terms)
    return
  }
  editor.planCode = props.defaultPlanCode?.trim() ?? ''
  editor.name = ''
  editor.reason = '平台控制台创建套餐首稿'
  editor.terms = emptyTerms()
}

watch(() => props.open, (open) => {
  if (open) reset()
})

function selectedModule(code: string) {
  return props.modules.find((item) => item.moduleCode === code)
}

function addSalesScope() { editor.terms.salesScope.push('') }
function removeSalesScope(index: number) { editor.terms.salesScope.splice(index, 1) }
function addModule() { editor.terms.modules.push({ moduleCode: '', capabilityCodes: [], quotas: [], fields: [] }) }
function removeModule(index: number) { editor.terms.modules.splice(index, 1) }

function moduleChanged(item: PlanModule) {
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
  const key = catalog?.quotaSchemaKeys.find((candidate) => !item.quotas.some((quota) => quota.key === candidate)) ?? ''
  item.quotas.push({ key, unlimited: false, value: 0 })
}

function addField(item: PlanModule) {
  const catalog = selectedModule(item.moduleCode)
  const key = catalog?.fieldPolicySchemaKeys.find((candidate) => !item.fields.some((field) => field.key === candidate)) ?? ''
  item.fields.push({ key, action: 'read', mode: 'deny' })
}

function normalizedTerms(): PlanTerms {
  return {
    modules: editor.terms.modules.map((item) => ({
      moduleCode: item.moduleCode.trim(),
      capabilityCodes: item.capabilityCodes.filter(Boolean),
      quotas: item.quotas.map((quota) => ({
        key: quota.key.trim(),
        unlimited: Boolean(quota.unlimited),
        value: quota.unlimited ? 0 : String(quota.value || 0),
      })),
      fields: item.fields.map((field) => ({ key: field.key.trim(), action: field.action, mode: field.mode })),
    })).filter((item) => item.moduleCode),
    salesScope: editor.terms.salesScope.map((scope) => scope.trim()).filter(Boolean),
    validityMode: editor.terms.validityMode,
    validityDays: editor.terms.validityMode === 'fixed_days' ? Number(editor.terms.validityDays || 0) : 0,
    priceRef: editor.terms.priceRef.trim(),
  }
}

function submit() {
  localError.value = ''
  const planCode = editor.planCode.trim()
  if (!planCode || !editor.name.trim()) {
    localError.value = '套餐代码和套餐名称不能为空。'
    return
  }
  const terms = normalizedTerms()
  if (!terms.salesScope.length) {
    localError.value = '至少声明一个 sales_scope；需要全范围时请显式填写 *。'
    return
  }
  if (terms.validityMode === 'fixed_days' && (!Number.isInteger(terms.validityDays) || terms.validityDays < 1 || terms.validityDays > 36500)) {
    localError.value = '固定有效期必须是 1～36500 天。'
    return
  }
  emit('submit', {
    planCode,
    name: editor.name.trim(),
    reason: editor.reason.trim(),
    terms,
  })
}
</script>

<template>
  <div v-if="open" class="plan-editor-backdrop" role="presentation" @click.self="emit('close')">
    <section class="plan-editor card" role="dialog" aria-modal="true" aria-labelledby="plan-editor-title">
      <header class="plan-editor-header">
        <div>
          <h2 id="plan-editor-title">{{ mode === 'create' ? '新建套餐首稿' : '编辑套餐草稿' }}</h2>
          <p>发布后内容不可覆盖；服务端仍会执行 CE-07 完整校验。</p>
        </div>
        <button class="btn" type="button" @click="emit('close')">关闭</button>
      </header>

      <div class="plan-editor-grid">
        <label><span>套餐代码</span><input v-model="editor.planCode" class="input" :disabled="mode === 'edit'" autocomplete="off" /></label>
        <label><span>套餐名称</span><input v-model="editor.name" class="input" autocomplete="off" /></label>
        <label><span>有效期模式</span><select v-model="editor.terms.validityMode" class="input"><option value="unlimited">unlimited</option><option value="fixed_days">fixed_days</option></select></label>
        <label v-if="editor.terms.validityMode === 'fixed_days'"><span>有效天数</span><input v-model.number="editor.terms.validityDays" class="input" type="number" min="1" max="36500" /></label>
        <label><span>价格引用 price_ref</span><input v-model="editor.terms.priceRef" class="input" autocomplete="off" /></label>
        <label class="wide"><span>变更原因</span><input v-model="editor.reason" class="input" autocomplete="off" /></label>
      </div>

      <section class="editor-section">
        <div class="section-head">
          <div><h3>销售范围 sales_scope</h3><p>使用 * 表示全范围；适用资格查询本身不能用 *。</p></div>
          <button class="btn" type="button" @click="addSalesScope">添加范围</button>
        </div>
        <div v-for="(_, index) in editor.terms.salesScope" :key="`scope-${index}`" class="inline-row">
          <input v-model="editor.terms.salesScope[index]" class="input" placeholder="default" />
          <button class="btn danger-text" type="button" @click="removeSalesScope(index)">移除</button>
        </div>
        <p v-if="!editor.terms.salesScope.length" class="muted">尚未声明销售范围。</p>
      </section>

      <section class="editor-section">
        <div class="section-head">
          <div><h3>模块与权益</h3><p v-if="moduleError" class="error-text">模块目录不可用：{{ moduleError }}</p><p v-else>模块、能力、额度键与字段键均来自真实模块目录。</p></div>
          <button class="btn" type="button" :disabled="!modules.length" @click="addModule">添加模块</button>
        </div>

        <article v-for="(item, moduleIndex) in editor.terms.modules" :key="`module-${moduleIndex}`" class="module-editor">
          <div class="module-head">
            <label class="grow"><span>模块</span><select v-model="item.moduleCode" class="input" @change="moduleChanged(item)"><option value="">请选择</option><option v-for="module in modules" :key="module.moduleCode" :value="module.moduleCode">{{ module.name || module.moduleCode }} · {{ module.moduleCode }}</option></select></label>
            <button class="btn danger-text" type="button" @click="removeModule(moduleIndex)">移除模块</button>
          </div>

          <div class="subsection">
            <strong>能力</strong>
            <div class="checkbox-grid">
              <label v-for="capability in selectedModule(item.moduleCode)?.capabilityCodes ?? []" :key="capability" class="check-row"><input type="checkbox" :checked="item.capabilityCodes.includes(capability)" @change="toggleCapability(item, capability, $event)" />{{ capability }}</label>
              <span v-if="!selectedModule(item.moduleCode)?.capabilityCodes?.length" class="muted">当前模块没有可声明能力。</span>
            </div>
          </div>

          <div class="subsection">
            <div class="subsection-head"><strong>额度</strong><button class="btn small" type="button" :disabled="!selectedModule(item.moduleCode)?.quotaSchemaKeys?.length" @click="addQuota(item)">添加额度</button></div>
            <div v-for="(quota, quotaIndex) in item.quotas" :key="`quota-${quotaIndex}`" class="quota-row">
              <select v-model="quota.key" class="input"><option value="">额度键</option><option v-for="key in selectedModule(item.moduleCode)?.quotaSchemaKeys ?? []" :key="key" :value="key">{{ key }}</option></select>
              <label class="check-row"><input v-model="quota.unlimited" type="checkbox" />unlimited</label>
              <input v-model="quota.value" class="input" type="number" min="0" :disabled="quota.unlimited" aria-label="额度值" />
              <button class="btn danger-text" type="button" @click="item.quotas.splice(quotaIndex, 1)">移除</button>
            </div>
          </div>

          <div class="subsection">
            <div class="subsection-head"><strong>字段策略</strong><button class="btn small" type="button" :disabled="!selectedModule(item.moduleCode)?.fieldPolicySchemaKeys?.length" @click="addField(item)">添加字段</button></div>
            <div v-for="(field, fieldIndex) in item.fields" :key="`field-${fieldIndex}`" class="field-row">
              <select v-model="field.key" class="input"><option value="">字段键</option><option v-for="key in selectedModule(item.moduleCode)?.fieldPolicySchemaKeys ?? []" :key="key" :value="key">{{ key }}</option></select>
              <select v-model="field.action" class="input"><option value="read">read</option><option value="write">write</option><option value="export">export</option></select>
              <select v-model="field.mode" class="input"><option value="deny">deny</option><option value="masked">masked</option><option value="allow">allow</option></select>
              <button class="btn danger-text" type="button" @click="item.fields.splice(fieldIndex, 1)">移除</button>
            </div>
          </div>
        </article>
        <p v-if="!editor.terms.modules.length" class="muted">尚未添加模块条款。</p>
      </section>

      <div v-if="localError || serverError" class="notice danger" role="alert">{{ localError || serverError }}</div>
      <footer class="plan-editor-footer">
        <button class="btn" type="button" @click="emit('close')">取消</button>
        <button class="btn primary" type="button" :disabled="pending" @click="submit">{{ pending ? '提交中…' : '提交到服务端' }}</button>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.plan-editor-backdrop { position: fixed; inset: 0; z-index: 80; background: rgb(17 24 39 / 42%); display: grid; place-items: center; padding: 24px; }
.plan-editor { width: min(1080px, 96vw); max-height: 92vh; overflow: auto; padding: 20px; }
.plan-editor-header, .plan-editor-footer, .section-head, .subsection-head, .module-head { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.plan-editor-header { align-items: flex-start; padding-bottom: 14px; border-bottom: 1px solid var(--color-border); }
.plan-editor-header h2 { margin: 0; font-size: 16px; }
.plan-editor-header p, .section-head p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.plan-editor-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 16px 0; }
.plan-editor-grid label { display: grid; gap: 6px; }
.plan-editor-grid label > span, .module-head label > span { font-size: 12px; color: var(--color-text-secondary); }
.plan-editor-grid .wide { grid-column: 1 / -1; }
.input { width: 100%; min-height: 36px; border: 1px solid var(--color-border); border-radius: 7px; padding: 7px 10px; background: var(--color-surface); color: var(--color-text-primary); font: inherit; }
.input:focus { outline: 2px solid var(--color-primary-soft); border-color: var(--color-primary); }
.editor-section { padding: 16px 0; border-top: 1px solid var(--color-border); }
.section-head { align-items: flex-start; margin-bottom: 10px; }
.section-head h3 { margin: 0; font-size: 13px; }
.inline-row { display: grid; grid-template-columns: 1fr auto; gap: 8px; margin-bottom: 8px; }
.module-editor { border: 1px solid var(--color-border); border-radius: 9px; padding: 14px; margin-top: 10px; }
.module-head { align-items: end; }
.grow { flex: 1; }
.subsection { padding-top: 12px; }
.checkbox-grid { display: flex; flex-wrap: wrap; gap: 8px 14px; margin-top: 8px; }
.check-row { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--color-text-secondary); }
.quota-row { display: grid; grid-template-columns: minmax(160px, 1fr) auto 120px auto; gap: 8px; align-items: center; margin-top: 8px; }
.field-row { display: grid; grid-template-columns: minmax(160px, 1fr) 110px 120px auto; gap: 8px; align-items: center; margin-top: 8px; }
.muted { color: var(--color-text-muted); font-size: 12px; }
.error-text { color: var(--color-danger, #d14343) !important; }
.notice { padding: 10px 13px; border: 1px solid var(--color-danger, #d14343); border-radius: 8px; font-size: 13px; }
.btn.small { min-height: 30px; padding: 4px 8px; font-size: 12px; }
.btn.primary { background: var(--color-primary); border-color: var(--color-primary); color: white; }
.btn.danger-text { color: var(--color-danger, #d14343); }
.plan-editor-footer { justify-content: flex-end; padding-top: 16px; border-top: 1px solid var(--color-border); }
@media (max-width: 760px) {
  .plan-editor-backdrop { padding: 8px; }
  .plan-editor-grid, .quota-row, .field-row { grid-template-columns: 1fr; }
  .plan-editor-grid .wide { grid-column: auto; }
  .section-head, .module-head { align-items: flex-start; flex-wrap: wrap; }
}
</style>
