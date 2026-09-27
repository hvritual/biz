import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  markAllNotificationsRead,
  notificationInboxErrorKey,
  notificationInboxRequestId,
  readNotificationInbox,
  type MarkAllReadReceipt,
  type NotificationInboxSnapshot,
} from '@/services/enterprise/notificationInboxRuntime'
import { readSession, type TrustedSession } from '@/services/runtime/api'
import { isPersonalProfileSession, samePersonalProfileSession } from '@/services/enterprise/personalProfileRuntime'

export const useNotificationInboxStore = defineStore('notification-inbox', () => {
  const snapshot = ref<NotificationInboxSnapshot | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const ready = ref(false)
  const error = ref('')

  let generation = 0
  let activeRead: { signature: string; promise: Promise<boolean> } | null = null
  let mutation: { signature: string; key: string } | null = null

  function signature(session: TrustedSession) {
    return JSON.stringify({
      authenticated: session.authenticated,
      actorKind: session.actor_kind ?? '',
      userId: session.user_id ?? '',
      tenantId: session.active_tenant_id ?? '',
      contextVersion: session.context_version ?? 0,
    })
  }

  function clear() {
    generation += 1
    snapshot.value = null
    loading.value = false
    saving.value = false
    ready.value = false
    error.value = ''
    activeRead = null
    mutation = null
  }

  function accepts(session: TrustedSession, value: NotificationInboxSnapshot) {
    return isPersonalProfileSession(session) && value.tenant_id === session.active_tenant_id && value.user_id === session.user_id
  }

  async function load(session: TrustedSession): Promise<boolean> {
    const sessionSignature = signature(session)
    if (activeRead?.signature === sessionSignature) return activeRead.promise
    if (saving.value) return false

    const token = ++generation
    loading.value = true
    error.value = ''
    const promise = (async () => {
      try {
        const next = await readNotificationInbox(session)
        if (token !== generation) return false
        const current = await readSession()
        if (token !== generation || !samePersonalProfileSession(session, current) || !accepts(session, next)) return false
        snapshot.value = next
        ready.value = true
        return true
      } catch (caught) {
        if (token === generation) {
          snapshot.value = null
          ready.value = true
          error.value = notificationInboxErrorKey(caught)
        }
        return false
      } finally {
        if (token === generation) loading.value = false
        if (activeRead?.signature === sessionSignature) activeRead = null
      }
    })()
    activeRead = { signature: sessionSignature, promise }
    return promise
  }

  async function refresh(session?: TrustedSession | null) {
    const trusted = session ?? await readSession()
    if (!isPersonalProfileSession(trusted)) {
      clear()
      ready.value = true
      error.value = 'signIn'
      return false
    }
    return load(trusted)
  }

  async function markAllRead(session: TrustedSession): Promise<{ receipt: MarkAllReadReceipt; snapshot: NotificationInboxSnapshot } | null> {
    if (!isPersonalProfileSession(session) || saving.value) return null
    const current = snapshot.value
    if (!current || !accepts(session, current) || current.unread_count === 0) return null

    const stable = await readSession()
    if (!samePersonalProfileSession(session, stable)) {
      clear()
      error.value = 'sessionChanged'
      return null
    }
    const sessionSignature = signature(session)
    if (!mutation || mutation.signature !== sessionSignature) {
      mutation = { signature: sessionSignature, key: notificationInboxRequestId() }
    }

    const token = generation
    saving.value = true
    error.value = ''
    try {
      const receipt = await markAllNotificationsRead(session, mutation.key)
      if (token !== generation) return null
      const next = await readNotificationInbox(session)
      if (token !== generation) return null
      const after = await readSession()
      if (token !== generation || !samePersonalProfileSession(session, after) || !accepts(session, next)) return null
      snapshot.value = next
      ready.value = true
      mutation = null
      return { receipt, snapshot: next }
    } catch (caught) {
      if (token === generation) error.value = notificationInboxErrorKey(caught)
      return null
    } finally {
      if (token === generation) saving.value = false
    }
  }

  return { snapshot, loading, saving, ready, error, clear, refresh, markAllRead }
})
