/** Rental terms are contract versions, never permanent attributes of a site. */
export type RentalMode = 'fixed' | 'metered' | 'included'
export type BillingScope = 'independent' | 'shared'
export interface RentalSite {
  id: string
  customerId: string
  name: string
  parent: string
  kind: 'site' | 'group'
  scene: string
  address: string
  contact: string
  phone: string
  owner: string
  hours: string
  access: string
  water: string
  power: string
  network: string
  cleaning: string
  supplies: string
  phase: '待勘察' | '安装验收中' | '服务中' | '撤场中' | '已撤场'
  operation: '正常运营' | '临时停用'
  service: '正常' | '部分受限' | '待核实'
  nextAction: string
  nextAt: string
  version: number
}
export interface RentalDeployment {
  id: string
  siteId: string
  device: string
  model: string
  from: string
  until: string
  role: '主机' | '备用机'
  state: '有效' | '历史保留'
  online: boolean | null
}
export interface SiteUsage {
  siteId: string
  period: string
  raw: number
  excluded: number
  exclusionNote: string
  quality: '完整' | '待补传'
  asOf: string
  revision: number
}
export interface RentalRule {
  id: string
  groupId: string
  version: number
  name: string
  customerId: string
  contractId: string
  siteIds: string[]
  mode: RentalMode
  scope: BillingScope
  fixedUnit: 'site' | 'device'
  baseCents: number
  unitCents: number
  includedCups: number
  minimumKind: 'included' | 'minimum-spend'
  billingOwner: string
  billPresentation: '客户汇总' | '分别结算'
  effectiveFrom: string
  reason: string
}
export interface RentalDraft {
  rule: RentalRule
  samples: Record<string, number>
  testedFingerprint: string
}
export interface QuoteLine {
  key: string
  title: string
  cups: number | null
  baseCents: number
  includedCups: number
  overCups: number | null
  extraCents: number | null
  totalCents: number | null
}
export interface RentalQuote {
  period: string
  ruleId: string
  ruleVersion: number
  mode: RentalMode
  scope: BillingScope
  contributions: {
    siteId: string
    name: string
    raw: number | null
    excluded: number | null
    cups: number | null
    quality: string
  }[]
  lines: QuoteLine[]
  totalCents: number | null
  blockers: string[]
  sourceFingerprint: string
}
export interface RentalStatement {
  id: string
  groupId: string
  period: string
  rule: RentalRule
  quote: RentalQuote
  state: '草稿' | '异议处理中' | '已确认'
  history: { time: string; action: string; reason: string }[]
  confirmedAt: string
}
export interface RentalState {
  schema: 1
  operations: { key: string; payload: string; target: string; detail: string }[]
  profiles: RentalSite[]
  deployments: RentalDeployment[]
  usage: SiteUsage[]
  rules: RentalRule[]
  drafts: Record<string, RentalDraft>
  statements: RentalStatement[]
}
