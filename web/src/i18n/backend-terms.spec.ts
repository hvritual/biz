import { afterEach, describe, expect, it } from 'vitest'
import { i18n } from './index'
import { commercialStateVocabulary, type CommercialStateKind } from '@/services/commercial/state-vocabulary.generated'
import {
  backendBusinessText,
  backendErrorFallback,
  backendTermKnown,
  backendTermLabel,
  commercialStateCode,
  commercialStateDiagnostic,
  subscriptionStateTone,
} from './backend-terms'

afterEach(() => {
  i18n.global.locale.value = 'zh-CN'
})

describe('backend term presentation', () => {
  it('translates backend enums through the active locale', () => {
    i18n.global.locale.value = 'zh-CN'
    expect(backendTermLabel('entitlementEffect', 'ENTITLEMENT_EFFECT_DENY')).toBe('拒绝')
    expect(backendTermLabel('entitlementKey', 'tenant.members')).toBe('成员额度')
    i18n.global.locale.value = 'en-US'
    expect(backendTermLabel('entitlementEffect', 'ENTITLEMENT_EFFECT_DENY')).toBe('Deny')
    expect(backendTermLabel('entitlementKey', 'tenant.members')).toBe('Member quota')
  })

  it('never falls back to an unknown raw engineering code', () => {
    const raw = 'ENTITLEMENT_EFFECT_FUTURE_MODE'
    expect(backendTermKnown('entitlementEffect', raw)).toBe(false)
    expect(backendTermLabel('entitlementEffect', raw)).toBe('授权效果待确认')
    expect(backendTermLabel('entitlementEffect', raw)).not.toContain(raw)
  })

  it('translates module catalog codes used by backend DTOs', () => {
    expect(backendTermLabel('moduleCategory', 'operations')).toBe('运营能力')
    expect(backendTermLabel('entitlementKey', 'device.count')).toBe('设备额度')
    expect(backendTermLabel('entitlementKey', 'customer.phone')).toBe('客户手机号')
    expect(backendTermLabel('module', 'customer-operations')).toBe('客户经营')
  })

  it('translates permission codes and safely handles unknown permissions', () => {
    expect(backendTermLabel('permission', 'tenant.member.read')).toBe('查看成员')
    expect(backendTermLabel('permission', 'tenant.future.permission')).toBe('其他权限')
    i18n.global.locale.value = 'en-US'
    expect(backendTermLabel('permission', 'tenant.member.read')).toBe('View members')
  })

  it('translates tenant statuses without leaking unknown raw values', () => {
    expect(backendTermLabel('memberStatus', 'TENANT_MEMBER_STATUS_ACTIVE')).toBe('已启用')
    expect(backendTermLabel('memberStatus', 'TENANT_MEMBER_STATUS_FUTURE')).toBe('成员状态待确认')
  })

  it('provides localized safe fallbacks for unknown backend errors', () => {
    expect(backendErrorFallback('member')).toBe('成员信息暂不可用，请稍后重试。')
    i18n.global.locale.value = 'en-US'
    expect(backendErrorFallback('member')).toBe('Member information is temporarily unavailable. Try again later.')
  })

  it('keeps human business reasons but masks engineering-looking backend text', () => {
    expect(backendBusinessText('客户合同专项能力')).toBe('客户合同专项能力')
    expect(backendBusinessText('plan grant')).toBe('当前套餐已包含')
    expect(backendBusinessText('resolver_version')).toBe('按当前业务规则计算')
  })
})


describe('commercial source vocabulary', () => {
  it('covers every derived wire value in both supported locales', () => {
    for (const locale of ['zh-CN', 'en-US'] as const) {
      i18n.global.locale.value = locale
      for (const kind of Object.keys(commercialStateVocabulary) as CommercialStateKind[]) {
        for (const value of commercialStateVocabulary[kind]) {
          expect(backendTermKnown(kind, value), `${kind}.${value}`).toBe(true)
          const label = backendTermLabel(kind, value)
          expect(label).not.toContain('backendTerms.')
          expect(label).not.toBe(backendTermLabel(kind, 'NOT_A_KNOWN_VALUE'))
          expect(label.trim()).not.toBe('')
        }
      }
    }
  })

  it.each([
    ['ACTIVE', '有效', 'success'], ['TRIAL', '试用', 'primary'],
    ['GRACE', '宽限期', 'warning'], ['RESTRICTED', '使用受限', 'danger'],
    ['ENDED', '已结束', 'neutral'],
  ] as const)('projects subscription %s without inventing a different state', (code, label, tone) => {
    i18n.global.locale.value = 'zh-CN'
    expect(backendTermLabel('subscriptionState', code)).toBe(label)
    expect(backendTermLabel('subscriptionState', code.toLowerCase())).toBe(label)
    expect(subscriptionStateTone(code)).toBe(tone)
  })

  it.each(['GRACE_PERIOD', 'SUSPENDED', 'EXPIRED', 'TERMINATED', 'constructor', '__proto__'])('does not treat %s as a real subscription status', (code) => {
    i18n.global.locale.value = 'zh-CN'
    expect(commercialStateCode('subscriptionState', code)).toBeNull()
    expect(backendTermKnown('subscriptionState', code)).toBe(false)
    expect(backendTermLabel('subscriptionState', code)).toBe('状态未知，请刷新')
    expect(subscriptionStateTone(code)).toBe('neutral')
  })

  it('keeps diagnostics bounded and out of user copy', () => {
    const code = 'FUTURE_STATE_'.repeat(40)
    const diagnostic = commercialStateDiagnostic('subscriptionState', code)
    expect(diagnostic.known).toBe(false)
    if (!diagnostic.known) {
      expect(diagnostic.code).toBe('UNKNOWN_COMMERCIAL_STATE')
      expect(diagnostic.raw?.length).toBe(128)
    }
    expect(backendTermLabel('subscriptionState', code)).not.toContain(code)
  })

  it('never coerces objects or numbers into a known state', () => {
    const object = { toString: () => { throw new Error('must not stringify untrusted objects') } }
    for (const input of [object, 0, true, null, undefined, ['ACTIVE']]) {
      expect(commercialStateCode('subscriptionState', input)).toBeNull()
      expect(subscriptionStateTone(input)).toBe('neutral')
    }
  })

  it('keeps future receipts distinct from processing or success', () => {
    i18n.global.locale.value = 'en-US'
    expect(backendTermLabel('receiptStatus', 'FUTURE')).toBe('Unknown status; refresh to check')
    expect(backendTermLabel('receiptStatus', 'PROVISIONING')).toBe('Processing')
    expect(backendTermLabel('sourceState', 'scheduled')).toBe('Scheduled')
    expect(backendTermKnown('sourceState', 'PENDING')).toBe(false)
  })
})
