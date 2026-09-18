type SessionContextSignal = Readonly<{
  type: 'session-context-changed'
  contextVersion: number
  nonce: string
}>

const channelName = 'coffeelink-session-context-v1'
const storageKey = '__coffeelink_session_context_signal_v1'

type Listener = (signal: SessionContextSignal) => void

const listeners = new Set<Listener>()
let channel: BroadcastChannel | null = null
let initialized = false

function randomNonce() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return String(Date.now()) + '-' + Math.random().toString(16).slice(2)
}

function parseSignal(value: unknown): SessionContextSignal | null {
  if (!value || typeof value !== 'object') return null
  const signal = value as Partial<SessionContextSignal>
  if (signal.type !== 'session-context-changed') return null
  const contextVersion = Number(signal.contextVersion ?? 0)
  if (!Number.isFinite(contextVersion) || contextVersion < 0) return null
  if (typeof signal.nonce !== 'string' || !signal.nonce) return null
  return { type: signal.type, contextVersion, nonce: signal.nonce }
}

function dispatch(value: unknown) {
  const signal = parseSignal(value)
  if (!signal) return
  for (const listener of [...listeners]) listener(signal)
}

function initialize() {
  if (initialized || typeof window === 'undefined') return
  initialized = true
  if (typeof BroadcastChannel !== 'undefined') {
    channel = new BroadcastChannel(channelName)
    channel.addEventListener('message', (event) => dispatch(event.data))
  }
  window.addEventListener('storage', (event) => {
    if (event.key !== storageKey || !event.newValue) return
    try {
      dispatch(JSON.parse(event.newValue))
    } catch {
      // A storage signal is only a wake-up hint; malformed values carry no authority.
    }
  })
}

export function subscribeSessionContextChange(listener: Listener) {
  initialize()
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function publishSessionContextChange(contextVersion = 0) {
  initialize()
  const signal: SessionContextSignal = {
    type: 'session-context-changed',
    contextVersion: Number.isFinite(contextVersion) ? Math.max(0, contextVersion) : 0,
    nonce: randomNonce(),
  }
  channel?.postMessage(signal)
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(signal))
  } catch {
    // BroadcastChannel remains the primary path. Storage is only a compatibility wake-up signal.
  }
}

export function parseSessionContextSignalForTest(value: unknown) {
  return parseSignal(value)
}
