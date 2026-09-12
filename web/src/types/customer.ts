import type { RentalState } from './siteRental'
export type Tone = 'success' | 'warning' | 'danger' | 'neutral' | 'primary'
export type Lifecycle = '潜在客户' | '试用中' | '合作中' | '合作终止'
export type WorkKind = 'visit' | 'delivery' | 'service' | 'payment' | 'renewal' | 'return' | 'improvement'
export type WorkStatus = '待开始' | '处理中' | '等待客户' | '待验收' | '已结束'
export type Resolution = '' | '成功' | '未达成' | '取消'
export interface Customer {
  id: string
  name: string
  category: string
  mode: string
  lifecycle: Lifecycle
  owner: string
  area: string
  tenantLink: string
  version: number
  archived: boolean
  sites: number
  devices: number
  lastContact: string
  nextContact: string
  risk: string
}
export interface Contact {
  id: string
  customerId: string
  name: string
  role: string
  phone: string
  email: string
  responsibility: string
  authorized: boolean
}
export interface WorkItem {
  siteIds?: string[]
  id: string
  customerId: string
  title: string
  kind: WorkKind
  status: WorkStatus
  stage: number
  owner: string
  collaborators: string[]
  priority: '高' | '中' | '低'
  deadline: string
  nextAction: string
  nextAt: string
  description: string
  criteria: string[]
  version: number
  workflowVersion: number
  resolution: Resolution
  cycle: number
  writable: boolean
  dependencies: string[]
  evidenceIds: string[]
  businessKey: string
  planId?: string
}
export interface SourceRecord {
  id: string
  customerId: string
  kind:
    | 'contract'
    | 'receivable'
    | 'acceptance'
    | 'placement'
    | 'service'
    | 'recovery'
    | 'confirmation'
    | 'settlement'
    | 'termination'
  title: string
  state: string
  verified: boolean
  facts: Record<string, string | number | boolean>
}
export interface DeliverySite {
  customerId: string
  workId: string
  id: string
  name: string
  device: string
  placement: string
  owner: string
  trial: boolean
  training: boolean
  accepted: boolean
}
export interface Plan {
  id: string
  customerId: string
  title: string
  owner: string
  start: string
  end: string
  state: '进行中' | '已结案'
  conclusion: string
  version: number
  targets: { title: string; baseline: number; actual: number; target: number; unit: string; source: string }[]
  milestones: { title: string; date: string; status: string; workIds: string[] }[]
}
export interface AutomationRule {
  id: string
  name: string
  trigger: string
  days: number
  scope: string
  owner: string
  enabled: boolean
  version: number
  tested: boolean
}
export interface Execution {
  id: string
  ruleId: string
  businessKey: string
  customerId: string
  state: '成功' | '部分失败' | '已恢复' | '跳过重复'
  workId: string
  failedStep: string
  time: string
  history: string[]
}
export interface ShareGrant {
  id: string
  customerId: string
  contactId: string
  workId: string
  fields: string[]
  attachments: string[]
  expires: string
  revoked: boolean
}
export interface Notice {
  id: string
  title: string
  description: string
  workId: string
  read: boolean
  category: string
}
export interface Activity {
  id: string
  target: string
  action: string
  actor: string
  time: string
  detail: string
  visibility: 'internal' | 'shared'
}
export interface Receipt {
  id: string
  key: string
  action: string
  target: string
  time: string
  detail: string
}
export interface SavedWorkView {
  name: string
  search: string
  kind: string
  owner: string
  density: string
  columns: string[]
}
export interface CustomerSnapshot {
  rental?: RentalState
  schema: 1
  tenant: string
  revision: number
  customers: Customer[]
  contacts: Contact[]
  work: WorkItem[]
  sources: SourceRecord[]
  sites: DeliverySite[]
  plans: Plan[]
  rules: AutomationRule[]
  executions: Execution[]
  grants: ShareGrant[]
  notifications: Notice[]
  activities: Activity[]
  receipts: Receipt[]
  views: SavedWorkView[]
  workflowVersion: number
  sla: { responseMinutes: number; recoveryHours: number; calendar: string; pause: string }
  drafts: Record<string, Record<string, string | boolean>>
}
export interface FormField {
  key: string
  label: string
  type?: 'text' | 'textarea' | 'select' | 'datetime-local' | 'date' | 'number' | 'checkbox'
  required?: boolean
  full?: boolean
  options?: string[]
  help?: string
  readonly?: boolean
}
export type FormValues = Record<string, string | boolean>
export interface ActionDefinition {
  title: string
  description: string
  primary: string
  fields: FormField[]
  note?: string
  warning?: string
  drawer?: boolean
}
