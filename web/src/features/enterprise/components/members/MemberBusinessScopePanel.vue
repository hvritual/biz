<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiInput } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Member } from '@/types/enterprise'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import { sessionContext, type TrustedSession } from '@/services/runtime/api'
import {
  readCompleteCandidateDirectory, sameSiteSelection, scopeConfirmationMatches, validResourceVersion,
} from '@/services/enterprise/dataPermissionRules'
import {
  getEnterpriseMemberBusinessScope, listEnterpriseMemberScopeCandidates,
  memberRequestId, readEnterpriseMemberSession, sameTrustedSession, setEnterpriseMemberBusinessScope,
  type EnterpriseMemberBusinessScope, type EnterpriseMemberScopeCandidate,
} from '@/services/enterprise/memberRuntime'

const props = defineProps<{ member: Member; canRead?: boolean; canEdit?: boolean }>()
const emit = defineEmits<{ saved: [] }>()
const store = useEnterpriseStore(), ui = useUiStore(), { t } = useI18n()
const scope = ref<EnterpriseMemberBusinessScope | null>(null)
const candidates = ref<EnterpriseMemberScopeCandidate[]>([]), selected = ref<string[]>([])
const loading = ref(false), directoryLoading = ref(false), directoryReady = ref(false)
const editorOpen = ref(false), busy = ref(false), error = ref(''), candidateError = ref(''), query = ref('')
const pending = ref<{ userId: string; tenantId: string; siteIds: string[]; version?: string | number } | null>(null)
let epoch = 0, disposed = false
const identity = computed(() => JSON.stringify([
  props.member.id, props.canRead, props.canEdit, store.tenantId,
  sessionContext(store.session ?? { authenticated: false }),
]))
type Context = { key: string; epoch: number; memberId: string; session: TrustedSession }
const unavailableIds = computed(() => directoryReady.value
  ? selected.value.filter(id => !candidates.value.some(candidate => candidate.id === id && candidate.assignable)) : [])
const visibleCandidates = computed(() => {
  const keyword = query.value.trim().toLocaleLowerCase()
  return candidates.value.filter(candidate => !keyword || `${candidate.name} ${candidate.id}`.toLocaleLowerCase().includes(keyword))
})
const canSave = computed(() => props.canEdit && props.member.status !== 'removed' && scope.value &&
  directoryReady.value && !loading.value && !directoryLoading.value && !busy.value && !pending.value &&
  !unavailableIds.value.length && !sameSiteSelection(scope.value.siteIds, selected.value))
