import type { CustomerSnapshot, FormValues, WorkItem, WorkKind } from '@/types/customer'
import { assertRule, requireText } from './policy'
import { operators, workKindNames } from './seed'
import { customerCommand } from './customerCommands'
import { workCommand } from './workCommands'
import { governanceCommand } from './governanceCommands'
export type ActionId =
  | 'create-customer'
  | 'edit-customer'
  | 'contact'
  | 'handover'
  | 'visit'
  | 'create-work'
  | 'assign'
  | 'reschedule'
  | 'dependency'
  | 'transition'
  | 'reject'
  | 'accept'
  | 'reopen'
  | 'nonrenewal'
  | 'comment'
  | 'subtask'
  | 'link-source'
  | 'create-plan'
  | 'milestone'
  | 'recap'
  | 'triage'
  | 'create-rule'
  | 'rule-test'
  | 'rule-publish'
  | 'rule-toggle'
  | 'recover'
  | 'workflow-publish'
  | 'sla'
  | 'share'
  | 'revoke-share'
  | 'client-accept'
  | 'client-reject'
  | 'archive'
  | 'restore'
  | 'save-view'
  | 'read-notice'

export interface Command {
  action: ActionId
  id: string
  values: FormValues
  version?: number
  selected?: string[]
  key: string
}
export interface CommandResult {
  target: string
  detail: string
  navigate?: string
}
export const stamp = () =>
  new Intl.DateTimeFormat('sv-SE', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date())
export function addActivity(
  s: CustomerSnapshot,
  target: string,
  action: string,
  detail: string,
  shared = false,
) {
  s.activities.unshift({
    id: `EV-${crypto.randomUUID()}`,
    target,
    action,
    actor: '张敏',
    time: stamp(),
    detail,
    visibility: shared ? 'shared' : 'internal',
  })
}
export function addWork(
  s: CustomerSnapshot,
  customerId: string,
  values: FormValues,
  kind: WorkKind = 'improvement',
): WorkItem {
  assertRule(
    s.customers.some((c) => c.id === customerId && !c.archived),
    'CUSTOMER_MISSING',
    '请选择当前租户下未归档的客户',
  )
  assertRule(kind in workKindNames, 'INVALID_KIND', '事项类型无效')
  const title = requireText(values, 'title', '事项标题'),
    owner = requireText(values, 'owner', '负责人'),
    deadline = requireText(values, 'deadline', '截止时间')
  assertRule(operators.includes(owner), 'INVALID_OWNER', '负责人不在当前预览成员范围')
  const nextAction = requireText(values, 'nextAction', '下一步行动')
  const nextAt = requireText(values, 'nextAt', '下次行动时间')
  assertRule(nextAt <= `${deadline}T23:59`, 'AFTER_DEADLINE', '下次行动时间不能晚于事项截止时间')
  const n = Math.max(113, ...s.work.map((w) => Number(w.id.replace('CS-', ''))).filter(Number.isFinite)) + 1
  const w: WorkItem = {
    id: `CS-${n}`,
    customerId,
    title,
    kind,
    status: '待开始',
    stage: 0,
    owner,
    collaborators: [],
    priority: '中',
    deadline,
    nextAction,
    nextAt,
    description: String(values.description ?? '已明确责任、期限及验收条件的客户事项。'),
    criteria: [String(values.criteria || '执行结果有明确依据并完成确认')],
    version: 1,
    workflowVersion: s.workflowVersion,
    resolution: '',
    cycle: 1,
    writable: true,
    dependencies: [],
    evidenceIds: [],
    businessKey: `${s.tenant}:manual:${n}`,
  }
  if (values.sourceId) {
    const source = s.sources.find(
      (e) => e.id === values.sourceId && e.customerId === customerId && e.verified,
    )
    assertRule(source, 'INVALID_SOURCE', '业务来源必须属于同一客户并已核验')
    w.evidenceIds.push(source.id)
    if (kind === 'renewal') {
      w.businessKey = `${s.tenant}:${source.id}:${String(source.facts.end).slice(0, 7)}`
      assertRule(
        !s.work.some((old) => old.businessKey === w.businessKey && old.status !== '已结束'),
        'DUPLICATE_WORK',
        '本合同周期已有续约事项，请复用已有记录',
      )
    }
  }
  s.work.unshift(w)
  addActivity(s, w.id, '创建事项', title)
  return w
}
export function executeCustomerCommand(
  snapshot: CustomerSnapshot,
  command: Command,
): { snapshot: CustomerSnapshot; result: CommandResult } {
  assertRule(command.key.length > 0, 'KEY_REQUIRED', '缺少操作幂等标识')
  const previous = snapshot.receipts.find((r) => r.key === command.key)
  if (previous) return { snapshot, result: { target: previous.target, detail: previous.detail } }
  const s: CustomerSnapshot = JSON.parse(JSON.stringify(snapshot))
  const { action, values } = command
  // All policy evaluation and mutation is on a private copy. Failed checks never publish partial changes.
  let result: CommandResult | undefined
  if (action === 'create-work') {
    const w = addWork(
      s,
      requireText(values, 'customerId', '关联客户'),
      values,
      (values.kind || 'improvement') as WorkKind,
    )
    result = {
      target: w.id,
      detail: `已创建 ${w.id}，负责人 ${w.owner}`,
      navigate: `/customers/work/${w.id}`,
    }
  } else {
    result = customerCommand(s, command) ?? workCommand(s, command) ?? governanceCommand(s, command)
  }
  assertRule(result, 'UNKNOWN_ACTION', '尚未接入此业务动作，不执行写入')
  s.revision++
  s.receipts.unshift({
    id: `R-${crypto.randomUUID()}`,
    key: command.key,
    action,
    target: result.target,
    time: stamp(),
    detail: result.detail,
  })
  addActivity(s, result.target, action, result.detail)
  return { snapshot: s, result }
}
