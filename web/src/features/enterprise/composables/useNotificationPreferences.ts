import { computed, reactive, shallowRef, ref } from 'vue'
import { commercialRequestId } from '@/services/commercial/platformCommercial'
import type { TrustedSession } from '@/services/runtime/api'
import { isPersonalProfileSession, samePersonalProfileSession } from '@/services/enterprise/personalProfileRuntime'
import {
  changeNotificationPreference, confirmNotificationPreference, readNotificationPreferences,
  notificationPreferenceErrorKey, preferenceWriteWasRejected, PreferenceReadbackChanged,
  type NotificationPreferences, type PreferenceChannel, type PreferenceReceipt,
} from '@/services/enterprise/notificationPreferencesRuntime'

type Pending = {
  session: TrustedSession
  current: NotificationPreferences
  channel: PreferenceChannel
  allowed: boolean
  key: string
}
export function useNotificationPreferences(currentSession: () => TrustedSession | null) {
  const snapshot = shallowRef<NotificationPreferences | null>(null)
  const draft = reactive({ sms: false, email: false })
  const selected = shallowRef<{ channel: PreferenceChannel; allowed: boolean } | null>(null)
  const pending = shallowRef<Pending | null>(null), receipt = shallowRef<PreferenceReceipt | null>(null)
  const busy = ref(false), dialogOpen = ref(false), error = ref(''), success = ref(false)
  const recovery = ref<'' | 'write' | 'read'>('')
  const canEdit = computed(() => Boolean(snapshot.value) && !busy.value && !pending.value && !error.value)
  let epoch = 0, loadedSession: TrustedSession | null = null
  function restoreDraft() {
    draft.sms = snapshot.value?.sms.allowed ?? false
    draft.email = snapshot.value?.email.allowed ?? false
  }
  function invalidate(reason = '') {
    epoch++; snapshot.value = null; selected.value = null; pending.value = null; receipt.value = null
    busy.value = false; dialogOpen.value = false; error.value = reason; success.value = false; recovery.value = ''
    loadedSession = null; restoreDraft()
  }
  function active(token: number, captured: TrustedSession) {
    const current = currentSession()
    return token === epoch && current !== null && samePersonalProfileSession(captured, current)
  }
  async function load() {
    if (busy.value || pending.value || dialogOpen.value) return
    invalidate()
    const current = currentSession()
    if (!isPersonalProfileSession(current)) { error.value = 'signIn'; return }
    const captured = { ...current }, token = epoch
    busy.value = true
    try {
      const value = await readNotificationPreferences(captured)
      if (!active(token, captured)) return
      loadedSession = captured; snapshot.value = value; restoreDraft()
    } catch (caught) { if (active(token, captured)) error.value = notificationPreferenceErrorKey(caught) }
    finally { if (active(token, captured)) busy.value = false }
  }
  function begin(channel: PreferenceChannel, allowed: boolean) {
    if (dialogOpen.value) return
    if (!canEdit.value || !loadedSession) { restoreDraft(); return }
    selected.value = { channel, allowed }; draft[channel] = allowed
    dialogOpen.value = true; success.value = false
  }
  function close() {
    if (busy.value) return
    dialogOpen.value = false; restoreDraft()
    if (!pending.value) selected.value = null
  }
  function accept(value: NotificationPreferences) {
    snapshot.value = value; pending.value = null; receipt.value = null; selected.value = null
    recovery.value = ''; dialogOpen.value = false; restoreDraft()
  }
  async function execute() {
    const operation = pending.value
    if (!operation || busy.value) return
    const token = epoch
    busy.value = true; error.value = ''; restoreDraft()
    try {
      if (!receipt.value) {
        const accepted = await changeNotificationPreference(operation.session, operation.current,
          operation.channel, operation.allowed, operation.key)
        if (!active(token, operation.session)) return
        receipt.value = accepted
      }
      const value = await confirmNotificationPreference(operation.session, receipt.value)
      if (!active(token, operation.session)) return
      accept(value); success.value = true
    } catch (caught) {
      if (!active(token, operation.session)) return
      const key = notificationPreferenceErrorKey(caught)
      if (key === 'sessionChanged' || key === 'signIn') { invalidate(key); return }
      if (caught instanceof PreferenceReadbackChanged) { accept(caught.current); error.value = key }
      else {
        restoreDraft(); error.value = key
        if (receipt.value) recovery.value = 'read'
        else if (preferenceWriteWasRejected(caught)) {
          pending.value = null; selected.value = null; dialogOpen.value = false; recovery.value = ''
        } else recovery.value = 'write'
      }
    } finally { if (active(token, operation.session)) busy.value = false }
  }
  async function confirm() {
    if (!selected.value || !snapshot.value || !loadedSession || pending.value || busy.value) return
    if (!active(epoch, loadedSession)) { invalidate('sessionChanged'); return }
    pending.value = { session: { ...loadedSession }, current: snapshot.value, ...selected.value,
      key: commercialRequestId('notification-preference') }
    await execute()
  }
  // An acknowledged write retries only GET. An uncertain write retries the
  // original immutable command with its original key, never a new mutation.
  async function recover() { if (recovery.value) await execute() }
  return { snapshot, draft, selected, busy, dialogOpen, error, success, recovery, canEdit,
    invalidate, load, begin, close, confirm, recover }
}
