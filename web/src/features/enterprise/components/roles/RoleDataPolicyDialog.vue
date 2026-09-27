<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiOption, UiSelect } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Role } from '@/types/enterprise'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import { sessionContext, type TrustedSession } from '@/services/runtime/api'
import { validResourceVersion } from '@/services/enterprise/dataPermissionRules'
import {
  getEnterpriseRole, listEnterpriseDataPolicies, readEnterpriseRoleSession,
  roleRequestId, sameEnterpriseRoleSession, setEnterpriseRoleDataPolicy,
  type EnterpriseDataPolicy, type EnterpriseTenantRole,
} from '@/services/enterprise/roleRuntime'
import { formatDateTime } from '@/i18n'

const props = defineProps<{ open: boolean; role: Role | null; canManage?: boolean }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const store = useEnterpriseStore(), ui = useUiStore(), { t } = useI18n()
const current = ref<EnterpriseTenantRole | null>(null)
const policies = ref<EnterpriseDataPolicy[]>([])
const selected = ref(''), busy = ref(false), loading = ref(false), error = ref('')
const ready = ref(false)
const pending = ref<{ policyId: string; policyVersion: string | number; roleVersion?: string | number } | null>(null)
let epoch = 0, disposed = false
const identity = computed(() => JSON.stringify([
  props.open, props.role?.id, props.canManage, store.tenantId,
  sessionContext(store.session ?? { authenticated: false }),
]))
type Context = { key: string; epoch: number; roleId: string; session: TrustedSession }
const referenced = computed(() => current.value?.dataPolicy)
const selectedPolicy = computed(() => policies.value.find(policy => policy.id === selected.value) ?? null)
const selectable = (policy: EnterpriseDataPolicy) => policy.effective && policy.status === 'active' && validResourceVersion(policy.version)
const canSave = computed(() => props.canManage && !props.role?.builtin && ready.value &&
  !busy.value && !loading.value && !pending.value && Boolean(current.value) &&
  selected.value !== (current.value?.dataPolicy?.policyId ?? '') &&
  (!selected.value || Boolean(selectedPolicy.value && selectable(selectedPolicy.value))))
