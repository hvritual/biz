import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  readSession: vi.fn(),
  readCurrentAuthorization: vi.fn(),
  subscribe: vi.fn(() => vi.fn()),
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

  it('fails closed when the aggregate tenant does not match the trusted session', async () => {
    mocks.readSession.mockResolvedValue(session)
    mocks.readCurrentAuthorization.mockResolvedValue(snapshot({ tenant_id: 'tenant-b' }))
    const auth = await runtime()

    await auth.ensureCurrentAuthorization()

    expect(auth.currentAuthorizationState.status).toBe('forbidden')
    expect(auth.currentAuthorizationAllows('tenant.member.list')).toBe(false)
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
