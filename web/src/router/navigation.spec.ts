import { describe, expect, it } from 'vitest'
import { customerDomains } from './customerNavigation'
import {
  enterpriseNavigation,
  isPrimaryNavigationActive,
  platformCommercialNavigation,
  primaryNavigation,
  systemNavigation,
  systemQuickActions,
} from './navigation'

describe('primary navigation information architecture', () => {
  it('keeps the rail focused on business domains instead of page-level entries', () => {
    expect(primaryNavigation.map((item) => item.label)).toEqual([
      '工作台',
      '客户经营',
      '租赁运营',
      '设备运营',
      '经营管理',
      '企业中心',
      '平台管理',
      '系统设置',
    ])
  })

  it('maps existing customer and rental routes back to their grouped primary domains', () => {
    const customer = primaryNavigation.find((item) => item.id === 'customer-operations')!
    const rental = primaryNavigation.find((item) => item.id === 'rental-operations')!

    expect(isPrimaryNavigationActive(customer, 'customers')).toBe(true)
    expect(isPrimaryNavigationActive(customer, 'success')).toBe(true)
    expect(isPrimaryNavigationActive(rental, 'sites')).toBe(true)
    expect(isPrimaryNavigationActive(rental, 'rental')).toBe(true)
    expect(isPrimaryNavigationActive(customer, 'sites')).toBe(false)
  })

  it('separates semantic domain ids from stable automation selectors', () => {
    expect(
      primaryNavigation
        .filter((item) => item.selectorId)
        .map((item) => [item.id, item.selectorId]),
    ).toEqual([
      ['customer-operations', 'customers'],
      ['rental-operations', 'sites'],
      ['device-operations', 'devices'],
      ['business-operations', 'orders'],
    ])
  })

  it('keeps enterprise center aligned with the approved six functional entries and terminology', () => {
    expect(enterpriseNavigation.map((item) => item.id)).toEqual([
      'members',
      'roles',
      'organization',
      'plan',
      'company',
      'logs',
    ])
    expect(enterpriseNavigation.map((item) => item.label)).toContain('套餐额度')
  })

  it('groups related operational functions and marks unimplemented routes explicitly', () => {
    expect(customerDomains['device-operations']?.links.map((item) => item.label)).toEqual([
      '设备管理',
      '远程运维',
      '故障工单',
    ])
    expect(customerDomains['business-operations']?.links.map((item) => item.label)).toEqual([
      '订单管理',
      '饮品管理',
      '数据分析',
    ])
    expect(
      customerDomains['business-operations']?.links.every((item) => item.path === undefined),
    ).toBe(true)
  })

  it('keeps platform management as one management-system domain', () => {
    expect(platformCommercialNavigation.map((item) => item.label)).toEqual([
      '平台总览',
      '租户管理',
      '模块目录',
      '套餐版本',
      '租户权益',
    ])
    expect(platformCommercialNavigation.every((item) => item.path?.startsWith('/platform/'))).toBe(true)
  })

  it('keeps system settings tenant-scoped and free of platform or enterprise shortcuts', () => {
    expect(systemNavigation.map((item) => item.label)).toEqual([
      '基础设置',
      '安全设置',
      '通知设置',
      '接口与集成',
      '数据字典',
    ])
    expect(systemNavigation.every((item) => item.path?.startsWith('/system/'))).toBe(true)
    expect(systemQuickActions.every((item) => item.path.startsWith('/system/'))).toBe(true)
  })
})
