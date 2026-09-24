import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CommercialApiError, mutate } from '@/services/commercial/platformCommercial'
import { readSession, type TrustedSession } from '@/services/runtime/api'
import { getMyPersonalProfile, type TenantPersonalProfile } from './personalProfileRuntime'
import {
  completeContactChange, completeTenantDeletion, confirmContactReadback, parseSecurityChallenge,
  personalSecurityErrorKey, requestSecurityCode, safeSecurityVersion, type SecurityChallenge,
} from './personalSecurityRuntime'

vi.mock('@/services/commercial/platformCommercial', async (original) => ({
  ...await original<typeof import('@/services/commercial/platformCommercial')>(), mutate: vi.fn(),
}))
vi.mock('@/services/runtime/api', async (original) => ({
  ...await original<typeof import('@/services/runtime/api')>(), readSession: vi.fn(),
}))
vi.mock('./personalProfileRuntime', async (original) => ({
  ...await original<typeof import('./personalProfileRuntime')>(), getMyPersonalProfile: vi.fn(),
}))
const session: TrustedSession = { authenticated: true, actor_kind: 'tenant', user_id: 'self', active_tenant_id: 'a', context_version: 7 }
const challenge: SecurityChallenge = { challenge_id: 'proof', flow_id: 'flow', masked_destination: 'n***@example.invalid',
  expires_at: '2099-01-01T00:00:00Z', delivery_state: 'PENDING', version: 2, resend_after_seconds: 60 }
const receipt = { tenant_id: 'a', user_id: 'self', email: 'n***@example.invalid', phone: '', version: 3,
  notification_event_id: 'event', notification_state: 'PENDING' }
beforeEach(() => { vi.resetAllMocks(); vi.mocked(readSession).mockResolvedValue(session) })

describe('personal security transport boundary', () => {
  it('binds contact requests to trusted session without client-selected identity', async () => {
    vi.mocked(mutate).mockResolvedValue(challenge)
    await requestSecurityCode(session, 'contact', 'email', 'password', 'new@example.invalid', 'same-request')
    expect(mutate).toHaveBeenCalledWith('/auth/personal/contact-change/request', 'POST',
      { channel: 'email', destination: 'new@example.invalid', current_password: 'password' },
      expect.objectContaining({ idempotencyKey: 'same-request', sessionContext: expect.stringContaining('"active_tenant_id":"a"') }))
  })
  it('does not send a write after a session or tenant switch', async () => {
    vi.mocked(readSession).mockResolvedValue({ ...session, active_tenant_id: 'b', context_version: 8 })
    await expect(requestSecurityCode(session, 'contact', 'email', 'password', 'new@example.invalid', 'key')).rejects.toThrow('sessionChanged')
    expect(mutate).not.toHaveBeenCalled()
  })
  it('never accepts masked user input as a contact target', async () => {
    await expect(requestSecurityCode(session, 'contact', 'email', 'password', 'n***@example.invalid', 'key')).rejects.toThrow('contactInput')
    expect(mutate).not.toHaveBeenCalled()
  })
  it('repeats the exact challenge target, not user identity, on completion', async () => {
    vi.mocked(mutate).mockResolvedValue(receipt)
    await completeContactChange(session, 'email', challenge, 'new@example.invalid', '123456', 'confirm')
    expect(mutate).toHaveBeenCalledWith('/auth/personal/contact-change/complete', 'POST', {
      channel: 'email', destination: 'new@example.invalid', challenge_id: 'proof', flow_id: 'flow', otp_code: '123456', version: 2,
    }, expect.objectContaining({ idempotencyKey: 'confirm' }))
  })
  it('rejects unsafe version numbers instead of silently rounding them', () => {
    expect(() => safeSecurityVersion(Number.MAX_SAFE_INTEGER + 1)).toThrow('invalidResponse')
    expect(() => safeSecurityVersion('2')).toThrow('invalidResponse')
  })
  it('rejects an unmasked response and invalid resend metadata', () => {
    expect(() => parseSecurityChallenge({ ...challenge, masked_destination: 'new@example.invalid' })).toThrow()
    expect(() => parseSecurityChallenge({ ...challenge, resend_after_seconds: 0 })).toThrow()
  })
  it('does not publish a receipt belonging to another tenant or containing raw contacts', async () => {
    vi.mocked(mutate).mockResolvedValueOnce({ ...receipt, tenant_id: 'b' }).mockResolvedValueOnce({ ...receipt, email: 'new@example.invalid' })
    await expect(completeContactChange(session, 'email', challenge, 'new@example.invalid', '123456', 'key')).rejects.toThrow()
    await expect(completeContactChange(session, 'email', challenge, 'new@example.invalid', '123456', 'key')).rejects.toThrow()
  })
  it('requires matching profile readback and never repeats POST during recovery', async () => {
    vi.mocked(getMyPersonalProfile).mockResolvedValue({ tenantId: 'a', userId: 'self', version: '3', email: receipt.email, phone: '' } as TenantPersonalProfile)
    await confirmContactReadback(session, receipt)
    expect(mutate).not.toHaveBeenCalled()
    vi.mocked(getMyPersonalProfile).mockResolvedValue({ version: '2', email: receipt.email, phone: '' } as TenantPersonalProfile)
    await expect(confirmContactReadback(session, receipt)).rejects.toThrow('confirmRead')
  })
  it('requires explicit deletion consent and pins confirmation to the captured tenant', async () => {
    await expect(completeTenantDeletion(session, 'email', challenge, '123456', false, 'key')).rejects.toThrow('confirmation')
    expect(mutate).not.toHaveBeenCalled()
    vi.mocked(mutate).mockResolvedValue({ ...receipt, deleted_at: '2026-09-24T00:00:00Z', reauthentication_required: true })
    await completeTenantDeletion(session, 'email', challenge, '123456', true, 'key')
    expect(mutate).toHaveBeenCalledWith('/auth/personal/tenant-deletion/complete', 'POST', expect.objectContaining({
      confirm_tenant_id: 'a', confirm_irreversible: true, version: 2,
    }), expect.anything())
  })
  it('maps last-owner protection but never echoes untrusted contact or password text', () => {
    expect(personalSecurityErrorKey(new CommercialApiError('LAST_OWNER_PROTECTED', 409, 'conflict'))).toBe('lastOwner')
    expect(personalSecurityErrorKey(new CommercialApiError('secret@example.invalid password=123', 500, 'http'))).toBe('unavailable')
  })
})