function reason(value: string) {
  const key = ['revoked', 'expired', 'not_started', 'missing'].includes(value) ? value : 'unavailable'
  return t(`dataPermissions.reason.${key}`)
}
function active(context: Context) { return !disposed && context.key === identity.value && context.epoch === epoch }
function assertActive(context: Context) { if (!active(context)) throw new Error('CONTEXT_CHANGED') }
function capture(): Context {
  const session = store.session
  if (!session?.authenticated || !session.active_tenant_id || !props.role) throw new Error('CONTEXT_CHANGED')
  return { key: identity.value, epoch, roleId: props.role.id, session: { ...session } }
}
async function trusted(context: Context) {
  const session = await readEnterpriseRoleSession()
  assertActive(context)
  if (!sameEnterpriseRoleSession(context.session, session)) throw new Error('CONTEXT_CHANGED')
  return session
}
function failure(cause: unknown, fallback: string) {
  if (cause instanceof CommercialApiError && cause.code === 'forbidden') return t('dataPermissions.forbidden')
  if ((cause instanceof CommercialApiError && cause.code === 'unauthenticated') ||
      (cause instanceof Error && cause.message === 'CONTEXT_CHANGED')) return t('dataPermissions.sessionChanged')
  return t(fallback)
}
async function load() {
  if (!props.open || !props.role || store.previewMode) return
  const context = capture()
  loading.value = true; ready.value = false; error.value = ''
  try {
    const session = await trusted(context)
    const [role, catalog] = await Promise.all([
      getEnterpriseRole(session, context.roleId), listEnterpriseDataPolicies(session),
    ])
    assertActive(context)
    if (role.id !== context.roleId || !validResourceVersion(role.version)) throw new Error('INVALID_ROLE')
    current.value = role; policies.value = catalog; selected.value = role.dataPolicy?.policyId ?? ''
    ready.value = true
  } catch (cause) {
    if (active(context)) error.value = failure(cause, 'dataPermissions.policyLoadFailed')
  } finally { if (active(context)) loading.value = false }
}
function reload() {
  if (busy.value) return
  epoch += 1; pending.value = null; current.value = null; policies.value = []; selected.value = ''
  void load()
}
function close() { if (!busy.value) emit('close') }
function selectPolicy(value: unknown) {
  if (busy.value || pending.value) return
  selected.value = String(value ?? ''); error.value = ''
}
async function confirmResult(context: Context) {
  const expected = pending.value
  if (!expected) return
  const session = await trusted(context)
  const verified = await getEnterpriseRole(session, context.roleId)
  assertActive(context)
  const ref = verified.dataPolicy
  if (verified.id !== context.roleId || !validResourceVersion(verified.version) ||
      (expected.roleVersion !== undefined && String(verified.version) !== String(expected.roleVersion)) ||
      (ref?.policyId ?? '') !== expected.policyId ||
      (expected.policyId && (String(ref?.acceptedVersion) !== String(expected.policyVersion) || !ref?.effective))) {
    error.value = t('dataPermissions.mismatch')
    return
  }
  current.value = verified; selected.value = ref?.policyId ?? ''; pending.value = null
  ui.toast(t('dataPermissions.policySaved'), 'success')
  emit('saved'); emit('close')
}
async function retryConfirmation() {
  if (!pending.value || busy.value) return
  const context = capture()
  busy.value = true; error.value = ''
  try { await confirmResult(context) }
  catch (cause) { if (active(context)) error.value = failure(cause, 'dataPermissions.uncertain') }
  finally { if (active(context)) busy.value = false }
}
async function save() {
  if (!canSave.value || !current.value) return
  const context = capture(), policy = selectedPolicy.value
  const target = { policyId: policy?.id ?? '', policyVersion: policy?.version ?? 0 }
  busy.value = true; error.value = ''
  let accepted = false
  try {
    const session = await trusted(context)
    assertActive(context)
    pending.value = target
    const receipt = await setEnterpriseRoleDataPolicy(session, current.value, policy, roleRequestId('data-policy'))
    accepted = true
    assertActive(context)
    pending.value = { ...target, roleVersion: receipt.version }
    await confirmResult(context)
  } catch (cause) {
    if (!active(context)) return
    if (accepted) error.value = failure(cause, 'dataPermissions.uncertain')
    else if (cause instanceof CommercialApiError && cause.code === 'conflict') {
      pending.value = null
      await load()
      if (active(context) && ready.value) error.value = t('dataPermissions.policyChanged')
    } else if (cause instanceof CommercialApiError && cause.status >= 400 && cause.status < 500) {
      pending.value = null; error.value = failure(cause, 'dataPermissions.failed')
    } else {
      error.value = failure(cause, pending.value ? 'dataPermissions.uncertain' : 'dataPermissions.failed')
    }
  } finally { if (active(context)) busy.value = false }
}
watch(identity, () => {
  epoch += 1; current.value = null; policies.value = []; selected.value = ''; pending.value = null
  ready.value = false; busy.value = false; loading.value = false; error.value = ''
  if (props.open && props.role && store.session?.authenticated && !store.previewMode) void load()
}, { immediate: true, flush: 'sync' })
onBeforeUnmount(() => { disposed = true; epoch += 1 })
</script>
<template>
  <UiDialog :open="open" :title="t('dataPermissions.policyTitle', { name: role?.name ?? '' })" width="620px" @close="close">
    <div class="policy-stack" data-role-data-policy-dialog :aria-busy="loading || busy">
      <div class="notice-box">{{ t('dataPermissions.policyHint') }}</div>
      <p v-if="loading">{{ t('dataPermissions.loadingPolicy') }}</p>
      <section v-else-if="ready" class="policy-current">
        <div class="row-between"><strong>{{ t('dataPermissions.currentPolicy') }}</strong>
          <StatusBadge v-if="referenced" :text="t(referenced.effective ? 'dataPermissions.effective' : 'dataPermissions.ineffective')" :tone="referenced.effective ? 'success' : 'danger'" />
        </div>
        <template v-if="referenced">
          <b>{{ referenced.policyName || referenced.policyId }}</b><small>{{ referenced.policyId }}</small>
          <small>{{ t('dataPermissions.policyVersion', { current: referenced.policyVersion, accepted: referenced.acceptedVersion }) }}</small>
          <p v-if="!referenced.effective" class="form-error">{{ t('dataPermissions.invalidPolicy', { reason: reason(referenced.invalidReason) }) }}</p>
        </template>
        <p v-else class="muted">{{ t('dataPermissions.unbound') }}</p>
      </section>
      <label class="field">
        <span>{{ t('dataPermissions.constraint') }}</span>
        <UiSelect :model-value="selected" :disabled="!ready || busy || loading || !canManage || Boolean(pending)" :aria-label="t('dataPermissions.constraint')" @update:model-value="selectPolicy">
          <UiOption value="">{{ t('dataPermissions.noPolicy') }}</UiOption>
          <UiOption v-for="policy in policies" :key="policy.id" :value="policy.id" :disabled="!selectable(policy)">
            {{ policy.name }} · v{{ policy.version }}{{ selectable(policy) ? '' : ` · ${reason(policy.invalidReason)}` }}
          </UiOption>
        </UiSelect>
      </label>
      <div v-if="selectedPolicy" class="policy-current">
        <strong>{{ selectedPolicy.name }}</strong><span>{{ t('dataPermissions.points', { count: selectedPolicy.siteIds.length }) }}</span>
        <small v-if="selectedPolicy.expiresAt">{{ t('dataPermissions.expires', { date: formatDateTime(selectedPolicy.expiresAt, store.session?.active_tenant_timezone ?? 'UTC') }) }}</small>
      </div>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <UiButton v-if="error && !busy" class="btn-link" @click="reload">{{ t('dataPermissions.reload') }}</UiButton>
    </div>
    <template #footer>
      <UiButton class="btn" :disabled="busy" @click="close">{{ t('dataPermissions.close') }}</UiButton>
      <UiButton v-if="pending" class="btn btn-primary" :disabled="busy" @click="retryConfirmation">{{ t(busy ? 'dataPermissions.verifying' : 'dataPermissions.retryConfirmation') }}</UiButton>
      <UiButton v-else-if="canManage" class="btn btn-primary" :disabled="!canSave" @click="save">{{ t(busy ? 'dataPermissions.saving' : 'dataPermissions.savePolicy') }}</UiButton>
    </template>
  </UiDialog>
</template>
<style scoped>
.policy-stack{display:grid;gap:16px}.policy-current{display:grid;gap:6px;padding:14px;border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-surface-soft);overflow-wrap:anywhere}.policy-current b{font-size:13px}.policy-current small{color:var(--color-text-muted);font-size:11px}.field{display:grid;gap:7px;min-width:0}.field>span{font-size:12px;font-weight:600}
</style>
