import { reactive, readonly } from 'vue'
import { cancelTrustedSessionRequests, CommercialApiError } from '@/services/commercial/platformCommercial'
import {
  loginUrl,
  readCurrentAuthorization,
  readSession,
  type CurrentAuthorizationResponse,
  type TrustedSession,
} from '@/services/runtime/api'
import { subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'

export type AuthorizationStatus =
  | 'idle'
  | 'loading'
  | 'ready'
  | 'unauthenticated'
  | 'forbidden'
  | 'error'

type AuthorizationState = {
  status: AuthorizationStatus
  snapshot: CurrentAuthorizationResponse | null
  session: TrustedSession | null
  contextKey: string
  error: string
}

const apiMode = (import.meta.env.VITE_DATA_MODE ?? 'demo') === 'api'
const state = reactive<AuthorizationState>({
  status: apiMode ? 'idle' : 'ready',
  snapshot: null,
  session: null,
  contextKey: '',
  error: '',
})

let generation = 0
let inFlight: Promise<CurrentAuthorizationResponse | null> | null = null
let unsubscribe: (() => void) | null = null

function isTenantUserActor(actorKind: string | undefined) {
  return actorKind === 'user' || actorKind === 'tenant'
}

function sessionKey(session: TrustedSession) {
  return [
    session.actor_kind ?? '',
    session.user_id ?? '',
    session.active_tenant_id ?? '',
    String(session.context_version ?? 0),
  ].join(':')
}

export function authorizationApiMode() {
  return apiMode
}

export const currentAuthorizationState = readonly(state)

export function invalidateCurrentAuthorization() {
  generation += 1
  inFlight = null
  state.status = apiMode ? 'idle' : 'ready'
  state.snapshot = null
  state.session = null
  state.contextKey = ''
  state.error = ''
}

export function currentAuthorizationAllows(actionCode: string) {
  if (!apiMode) return true
  if (state.status !== 'ready' || !state.snapshot) return false
  return state.snapshot.button_codes.includes(actionCode)
}

export function currentAuthorizationAllowsAny(actionCodes: readonly string[] = []) {
  if (!apiMode) return true
  if (!actionCodes.length) return false
  return actionCodes.some((code) => currentAuthorizationAllows(code))
}

export function currentAuthorizationModuleAllowed(moduleCode: string) {
  if (!apiMode) return true
  if (state.status !== 'ready' || !state.snapshot) return false
  return state.snapshot.modules.some((module) => module.code === moduleCode && module.allowed)
}

export function redirectToTrustedLogin() {
  window.location.assign(loginUrl())
}

export async function ensureCurrentAuthorization(force = false): Promise<CurrentAuthorizationResponse | null> {
  if (!apiMode) return null
  if (inFlight && !force) return inFlight

  const requestGeneration = generation
  const task = (async () => {
    const priorSnapshot = state.snapshot
    const priorContextKey = state.contextKey
    const priorReady = state.status === 'ready' && Boolean(priorSnapshot)
    if (!priorReady) state.status = 'loading'
    state.error = ''
    try {
      const session = await readSession()
      if (requestGeneration !== generation) return null
      state.session = session
      const key = sessionKey(session)
      if (!session.authenticated || !isTenantUserActor(session.actor_kind) || !session.active_tenant_id) {
        state.status = 'unauthenticated'
        state.snapshot = null
        state.contextKey = key
        return null
      }
      if (!force && priorSnapshot && priorContextKey === key) {
        state.snapshot = priorSnapshot
        state.contextKey = key
        state.status = 'ready'
        return priorSnapshot
      }

      state.status = 'loading'
      const snapshot = await readCurrentAuthorization()
      if (requestGeneration !== generation) return null
      if (
        !snapshot.authenticated ||
        !isTenantUserActor(snapshot.actor_kind) ||
        snapshot.actor_kind !== session.actor_kind ||
        snapshot.tenant_id !== session.active_tenant_id
      ) {
        state.status = 'forbidden'
        state.snapshot = null
        state.contextKey = key
        state.error = '服务端授权上下文与当前可信租户不一致。'
        return null
      }

      state.snapshot = snapshot
      state.contextKey = key
      state.status = 'ready'
      return snapshot
    } catch (cause) {
      if (requestGeneration !== generation) return null
      state.snapshot = null
      if (cause instanceof CommercialApiError && cause.status === 401) {
        cancelTrustedSessionRequests()
        state.session = null
        state.contextKey = ''
        state.status = 'unauthenticated'
      } else if (cause instanceof CommercialApiError && cause.status === 403) {
        state.status = 'forbidden'
      } else {
        state.status = 'error'
      }
      state.error = cause instanceof Error ? cause.message : '当前授权事实读取失败。'
      return null
    } finally {
      if (requestGeneration === generation) inFlight = null
    }
  })()

  inFlight = task
  return task
}

export function startAuthorizationSynchronization() {
  if (!apiMode || unsubscribe) return
  unsubscribe = subscribeSessionContextChange(() => {
    invalidateCurrentAuthorization()
    void ensureCurrentAuthorization().catch(() => undefined)
  })
  void ensureCurrentAuthorization().catch(() => undefined)
}

export function stopAuthorizationSynchronization() {
  unsubscribe?.()
  unsubscribe = null
}
