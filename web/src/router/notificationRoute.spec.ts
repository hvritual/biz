import { beforeEach, describe, expect, it, vi } from 'vitest'
const access = vi.hoisted(() => ({ status: 'ready', allowed: new Set<string>(['notification.configuration.list']) }))
vi.mock('@/services/runtime/authorization', async (original) => ({
  ...await original<typeof import('@/services/runtime/authorization')>(),
  authorizationApiMode: () => true, currentAuthorizationState: access,
  currentAuthorizationAllowsAny: (required: string[]) => required.some(item => access.allowed.has(item)),
  currentAuthorizationAllows: (required: string) => access.allowed.has(required),
  currentAuthorizationMatchesSession: () => true, ensureCurrentAuthorization: vi.fn(async () => null), redirectToTrustedLogin: vi.fn(),
}))
import { router } from './index'
import { primaryNavigation, systemNavigation, systemQuickActions } from './navigation'
beforeEach(() => { access.status = 'ready'; access.allowed = new Set(['notification.configuration.list']) })
describe('notification route has bounded authorization', () => {
  it('binds the exact notification route without adding notification grants to unrelated sections', () => {
    expect(router.resolve('/system/notifications').meta.authorizationActions).toEqual(['notification.configuration.list'])
    for (const path of ['/system/general', '/system/security', '/system/integrations', '/system/dictionary']) {
      expect(router.resolve(path).meta.authorizationActions).toBeUndefined()
    }
  })
  it('allows the notification route but denies unrelated configuration deep links', async () => {
    await router.push('/system/notifications'); expect(router.currentRoute.value.path).toBe('/system/notifications')
    await router.push('/system/integrations'); expect(router.currentRoute.value.path).toBe('/authorization-state')
  })
  it('denies notification deep links after permission removal', async () => {
    access.allowed.clear(); await router.push('/system/notifications')
    expect(router.currentRoute.value.path).toBe('/authorization-state')
  })
  it('uses the same read action for primary, secondary and quick navigation', () => {
    expect(primaryNavigation.find(item => item.id === 'system')?.authorizationActions).toEqual(['notification.configuration.list'])
    expect(systemNavigation.find(item => item.id === 'notifications')?.authorizationActions).toEqual(['notification.configuration.list'])
    expect(systemQuickActions.find(item => item.path === '/system/notifications')?.authorizationActions).toEqual(['notification.configuration.list'])
  })
})
