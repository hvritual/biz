<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import type {
  CreateEntitlementOverrideInput,
  EntitlementEffect,
  EntitlementTarget,
  ModuleDTO,
} from '@/services/commercial/platformCommercial'

const props = defineProps<{
  open: boolean
  sourceVersion: string | number
  modules: ModuleDTO[]
  pending: boolean
  error: string
}>()

const emit = defineEmits<{
  close: []
  submit: [payload: CreateEntitlementOverrideInput]
}>()

const formError = ref('')
const form = reactive({
  moduleCode: '',
  target: 'ENTITLEMENT_TARGET_CAPABILITY' as EntitlementTarget,
  key: '',
  fieldAction: 'read',
  effect: 'ENTITLEMENT_EFFECT_GRANT' as EntitlementEffect,
  unlimited: false,
  value: '1',
  effectiveAt: '',
  expiresAt: '',
  reason: '',
})

const selectedModule = computed(() => props.modules.find((item) => item.moduleCode === form.moduleCode))
const availableKeys = computed(() => {
  if (!selectedModule.value) return []
  if (form.target === 'ENTITLEMENT_TARGET_CAPABILITY') return selectedModule.value.capabilityCodes ?? []
  if (form.target === 'ENTITLEMENT_TARGET_QUOTA') return selectedModule.value.quotaSchemaKeys ?? []
  if (form.target === 'ENTITLEMENT_TARGET_FIELD') return selectedModule.value.fieldPolicySchemaKeys ?? []
  return []
})

const effects = computed<Array<{ value: EntitlementEffect; label: string }>>(() => {
  if (form.target === 'ENTITLEMENT_TARGET_QUOTA') {
    return [
      { value: 'ENTITLEMENT_EFFECT_QUOTA_ADD', label: '额度追加' },
      { value: 'ENTITLEMENT_EFFECT_QUOTA_REPLACE', label: '额度替换' },
    ]
  }
  if (form.target === 'ENTITLEMENT_TARGET_FIELD') {
    return [
      { value: 'ENTITLEMENT_EFFECT_GRANT', label: '允许' },
      { value: 'ENTITLEMENT_EFFECT_DENY', label: '拒绝' },
      { value: 'ENTITLEMENT_EFFECT_SAFETY_DENY', label: '安全拒绝' },
      { value: 'ENTITLEMENT_EFFECT_SAFETY_MASK', label: '安全脱敏' },
    ]
  }
  return [
    { value: 'ENTITLEMENT_EFFECT_GRANT', label: '授权' },
    { value: 'ENTITLEMENT_EFFECT_DENY', label: '拒绝' },
    { value: 'ENTITLEMENT_EFFECT_SAFETY_DENY', label: '安全拒绝' },
  ]
})

function reset() {
  formError.value = ''
  form.moduleCode = props.modules[0]?.moduleCode ?? ''
  form.target = 'ENTITLEMENT_TARGET_CAPABILITY'
  form.key = ''
  form.fieldAction = 'read'
  form.effect = 'ENTITLEMENT_EFFECT_GRANT'
  form.unlimited = false
  form.value = '1'
  form.effectiveAt = ''
  form.expiresAt = ''
  form.reason = ''
}

watch(() => props.open, (open) => {
  if (open) reset()
})

watch(() => form.target, () => {
  form.key = ''
  form.fieldAction = 'read'
  form.unlimited = false
  form.value = '1'
  form.effect = effects.value[0]?.value ?? 'ENTITLEMENT_EFFECT_GRANT'
})

watch(() => form.moduleCode, () => {
  form.key = ''
})

function toRfc3339(value: string) {
  if (!value) return ''
  const parsed = new Date(value)
  return Number.isNaN(parsed.valueOf()) ? '' : parsed.toISOString()
}

function submit() {
  formError.value = ''
  if (!form.moduleCode) {
    formError.value = '请选择模块。'
    return
  }
  if (form.target !== 'ENTITLEMENT_TARGET_MODULE' && !form.key) {
    formError.value = '请选择目标 key。'
    return
  }
  if (!form.reason.trim()) {
    formError.value = '专项授权必须记录原因。'
    return
  }
  if (form.effect === 'ENTITLEMENT_EFFECT_SAFETY_MASK' && form.fieldAction === 'write') {
    formError.value = '安全脱敏只允许 read/export，不能用于 write。'
    return
  }
  if (form.target === 'ENTITLEMENT_TARGET_QUOTA' && form.effect === 'ENTITLEMENT_EFFECT_QUOTA_ADD') {
    const value = Number(form.value)
    if (form.unlimited || !Number.isFinite(value) || value <= 0) {
      formError.value = 'quota_add 必须是有限正数。'
      return
    }
  }

  const payload: CreateEntitlementOverrideInput = {
    requestId: '',
    expectedVersion: props.sourceVersion,
    moduleCode: form.moduleCode,
    target: form.target,
    key: form.target === 'ENTITLEMENT_TARGET_MODULE' ? '' : form.key,
    fieldAction: form.target === 'ENTITLEMENT_TARGET_FIELD' ? form.fieldAction : '',
    effect: form.effect,
    effectiveAt: toRfc3339(form.effectiveAt),
    expiresAt: toRfc3339(form.expiresAt),
    reason: form.reason.trim(),
  }
  if (form.target === 'ENTITLEMENT_TARGET_QUOTA') {
    payload.limit = {
      unlimited: form.unlimited,
      value: form.unlimited ? 0 : String(form.value || '0'),
    }
  }
  emit('submit', payload)
}
</script>

