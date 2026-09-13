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

function groups(domain: string) {
  return Array.from(new Set(customerDomains[domain]?.links.map((item) => item.group).filter(Boolean)))
}

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

  it('groups customer operations by management, collaboration and success instead of flattening pages', () => {
    expect(groups('customer-operations')).toEqual(['客户管理', '客户协同', '客户成功'])
    expect(customerDomains['customer-operations']?.links.map((item) => item.label)).not.toContain('客户工作区')
    expect(customerDomains['customer-operations']?.links.map((item) => item.label)).not.toContain('事项看板')
  })

  it('groups rental operations by lifecycle and never binds global navigation to sample entities', () => {
    expect(groups('rental-operations')).toEqual(['投放与资产', '计费与结算', '履约与服务'])

    const paths = Object.values(customerDomains).flatMap((domain) => [
      ...domain.links.map((item) => item.path).filter(Boolean),
      ...domain.actions.map((item) => item.path),
    ])
    expect(paths.some((path) => /\/(?:CUS-|CS-|SH-)[^/?]*/.test(String(path)))).toBe(false)

    for (const label of ['投放交付', '服务恢复验证', '回款跟进', '退租回收']) {
      const item = customerDomains['rental-operations']?.links.find((candidate) => candidate.label === label)
      expect(item?.path).toBeUndefined()
    }
  })

  it('places drink configuration under device operations rather than business operations', () => {
    expect(customerDomains['device-operations']?.links.map((item) => item.label)).toEqual([
      '设备管理',
      '饮品配置',
      '远程运维',
      '故障工单',
    ])
    expect(groups('device-operations')).toEqual(['设备资产', '设备配置', '远程运维', '服务维护'])
    expect(customerDomains['business-operations']?.links.map((item) => item.label)).toEqual([
      '订单管理',
      '数据分析',
    ])
    expect(customerDomains['business-operations']?.links.every((item) => item.path === undefined)).toBe(true)
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