function active(context: Context) { return !disposed && context.key === identity.value && context.epoch === epoch }
function assertActive(context: Context) { if (!active(context)) throw new Error('CONTEXT_CHANGED') }
function capture(): Context {
  const session = store.session
  if (!session?.authenticated || !session.active_tenant_id) throw new Error('CONTEXT_CHANGED')
  return { key: identity.value, epoch, memberId: props.member.id, session: { ...session } }
}
async function trusted(context: Context) {
  const session = await readEnterpriseMemberSession()
  assertActive(context)
  if (!sameTrustedSession(context.session, session)) throw new Error('CONTEXT_CHANGED')
  return session
}
function failure(cause: unknown, fallback: string) {
  if (cause instanceof CommercialApiError && cause.code === 'forbidden') return t('dataPermissions.forbidden')
  if ((cause instanceof CommercialApiError && cause.code === 'unauthenticated') ||
      (cause instanceof Error && cause.message === 'CONTEXT_CHANGED')) return t('dataPermissions.sessionChanged')
  return t(fallback)
}
function validateIdentity(value: EnterpriseMemberBusinessScope, context: Context) {
  if (value.userId !== context.memberId || value.tenantId !== context.session.active_tenant_id ||
      !validResourceVersion(value.version)) throw new Error('INVALID_SCOPE_RESPONSE')
}
async function loadScope(context = capture()) {
  if (store.previewMode || !props.canRead) return
  loading.value = true; error.value = ''
  try {
    const session = await trusted(context)
    const value = await getEnterpriseMemberBusinessScope(session, context.memberId)
    assertActive(context); validateIdentity(value, context)
    scope.value = value
    if (!editorOpen.value) selected.value = [...value.siteIds]
    return value
  } catch (cause) {
    if (active(context)) { scope.value = null; error.value = failure(cause, 'dataPermissions.scopeLoadFailed') }
  } finally { if (active(context)) loading.value = false }
}
async function loadCandidates(context = capture()) {
  if (directoryLoading.value || busy.value || pending.value) return
  directoryLoading.value = true; directoryReady.value = false; candidateError.value = ''
  try {
    const session = await trusted(context)
    const all = await readCompleteCandidateDirectory(
      page => listEnterpriseMemberScopeCandidates(session, { page, pageSize: 100 }),
      () => assertActive(context),
    )
    assertActive(context)
    candidates.value = all; directoryReady.value = true
  } catch (cause) {
    if (active(context)) {
      candidates.value = []
      candidateError.value = failure(cause, cause instanceof Error && cause.message === 'CANDIDATE_DIRECTORY_CHANGED'
        ? 'dataPermissions.directoryChanged' : 'dataPermissions.directoryFailed')
    }
  } finally { if (active(context)) directoryLoading.value = false }
}
async function openEditor() {
  if (!props.canEdit || store.previewMode || busy.value || loading.value) return
  const context = capture()
  editorOpen.value = true; error.value = ''; query.value = ''; pending.value = null
  directoryReady.value = false
  const loaded = await loadScope(context)
  if (!active(context) || !loaded) return
  selected.value = [...loaded.siteIds]
  await loadCandidates(context)
}
function toggle(id: string, checked: boolean) {
  if (busy.value || pending.value || !directoryReady.value) return
  selected.value = checked ? [...new Set([...selected.value, id])].sort() : selected.value.filter(value => value !== id)
  error.value = ''
}
function removeUnavailable(id: string) {
  if (!busy.value && !pending.value) selected.value = selected.value.filter(value => value !== id)
}
async function confirmResult(context: Context) {
  const expected = pending.value
  if (!expected) return
  const session = await trusted(context)
  const verified = await getEnterpriseMemberBusinessScope(session, context.memberId)
  assertActive(context); validateIdentity(verified, context)
  if (!scopeConfirmationMatches(verified, expected)) { error.value = t('dataPermissions.mismatch'); return }
  scope.value = verified; selected.value = [...verified.siteIds]; pending.value = null
  ui.toast(t('dataPermissions.scopeSaved'), 'success')
  editorOpen.value = false; emit('saved')
}
async function retryConfirmation() {
  if (!pending.value || busy.value) return
  const context = capture()
  busy.value = true; error.value = ''
  try { await confirmResult(context) }
  catch (cause) { if (active(context)) error.value = failure(cause, 'dataPermissions.uncertain') }
  finally { if (active(context)) busy.value = false }
}
async function reloadEditor() {
  if (busy.value) return
  epoch += 1; pending.value = null; scope.value = null; selected.value = []; candidates.value = []
  directoryLoading.value = false; directoryReady.value = false
  const context = capture()
  const loaded = await loadScope(context)
  if (!active(context) || !loaded) return
  selected.value = [...loaded.siteIds]
  await loadCandidates(context)
}
async function save() {
  if (!canSave.value || !scope.value) return
  const context = capture()
  const target = { userId: context.memberId, tenantId: context.session.active_tenant_id!, siteIds: [...new Set(selected.value)].sort() }
  busy.value = true; error.value = ''
  let accepted = false
  try {
    const session = await trusted(context)
    assertActive(context)
    pending.value = target
    const receipt = await setEnterpriseMemberBusinessScope(session, scope.value, target.siteIds, memberRequestId('scope'))
    accepted = true
    assertActive(context); validateIdentity(receipt, context)
    pending.value = { ...target, version: receipt.version }
    await confirmResult(context)
  } catch (cause) {
    if (!active(context)) return
    if (accepted) error.value = failure(cause, 'dataPermissions.uncertain')
    else if (cause instanceof CommercialApiError && cause.code === 'conflict') {
      pending.value = null; directoryReady.value = false
      await loadScope(context)
      if (active(context)) {
        selected.value = [...(scope.value?.siteIds ?? [])]
        error.value = t('dataPermissions.scopeChanged')
      }
    } else if (cause instanceof CommercialApiError && cause.status >= 400 && cause.status < 500) {
      pending.value = null; error.value = failure(cause, 'dataPermissions.failed')
    } else error.value = failure(cause, pending.value ? 'dataPermissions.uncertain' : 'dataPermissions.failed')
  } finally { if (active(context)) busy.value = false }
}
watch(identity, () => {
  epoch += 1; scope.value = null; candidates.value = []; selected.value = []; pending.value = null
  editorOpen.value = false; busy.value = false; loading.value = false; directoryLoading.value = false
  directoryReady.value = false; error.value = ''; candidateError.value = ''; query.value = ''
  if (!store.previewMode && props.canRead && store.session?.authenticated) void loadScope()
}, { immediate: true, flush: 'sync' })
watch(() => props.member.runtimeVersion, () => {
  if (!editorOpen.value && !busy.value && !loading.value && props.canRead && store.session?.authenticated) void loadScope()
})
onBeforeUnmount(() => { disposed = true; epoch += 1 })
</script>
<template>
  <div class="scope-panel" data-member-business-scope>
    <template v-if="store.previewMode"><h3>{{ t('dataPermissions.currentScope') }}</h3><p class="notice-box">{{ t('dataPermissions.preview') }}</p></template>
    <p v-else-if="!canRead" class="notice-box">{{ t('dataPermissions.noReadAccess') }}</p>
    <template v-else>
      <div class="row-between"><h3>{{ t('dataPermissions.scopeHeading') }}</h3>
        <UiButton v-if="canEdit && member.status !== 'removed'" class="btn-link" :disabled="loading || busy" @click="openEditor">{{ t('dataPermissions.adjustScope') }}</UiButton>
      </div>
      <p class="secondary">{{ t('dataPermissions.scopeHint') }}</p>
      <p v-if="loading">{{ t('dataPermissions.loadingScope') }}</p>
      <div v-else-if="scope" class="scope-summary">
        <strong>{{ t(scope.siteIds.length ? 'dataPermissions.assigned' : 'dataPermissions.noSites', { count: scope.siteIds.length }) }}</strong>
        <small>{{ t('dataPermissions.scopeVersion', { version: scope.version }) }}</small>
      </div>
      <div v-if="scope?.siteIds.length" class="scope-tags"><span v-for="id in scope.siteIds" :key="id">{{ candidates.find(candidate => candidate.id === id)?.name || id }}</span></div>
      <p class="secondary">{{ t('dataPermissions.scopeSummary', { scope: t(`dataPermissions.scopeLabels.${member.scope}`) }) }}</p>
      <p v-if="error && !editorOpen" class="form-error" role="alert">{{ error }}</p>
      <UiButton v-if="error && !editorOpen" class="btn-link" :disabled="loading" @click="loadScope()">{{ t('dataPermissions.retry') }}</UiButton>
    </template>
    <UiDialog :open="editorOpen" :title="t('dataPermissions.scopeTitle', { name: member.name })" width="680px" @close="() => { if (!busy) editorOpen = false }">
      <div class="scope-editor" data-member-scope-dialog :aria-busy="loading || directoryLoading || busy">
        <div class="notice-box">{{ t('dataPermissions.scopeRules') }}</div>
        <p v-if="loading">{{ t('dataPermissions.loadingScope') }}</p>
        <p v-if="directoryLoading">{{ t('dataPermissions.directoryLoading') }}</p>
        <div v-if="unavailableIds.length" class="warning-box">
          <strong>{{ t('dataPermissions.unavailableObjects') }}</strong><p>{{ t('dataPermissions.unavailableHint') }}</p>
          <div v-for="id in unavailableIds" :key="id" class="withdrawn-row"><span>{{ id }}</span><UiButton class="btn-link" :disabled="busy || Boolean(pending)" @click="removeUnavailable(id)">{{ t('dataPermissions.remove') }}</UiButton></div>
        </div>
        <p v-if="candidateError" class="form-error" role="alert">{{ candidateError }}</p>
        <UiButton v-if="candidateError" class="btn-link" :disabled="directoryLoading" @click="loadCandidates()">{{ t('dataPermissions.retry') }}</UiButton>
        <template v-if="directoryReady">
          <UiInput v-model="query" :aria-label="t('dataPermissions.filter')" :placeholder="t('dataPermissions.filter')" />
          <p>{{ t('dataPermissions.selectedCount', { count: selected.length }) }}</p>
          <div class="candidate-list" :aria-label="t('dataPermissions.candidates')">
            <label v-for="candidate in visibleCandidates" :key="candidate.id" class="candidate-row" :class="{ unavailable: !candidate.assignable }">
              <UiInput type="checkbox" :aria-label="candidate.name" :checked="selected.includes(candidate.id)" :disabled="!candidate.assignable || busy || Boolean(pending)" @change="toggle(candidate.id, ($event.target as HTMLInputElement).checked)" />
              <span><strong>{{ candidate.name }}</strong><small>{{ candidate.id }} · {{ t(candidate.assignable ? 'dataPermissions.available' : 'dataPermissions.unavailable') }}</small></span>
            </label>
            <p v-if="!visibleCandidates.length">{{ t(candidates.length ? 'dataPermissions.noMatches' : 'dataPermissions.noCandidates') }}</p>
          </div>
        </template>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
        <UiButton v-if="error && !busy" class="btn-link" @click="reloadEditor">{{ t('dataPermissions.reload') }}</UiButton>
      </div>
      <template #footer>
        <UiButton class="btn" :disabled="busy" @click="editorOpen = false">{{ t('dataPermissions.cancel') }}</UiButton>
        <UiButton v-if="pending" class="btn btn-primary" :disabled="busy" @click="retryConfirmation">{{ t(busy ? 'dataPermissions.verifying' : 'dataPermissions.retryConfirmation') }}</UiButton>
        <UiButton v-else class="btn btn-primary" :disabled="!canSave" @click="save">{{ t(busy ? 'dataPermissions.saving' : 'dataPermissions.saveScope') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
<style scoped>
.scope-panel,.scope-editor{display:grid;gap:14px;min-width:0}.scope-summary{display:grid;gap:4px;padding:16px;background:var(--color-surface-soft);border:1px solid var(--color-border);border-radius:var(--radius-md)}.scope-summary small,.secondary{color:var(--color-text-secondary);font-size:12px;line-height:1.65}.scope-tags{display:flex;flex-wrap:wrap;gap:6px}.scope-tags span{max-width:100%;overflow:hidden;padding:5px 8px;border:1px solid var(--color-border);border-radius:var(--radius-sm);font-size:11px;text-overflow:ellipsis;white-space:nowrap}.candidate-list{display:grid;gap:8px;max-height:320px;overflow:auto}.candidate-row{display:flex;align-items:flex-start;gap:10px;padding:11px;border:1px solid var(--color-border);border-radius:var(--radius-sm)}.candidate-row>:deep([data-slot="input"]){width:18px;min-width:18px;height:18px;padding:0;flex-shrink:0}.candidate-row>span{min-width:0;overflow-wrap:anywhere}.candidate-row strong,.candidate-row small{display:block}.candidate-row small{margin-top:3px;color:var(--color-text-muted);font-size:11px}.candidate-row.unavailable{opacity:.65}.warning-box{padding:12px;border:1px solid var(--color-warning);border-radius:var(--radius-sm);background:var(--color-warning-soft)}.withdrawn-row{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-top:6px;font-size:11px;overflow-wrap:anywhere}
</style>
