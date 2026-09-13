import { describe, expect, it } from 'vitest'
import { customerDomains } from './customerNavigation'
import {
  enterpriseNavigation,
  isPrimaryNavigationActive,
  primaryNavigation,
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

  it('keeps enterprise center aligned with the approved six functional entries', () => {
    expect(enterpriseNavigation.map((item) => item.id)).toEqual([
      'members',
      'roles',
      'organization',
      'plan',
      'company',
      'logs',
    ])
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
})