<template>
  <div v-if="open" class="dialog-backdrop" @click.self="emit('close')">
    <section class="dialog card" role="dialog" aria-modal="true" aria-labelledby="override-title">
      <header class="dialog-header">
        <div><h2 id="override-title">新增专项权益来源</h2><p>source_version {{ sourceVersion }} · 服务端 source_kind 固定为 override</p></div>
        <button class="btn" type="button" @click="emit('close')">关闭</button>
      </header>

      <div class="dialog-body">
        <label class="field"><span>模块</span><select v-model="form.moduleCode" class="input"><option v-for="item in modules" :key="item.moduleCode" :value="item.moduleCode">{{ item.name || item.moduleCode }} · {{ item.moduleCode }}</option></select></label>
        <label class="field"><span>目标类型</span><select v-model="form.target" class="input"><option value="ENTITLEMENT_TARGET_MODULE">模块</option><option value="ENTITLEMENT_TARGET_CAPABILITY">能力</option><option value="ENTITLEMENT_TARGET_QUOTA">额度</option><option value="ENTITLEMENT_TARGET_FIELD">字段动作</option></select></label>
        <label v-if="form.target !== 'ENTITLEMENT_TARGET_MODULE'" class="field"><span>目标 key</span><select v-model="form.key" class="input"><option value="">请选择</option><option v-for="key in availableKeys" :key="key" :value="key">{{ key }}</option></select></label>
        <label v-if="form.target === 'ENTITLEMENT_TARGET_FIELD'" class="field"><span>字段动作</span><select v-model="form.fieldAction" class="input"><option value="read">read</option><option value="write">write</option><option value="export">export</option></select></label>
        <label class="field"><span>效果</span><select v-model="form.effect" class="input"><option v-for="item in effects" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>

        <div v-if="form.target === 'ENTITLEMENT_TARGET_QUOTA'" class="quota-box">
          <label class="checkbox"><input v-model="form.unlimited" type="checkbox" />无限额度</label>
          <label v-if="!form.unlimited" class="field"><span>额度值</span><input v-model="form.value" class="input" type="number" min="0" step="1" /></label>
          <p>0 与 unlimited 语义不同；quota_add 必须是有限正数。</p>
        </div>

        <div class="two-column">
          <label class="field"><span>生效时间（可空）</span><input v-model="form.effectiveAt" class="input" type="datetime-local" /></label>
          <label class="field"><span>到期时间（可空）</span><input v-model="form.expiresAt" class="input" type="datetime-local" /></label>
        </div>
        <label class="field"><span>原因</span><textarea v-model="form.reason" class="input textarea" rows="3" placeholder="说明为什么需要本次专项授权或限制" /></label>

        <p v-if="formError || error" class="dialog-error" role="alert">{{ formError || error }}</p>
      </div>

      <footer class="dialog-footer">
        <button class="btn" type="button" @click="emit('close')">取消</button>
        <button class="btn primary" type="button" :disabled="pending" @click="submit">{{ pending ? '提交中…' : '创建专项来源' }}</button>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.dialog-backdrop { position: fixed; inset: 0; z-index: 80; display: grid; place-items: center; padding: 24px; background: rgba(15, 23, 42, .36); }
.dialog { width: min(760px, 100%); max-height: calc(100vh - 48px); overflow: auto; }
.dialog-header, .dialog-footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 20px; }
.dialog-header { border-bottom: 1px solid var(--color-border); }
.dialog-footer { border-top: 1px solid var(--color-border); justify-content: flex-end; }
.dialog-header h2 { margin: 0; font-size: 17px; }
.dialog-header p, .quota-box p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 12px; }
.dialog-body { display: grid; gap: 14px; padding: 20px; }
.field { display: grid; gap: 7px; font-size: 13px; color: var(--color-text-secondary); }
.textarea { resize: vertical; min-height: 80px; }
.two-column { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.quota-box { padding: 14px; border: 1px solid var(--color-border); border-radius: 10px; background: var(--color-surface-subtle); }
.checkbox { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.dialog-error { margin: 0; padding: 10px 12px; border-radius: 8px; color: var(--color-danger, #b42318); background: rgba(180, 35, 24, .08); }
@media (max-width: 700px) { .two-column { grid-template-columns: 1fr; } .dialog-backdrop { padding: 10px; } }
</style>
