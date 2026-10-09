import { readSession } from '@/services/runtime/api'
import type { ConfirmSubscriptionChangeInput } from './platformCommercial'

export interface InitialSubscriptionSubmission {
  tenantId: string
  changeId: string
  salesScope: string
  input: ConfirmSubscriptionChangeInput
}

// A tab-local request journal, not a subscription, entitlement or approval.
// Restoring it always requires fresh server preview/receipt reads; the server
// independently authenticates and admits every retry of the frozen request.
async function key(tenantId: string, changeId: string) {
  const session = await readSession()
  if (!session.authenticated || session.actor_kind !== 'platform' || !session.platform_subject) return null
  return `coffeelink:initial-request:${encodeURIComponent(session.platform_subject)}:${encodeURIComponent(tenantId)}:${encodeURIComponent(changeId)}`
}

function valid(value: unknown, tenantId: string, changeId: string): value is InitialSubscriptionSubmission {
  if (!value || typeof value !== 'object') return false
  const record = value as Partial<InitialSubscriptionSubmission>
  return record.tenantId === tenantId && record.changeId === changeId
    && typeof record.salesScope === 'string' && record.salesScope.length > 0 && record.salesScope.length <= 200
    && record.salesScope !== '*' && !!record.input
    && typeof record.input.requestId === 'string' && record.input.requestId.length > 0 && record.input.requestId.length <= 200
    && typeof record.input.previewHash === 'string' && /^[a-f0-9]{64}$/i.test(record.input.previewHash)
    && typeof record.input.reason === 'string' && record.input.reason.trim().length > 0 && record.input.reason.length <= 4096
}

export async function rememberInitialSubscription(record: InitialSubscriptionSubmission): Promise<boolean> {
  const storageKey = await key(record.tenantId, record.changeId)
  if (!storageKey || !valid(record, record.tenantId, record.changeId)) return false
  try { sessionStorage.setItem(storageKey, JSON.stringify(record)); return true } catch { return false }
}

export async function recallInitialSubscription(tenantId: string, changeId: string): Promise<InitialSubscriptionSubmission | null> {
  const storageKey = await key(tenantId, changeId)
  if (!storageKey) return null
  try {
    const raw = sessionStorage.getItem(storageKey)
    if (!raw || raw.length > 12000) return null
    const record: unknown = JSON.parse(raw)
    return valid(record, tenantId, changeId) ? record : null
  } catch { return null }
}

export async function forgetInitialSubscription(tenantId: string, changeId: string): Promise<void> {
  const storageKey = await key(tenantId, changeId)
  if (storageKey) {
    try { sessionStorage.removeItem(storageKey) } catch { /* Storage is not authority. */ }
  }
}
