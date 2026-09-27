import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { usePersonalSecurity } from './usePersonalSecurity'
import { completeContactChange, confirmContactReadback, requestSecurityCode } from '@/services/enterprise/personalSecurityRuntime'
import type { TenantPersonalProfile } from '@/services/enterprise/personalProfileRuntime'
vi.mock('@/services/enterprise/personalSecurityRuntime', async (original) => ({
  ...await original<typeof import('@/services/enterprise/personalSecurityRuntime')>(),
  requestSecurityCode: vi.fn(), completeContactChange: vi.fn(), confirmContactReadback: vi.fn(),
}))
const session = { authenticated: true, actor_kind: 'tenant', user_id: 'self', active_tenant_id: 'a', context_version: 1 }
const profile = { userId: 'self', tenantId: 'a', email: 'o***@example.invalid', phone: '', version: '2' } as TenantPersonalProfile
const challenge = { challenge_id: 'proof', flow_id: 'flow', masked_destination: 'n***@example.invalid', expires_at: '2099-01-01T00:00:00Z', delivery_state: 'PENDING', version: 2, resend_after_seconds: 60 }
const accepted = { user_id: 'self', tenant_id: 'a', email: 'n***@example.invalid', phone: '', version: 3, notification_event_id: 'event', notification_state: 'PENDING' }
let cleanup: (() => void) | undefined
beforeEach(() => { vi.resetAllMocks() })
afterEach(() => { cleanup?.(); vi.useRealTimers() })
function setup() {
  let flow!: ReturnType<typeof usePersonalSecurity>
  const applyProfile = vi.fn()
  const wrapper = mount(defineComponent({ setup() {
    flow = usePersonalSecurity({ currentSession: () => session, currentProfile: () => profile, applyProfile, deleted: vi.fn() })
    return () => null
  } }))
  cleanup = () => wrapper.unmount()
  flow.begin('contact', 'email'); flow.password.value = 'password'; flow.destination.value = 'new@example.invalid'
  return { flow, applyProfile }
}
it('clears the password and enforces the resend deadline', async () => {
  vi.useFakeTimers(); vi.mocked(requestSecurityCode).mockResolvedValue(challenge)
  const { flow } = setup(); await flow.send()
  expect(flow.password.value).toBe(''); expect(flow.seconds.value).toBe(60)
  flow.password.value = 'password'; await flow.send(); expect(requestSecurityCode).toHaveBeenCalledTimes(1)
  vi.advanceTimersByTime(60000); expect(flow.seconds.value).toBe(0)
  await flow.send(); expect(requestSecurityCode).toHaveBeenCalledTimes(2)
})
it('retains accepted receipt for read-only recovery without a second write', async () => {
  vi.mocked(requestSecurityCode).mockResolvedValue(challenge); vi.mocked(completeContactChange).mockResolvedValue(accepted)
  vi.mocked(confirmContactReadback).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ ...profile, email: accepted.email, version: '3' })
  const { flow, applyProfile } = setup(); await flow.send(); flow.code.value = '123456'; await flow.complete()
  expect(flow.confirmationPending.value).toBe(true); expect(applyProfile).not.toHaveBeenCalled()
  expect(flow.destination.value).toBe(''); expect(flow.code.value).toBe('')
  await flow.complete(); expect(completeContactChange).toHaveBeenCalledTimes(1)
  await flow.recover(); expect(flow.success.value).toBe(true); expect(applyProfile).toHaveBeenCalledTimes(1)
})
it('drops a late challenge after cancellation and clears transient secrets', async () => {
  let resolve!: (value: typeof challenge) => void
  vi.mocked(requestSecurityCode).mockImplementation(() => new Promise((done) => { resolve = done }))
  const { flow } = setup(); const task = flow.send(); flow.close(); resolve(challenge); await task
  expect(flow.open.value).toBe(false); expect(flow.challenge.value).toBeNull(); expect(flow.password.value).toBe(''); expect(flow.destination.value).toBe('')
})
it('does not infer success or retry destructive writes after an uncertain outcome', async () => {
  vi.mocked(requestSecurityCode).mockResolvedValue(challenge); vi.mocked(completeContactChange).mockRejectedValue(new Error('offline'))
  const { flow, applyProfile } = setup(); await flow.send(); flow.code.value = '123456'; await flow.complete()
  expect(flow.uncertain.value).toBe(true); await flow.complete()
  expect(completeContactChange).toHaveBeenCalledTimes(1); expect(applyProfile).not.toHaveBeenCalled()
})
