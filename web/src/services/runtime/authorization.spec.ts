import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  readSession: vi.fn(),
  readCurrentAuthorization: vi.fn(),
  subscribe: vi.fn(() => vi.fn()),
  cancelTrusted: vi.fn(),
}))

vi.mock('@/services/runtime/api', () => ({
  readSession: mocks.readSession,
  readCurrentAuthorization: mocks.readCurrentAuthorization,
  loginUrl: () => '/api/auth/login?return_to=%2F',
}))

vi.mock('@/services/runtime/sessionCoordinator', () => ({
  subscribeSessionContextChange: mocks.subscribe,
}))

vi.mock('@/services/commercial/platformCommercial', () => ({
  cancelTrustedSessionRequests: mocks.cancelTrusted,
  CommercialApiError: class CommercialApiError extends Error {
    constructor(
      message: string,
      readonly status: number,
      readonly code: string,
    ) {
      super(message)
    }
  },
}))

const session = {
  authenticated: true,
  actor_kind: 'tenant',
  user_id: 'user-1',
  active_tenant_id: 'tenant-a',
  context_version: 3,
}

function snapshot(overrides: Record<string, unknown> = {}) {
  return {
    authenticated: true,
    actor_kind: 'tenant',
    user_id: 'user-1',
    tenant_id: 'tenant-a',
    tenant_name: 'Tenant A',
    timezone: 'Asia/Shanghai',
    roles: ['viewer'],
    grants: [{ permission: 'tenant.member.read', role_id: 'role-viewer', role_name: 'viewer', scope: 'all' }],
    data_policies: [],
    site_ids: [],
    permission_version: 'sha256:test',
    modules: [{ code: 'access-management', allowed: true, reason: 'allowed', actions: ['tenant.member.list'] }],
    actions: [{ code: 'tenant.member.list', domain: 'access', application: 'tenant_member_lifecycle', use_case: 'list_tenant_members', tenant_required: true, authentication: ['web-session'], permissions: ['tenant.member.read'], permission_mode: 'all' }],
    button_codes: ['tenant.member.list'],
    ...overrides,
  }
}

async function runtime() {
  vi.resetModules()
  vi.stubEnv('VITE_DATA_MODE', 'api')
  return import('./authorization')
}

describe('current authorization runtime', () => {
  beforeEach(() => {
    mocks.readSession.mockReset()
    mocks.readCurrentAuthorization.mockReset()
    mocks.subscribe.mockClear()
    mocks.cancelTrusted.mockClear()
  })

  it('accepts the real server tenant-user actor kind only when session and aggregate match', async () => {
    mocks.readSession.mockResolvedValue({ ...session, actor_kind: 'user' })
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot({ actor_kind: 'user' }))
    const auth = await runtime()

    await auth.ensureCurrentAuthorization()

    expect(auth.currentAuthorizationState.status).toBe('ready')
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(true)
  })

  it('rejects mismatched actor kinds even when tenant id matches', async () => {
    mocks.readSession.mockResolvedValue({ ...session, actor_kind: 'user' })
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot({ actor_kind: 'tenant' }))
    const auth = await runtime()

    await auth.ensureCurrentAuthorization()

    expect(auth.currentAuthorizationState.status).toBe('forbidden')
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(false)
  })

  it('allows only action codes returned by the current server aggregate', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot())
    const auth = await runtime()

    await auth.ensureCurrentAuthorization()

    expect(auth.currentAuthorizationState.status).toBe('ready')
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(true)
    expect(auth.currentAuthorizationAllows('tenant.member.invite')).toBe(false)
    expect(auth.currentAuthorizationModuleAllowed('access-management')).toBe(true)
  })

  it('exposes whether authorization belongs to the current trusted session context', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot())
    const auth = await runtime()

    await auth.ensureCurrentAuthorization()

    expect(auth.currentAuthorizationMatchesSession(session)).toBe(true)
    expect(auth.currentAuthorizationMatchesSession({ ...session, active_tenant_id: 'tenant-b', context_version: 4 })).toBe(false)
  })

  it('fails closed when the aggregate tenant does not match the trusted session', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot({ tenant_id: 'tenant-b' }))
    const auth = await runtime()

    await auth.ensureCurrentAuthorization()

    expect(auth.currentAuthorizationState.status).toBe('forbidden')
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(false)
  })

  it('keeps an already-valid same-context authorization rendered while rechecking session freshness', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot())
    const auth = await runtime()
    await auth.ensureCurrentAuthorization()
    expect(auth.currentAuthorizationState.status).toBe('ready')

    let resolveSession!: (value: typeof session) => void
    mocks.readSession.mockReturnValue(new Promise<typeof session>((resolve) => {
      resolveSession = resolve
    }))
    const pending = auth.ensureCurrentAuthorization()
    await Promise.resolve()

    expect(auth.currentAuthorizationState.status).toBe('ready')
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(true)

    resolveSession(session)
    await pending
    expect(auth.currentAuthorizationState.status).toBe('ready')
    expect(mocks.readCurrentAuthorization).toHaveBeenCalledTimes(1)
  })

  it('drops prior allow state when the trusted context changes', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot())
    const auth = await runtime()
    await auth.ensureCurrentAuthorization()
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(true)

    auth.invalidateCurrentAuthorization()

    expect(auth.currentAuthorizationState.status).toBe('idle')
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(false)
  })

  it('discards a late authorization response after tenant context invalidation', async () => {
    let resolveAuthorization!: (value: ReturnType<typeof snapshot>) => void
    const delayed = new Promise<ReturnType<typeof snapshot>>((resolve) => {
      resolveAuthorization = resolve
    })
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockReturnValue(delayed)
    const auth = await runtime()

    const pending = auth.ensureCurrentAuthorization()
    await Promise.resolve()
    auth.invalidateCurrentAuthorization()
    resolveAuthorization(snapshot())
    await pending

    expect(auth.currentAuthorizationState.status).toBe('idle')
    expect(auth.currentAuthorizationState.snapshot).toBeNull()
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(false)
  })

  it('clears trusted authorization context and in-flight requests on 401', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot())
    const auth = await runtime()
    await auth.ensureCurrentAuthorization()
    expect(auth.currentAuthorizationState.session?.active_tenant_id).toBe('tenant-a')

    const { CommercialApiError } = await import('@/services/commercial/platformCommercial')
    mocks.readSession.mockRejectedValue(new CommercialApiError('expired', 401, 'unauthenticated'))
    await auth.ensureCurrentAuthorization(true)

    expect(auth.currentAuthorizationState.status).toBe('unauthenticated')
    expect(auth.currentAuthorizationState.snapshot).toBeNull()
    expect(auth.currentAuthorizationState.session).toBeNull()
    expect(auth.currentAuthorizationState.contextKey).toBe('')
    expect(mocks.cancelTrusted).toHaveBeenCalledTimes(1)
  })

  it('does not turn authorization read failure into demo or cached allow', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockRejectedValue(new Error('authorization unavailable'))
    const auth = await runtime()

    await auth.ensureCurrentAuthorization()

    expect(auth.currentAuthorizationState.status).toBe('error')
    expect(auth.currentAuthorizationState.snapshot).toBeNull()
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(false)
  })
})
