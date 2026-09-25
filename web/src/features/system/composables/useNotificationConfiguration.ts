import { computed, reactive, ref, shallowRef } from 'vue'
import type { TrustedSession } from '@/services/runtime/api'
import { samePersonalProfileSession, isPersonalProfileSession } from '@/services/enterprise/personalProfileRuntime'
import {
  ConfigurationReadbackChanged, configurationWriteWasRejected, confirmConfigurationReceipt,
  getMessageConfiguration, listMessageChannels, listMessageConfigurations, listMessageTypes,
  notificationConfigurationErrorKey, prepareConfigurationCommand, submitConfigurationCommand,
  type ConfigurationCommand, type ConfigurationDraft, type MessageChannel, type MessageConfiguration,
  type MessageConfigurationReceipt, type MessagePage, type MessageTypesPage,
} from '@/services/enterprise/notificationConfigurationRuntime'

export function emptyConfigurationDraft(): ConfigurationDraft {
  return { groupId: '', levels: [], channels: [], primaryUserId: '', secondaryUserId: '', additionalUserIds: [], notes: '' }
}
export function configurationDraft(value: MessageConfiguration): ConfigurationDraft {
  return { ...emptyConfigurationDraft(), groupId: value.groupId, levels: [value.level], channels: [...value.channels],
    primaryUserId: value.primaryUserId, secondaryUserId: value.secondaryUserId, additionalUserIds: [...value.additionalUserIds], notes: value.notes }
}
export function useNotificationConfiguration(currentSession: () => TrustedSession | null, allows: (operation: string) => boolean) {
  const rows = shallowRef<MessagePage<MessageConfiguration> | null>(null), types = shallowRef<MessageTypesPage | null>(null)
  const channels = shallowRef<readonly MessageChannel[]>([]), pending = shallowRef<ConfigurationCommand | null>(null)
  const receipt = shallowRef<MessageConfigurationReceipt | null>(null), lastReceipt = shallowRef<MessageConfigurationReceipt | null>(null)
  const selected = shallowRef<MessageConfiguration | null>(null), draft = reactive(emptyConfigurationDraft())
  const query = reactive({ groupId: '', level: '', recipientId: '', page: 1, pageSize: 10 })
  const typeQuery = reactive({ code: '', name: '', level: '', page: 1, pageSize: 10 })
  const busy = ref(false), listBusy = ref(false), typeBusy = ref(false), error = ref(''), listError = ref(''), typeError = ref('')
  const editorOpen = ref(false), deleteOpen = ref(false), recovery = ref<'' | 'write' | 'read'>(''), outcome = ref('')
  const loaded = ref(false), mustReload = ref(false)
  let epoch = 0, listSequence = 0, typeSequence = 0, loadedSession: TrustedSession | null = null
  const canStart = computed(() => loaded.value && !busy.value && !pending.value && !mustReload.value && !listBusy.value)
  function active(token: number, captured: TrustedSession) {
    const current = currentSession()
    return token === epoch && current !== null && samePersonalProfileSession(captured, current)
  }
  function invalidate(reason = '') {
    epoch++; listSequence++; typeSequence++; loadedSession = null; loaded.value = false
    rows.value = null; types.value = null; channels.value = []; selected.value = null; pending.value = null
    receipt.value = null; lastReceipt.value = null; error.value = reason; listError.value = ''; typeError.value = ''
    busy.value = false; listBusy.value = false; typeBusy.value = false; recovery.value = ''; outcome.value = ''
    editorOpen.value = false; deleteOpen.value = false; mustReload.value = false
    Object.assign(draft, emptyConfigurationDraft())
    Object.assign(query, { groupId: '', level: '', recipientId: '', page: 1, pageSize: 10 })
    Object.assign(typeQuery, { code: '', name: '', level: '', page: 1, pageSize: 10 })
  }
  async function loadList() {
    if (!loadedSession || pending.value) return
    const captured = loadedSession, token = epoch, sequence = ++listSequence
    listBusy.value = true; listError.value = ''; rows.value = null
    try {
      const value = await listMessageConfigurations(captured, { ...query })
      if (active(token, captured) && sequence === listSequence) rows.value = value
    } catch (cause) { if (active(token, captured) && sequence === listSequence) listError.value = notificationConfigurationErrorKey(cause) }
    finally { if (active(token, captured) && sequence === listSequence) listBusy.value = false }
  }
  async function loadTypes() {
    if (!loadedSession) return
    const captured = loadedSession, token = epoch, sequence = ++typeSequence
    typeBusy.value = true; typeError.value = ''; types.value = null
    try {
      const value = await listMessageTypes(captured, { ...typeQuery })
      if (active(token, captured) && sequence === typeSequence) types.value = value
    } catch (cause) { if (active(token, captured) && sequence === typeSequence) typeError.value = notificationConfigurationErrorKey(cause) }
    finally { if (active(token, captured) && sequence === typeSequence) typeBusy.value = false }
  }
  async function load() {
    if (busy.value || pending.value) return
    const current = currentSession()
    if (!isPersonalProfileSession(current)) { invalidate('signIn'); return }
    const captured = { ...current }, token = ++epoch
    loadedSession = captured; loaded.value = false; error.value = ''; busy.value = true
    try {
      const value = await listMessageChannels(captured)
      if (!active(token, captured)) return
      channels.value = value; selected.value = null; editorOpen.value = false; deleteOpen.value = false
      Object.assign(draft, emptyConfigurationDraft())
      await Promise.all([loadList(), loadTypes()])
      if (active(token, captured)) { loaded.value = true; mustReload.value = false }
    } catch (cause) { if (active(token, captured)) error.value = notificationConfigurationErrorKey(cause) }
    finally { if (active(token, captured)) busy.value = false }
  }
  function startCreate() {
    if (!canStart.value || !allows('notification.configuration.create')) return
    selected.value = null; Object.assign(draft, emptyConfigurationDraft()); editorOpen.value = true; error.value = ''; outcome.value = ''; lastReceipt.value = null
  }
  async function startEdit(row: MessageConfiguration, kind: 'update' | 'delete' = 'update') {
    if (!canStart.value || !loadedSession || !allows(`notification.configuration.${kind}`)) return
    const captured = loadedSession, token = epoch; busy.value = true; error.value = ''; outcome.value = ''; lastReceipt.value = null
    try {
      const current = await getMessageConfiguration(captured, row.id)
      if (!active(token, captured)) return
      selected.value = current; Object.assign(draft, configurationDraft(current))
      editorOpen.value = kind === 'update'; deleteOpen.value = kind === 'delete'
    } catch (cause) { if (active(token, captured)) error.value = notificationConfigurationErrorKey(cause) }
    finally { if (active(token, captured)) busy.value = false }
  }
  function close() {
    if (busy.value) return
    editorOpen.value = false; deleteOpen.value = false
    if (!pending.value) { selected.value = null; Object.assign(draft, emptyConfigurationDraft()) }
  }
  async function execute() {
    const command = pending.value
    if (!command || busy.value) return
    const token = epoch; busy.value = true; error.value = ''
    try {
      if (!receipt.value) {
        const accepted = await submitConfigurationCommand(command)
        if (!active(token, command.session)) return
        receipt.value = accepted
      }
      await confirmConfigurationReceipt(command.session, receipt.value)
      if (!active(token, command.session)) return
      lastReceipt.value = receipt.value; pending.value = null; receipt.value = null; recovery.value = ''
      outcome.value = command.kind === 'delete' ? 'deleted' : 'saved'
      closeAfterSuccess(); await loadList()
    } catch (cause) {
      if (!active(token, command.session)) return
      const key = notificationConfigurationErrorKey(cause)
      if (key === 'signIn' || key === 'sessionChanged') { invalidate(key); return }
      error.value = key
      if (cause instanceof ConfigurationReadbackChanged) {
        lastReceipt.value = receipt.value; pending.value = null; receipt.value = null; recovery.value = ''
        outcome.value = 'changed'; closeAfterSuccess(); await loadList()
      } else if (receipt.value) recovery.value = 'read'
      else if (configurationWriteWasRejected(cause)) {
        pending.value = null; recovery.value = ''
        mustReload.value = ['conflict', 'forbidden', 'notFound', 'channelUnavailable'].includes(key)
      } else recovery.value = 'write'
    } finally { if (active(token, command.session)) busy.value = false }
  }
  function closeAfterSuccess() {
    editorOpen.value = false; deleteOpen.value = false; selected.value = null; Object.assign(draft, emptyConfigurationDraft())
  }
  async function confirm(kind: 'create' | 'update' | 'delete') {
    if (!loadedSession || busy.value || pending.value || mustReload.value || !allows(`notification.configuration.${kind}`)) return
    if (!active(epoch, loadedSession)) { invalidate('sessionChanged'); return }
    try { pending.value = prepareConfigurationCommand(kind, loadedSession, draft, selected.value, channels.value) }
    catch (cause) { error.value = notificationConfigurationErrorKey(cause); return }
    await execute()
  }
  async function recover() { if (recovery.value) await execute() }
  return { rows, types, channels, query, typeQuery, draft, selected, pending, lastReceipt,
    busy, listBusy, typeBusy, loaded, error, listError, typeError, mustReload, outcome, editorOpen, deleteOpen, recovery, canStart,
    invalidate, load, loadList, loadTypes, startCreate, startEdit, close, confirm, recover }
}
