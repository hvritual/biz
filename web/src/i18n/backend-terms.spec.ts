import { afterEach, describe, expect, it } from 'vitest'
import { i18n } from './index'
import {
  backendBusinessText,
  backendErrorFallback,
  backendTermKnown,
  backendStateTone,
  backendTermDiagnostic,
  backendTermLabel,
} from './backend-terms'

afterEach(() => {
  i18n.global.locale.value = 'zh-CN'
})

describe('backend term presentation', () => {
  it.each([
    ['TRIAL', '试用', 'Trial', 'warning'],
    ['ACTIVE', '有效', 'Active', 'success'],
    ['GRACE', '宽限期', 'Grace period', 'warning'],
    ['RESTRICTED', '受限', 'Restricted', 'danger'],
    ['ENDED', '已结束', 'Ended', 'neutral'],
  ])('projects %s without a fallback or an incorrect success badge', (code, zh, en, tone) => {
    i18n.global.locale.value = 'zh-CN'
    expect(backendTermLabel('subscriptionState', code)).toBe(zh)
    expect(backendStateTone('subscriptionState', code)).toBe(tone)
    i18n.global.locale.value = 'en-US'
    expect(backendTermLabel('subscriptionState', code)).toBe(en)
  })

  it('does not turn unknown states or prototype keys into success', () => {
    for (const value of ['FUTURE', '__proto__', 'constructor', 'toString', '', undefined, {}]) {
      expect(backendTermKnown('subscriptionState', value)).toBe(false)
      expect(backendTermLabel('subscriptionState', value)).toBe('未知状态')
      expect(backendStateTone('subscriptionState', value)).toBe('neutral')
    }
    expect(backendTermLabel('sourceState', 'FUTURE')).toBe('未知状态')
    expect(backendTermLabel('receiptStatus', 'FUTURE')).toBe('未知状态')
    expect(backendTermDiagnostic('subscriptionState', 'FUTURE')).toEqual({ code: 'UNKNOWN_BACKEND_TERM', kind: 'subscriptionState', value: 'FUTURE' })
    expect(backendTermDiagnostic('subscriptionState', 'ACTIVE')).toBeNull()
    expect(backendTermDiagnostic('subscriptionState', '<script>secret</script>')?.value).toBeNull()
  })

  it('recognizes actual same-tier and renew classifications', () => {
    expect(backendTermLabel('changeClassification', 'SAME_TIER')).toBe('同级调整')
    expect(backendTermLabel('changeClassification', 'RENEW')).toBe('续订')
    expect(backendTermLabel('planState', 'PUBLISHED')).toBe('已发布')
    expect(backendTermLabel('planState', 'FUTURE')).toBe('未知状态')
  })

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
    expect(backendTermLabel('entitlementEffect', raw)).toBe('未知授权效果')
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
