import { afterEach, describe, expect, it } from 'vitest'
import { i18n } from './index'
import {
  backendBusinessText,
  backendErrorFallback,
  backendTermKnown,
  backendTermLabel,
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
    expect(backendTermLabel('entitlementEffect', raw)).toBe('已应用')
    expect(backendTermLabel('entitlementEffect', raw)).not.toContain(raw)
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
