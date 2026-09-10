import type { CustomerSnapshot } from '@/types/customer'
import type { Command, CommandResult } from './commands'
import { addWork, addActivity } from './commands'
import { operators } from './seed'
import {
  assertRule,
  editable,
  requireText,
  workById,
  sourceById,
  closeRequirements,
  assertNoCycle,
} from './policy'
const workActions = [
  'visit',
  'assign',
  'reschedule',
  'dependency',
  'transition',
  'reject',
  'accept',
  'reopen',
  'nonrenewal',
  'comment',
  'subtask',
  'link-source',
]
export function workCommand(s: CustomerSnapshot, c: Command): CommandResult | undefined {
  const { action, id, values: v, version } = c
  if (!workActions.includes(action)) return
  if (['assign', 'reopen', 'reject'].includes(action))
    assertRule(operators.includes(String(v.owner)), 'INVALID_OWNER', '负责人不在当前预览成员范围')
  if (['reopen', 'reject', 'transition'].includes(action)) {
    const deadline = String(v.deadline || s.work.find((item) => item.id === id)?.deadline || '')
    assertRule(
      String(v.nextAt || '') <= `${deadline}T23:59`,
      'AFTER_DEADLINE',
      '下次行动时间不能晚于事项截止时间',
    )
  }
  if (action === 'assign') {
    const selected = c.selected || []
    assertRule(selected.length, 'NO_SELECTION', '请先选择事项')
    const owner = requireText(v, 'owner', '接收人')
    requireText(v, 'reason', '分配说明')
    let count = 0
    const skipped: string[] = []
    selected.forEach((wid) => {
      const w = workById(s, wid)
      if (!w.writable || w.status === '已结束') {
        skipped.push(w.id)
        return
      }
      w.owner = owner
      w.version++
      count++
    })
    assertRule(count > 0, 'NOTHING_ELIGIBLE', '所选事项均不可分配，未修改任何记录')
    return {
      target: id,
      detail: `成功分配 ${count} 项给 ${owner}；跳过 ${skipped.length} 项${skipped.length ? '（' + skipped.join('、') + '）' : ''}`,
    }
  }
  const w = workById(s, id)
  editable(w, version)
  if (action === 'reopen') {
    assertRule(w.status === '已结束', 'NOT_CLOSED', '仅已结束事项可以重新打开')
    requireText(v, 'reason', '重新打开原因')
    w.owner = requireText(v, 'owner', '负责人')
    w.deadline = requireText(v, 'deadline', '截止日期')
    w.nextAt = requireText(v, 'nextAt', '下次行动')
    w.nextAction = requireText(v, 'nextAction', '下一步行动')
    w.status = '处理中'
    w.stage = 2
    w.resolution = ''
    w.cycle++
    w.version++
    return { target: id, detail: `已开始第 ${w.cycle} 轮处理，原验收证据和结束记录保留` }
  }
  if (action === 'comment') {
    const text = requireText(v, 'comment', '评论内容')
    addActivity(s, id, '评论', text, v.shared === true)
    return { target: id, detail: v.shared ? '已发布客户可见评论' : '已发布内部评论，客户不可见' }
  }
  assertRule(w.status !== '已结束', 'CLOSED_WORK', '此事项已结束，请重新打开后再操作')
  if (action === 'link-source') {
    const source = sourceById(s, w, requireText(v, 'sourceId', '业务来源'))
    if (!w.evidenceIds.includes(source.id)) w.evidenceIds.push(source.id)
    w.version++
    return { target: id, detail: `已关联 ${source.id}，业务状态仍由来源模块维护` }
  }
  if (action === 'subtask') {
    const sub = addWork(s, w.customerId, v, 'improvement')
    w.dependencies.push(sub.id)
    w.version++
    return { target: id, detail: `已创建子事项 ${sub.id} 并建立依赖`, navigate: `/customers/work/${id}` }
  }
  if (action === 'dependency') {
    const dependency = requireText(v, 'workId', '依赖事项')
    assertRule(
      workById(s, dependency).customerId === w.customerId,
      'SCOPE_MISMATCH',
      '依赖事项应属于同一客户',
    )
    assertNoCycle(s, id, dependency)
    if (!w.dependencies.includes(dependency)) w.dependencies.push(dependency)
    w.nextAction = requireText(v, 'nextAction', '解阻后的下一步')
    w.version++
    return {
      target: id,
      detail: `已关联依赖 ${dependency}；阻塞原因：${requireText(v, 'reason', '阻塞原因')}`,
    }
  }
  if (action === 'reschedule') {
    const nextAt = requireText(v, 'nextAt', '下次行动时间'),
      deadline = String(v.deadline || w.deadline)
    assertRule(
      nextAt <= `${deadline}T23:59`,
      'AFTER_DEADLINE',
      '行动时间不能晚于截止时间；调整截止时间需同时说明原因',
    )
    requireText(v, 'reason', '改期原因')
    w.nextAt = nextAt
    w.deadline = deadline
    w.nextAction = requireText(v, 'nextAction', '下一步行动')
    w.version++
    return { target: id, detail: '行动安排已更新，SLA 时钟不会因改期自动暂停' }
  }
  if (action === 'transition') {
    const target = requireText(v, 'status', '目标状态')
    assertRule(
      ['待开始', '处理中', '等待客户', '待验收'].includes(target),
      'USE_ACCEPTANCE',
      '成功关闭必须通过验收，不允许直接拖拽结束',
    )
    if (target === '待验收')
      assertRule(
        w.dependencies.every((d) => workById(s, d).status === '已结束'),
        'BLOCKED_DEPENDENCY',
        '依赖事项尚未完成，不能提交验收',
      )
    w.status = target as typeof w.status
    w.stage = target === '待验收' ? 3 : target === '待开始' ? 0 : 2
    w.nextAction = requireText(v, 'nextAction', '下一步行动')
    w.nextAt = requireText(v, 'nextAt', '下次行动时间')
    w.version++
    return { target: id, detail: `已流转至 ${target}；关联业务来源未被修改` }
  }
  if (action === 'reject') {
    requireText(v, 'reason', '未通过原因')
    w.owner = requireText(v, 'owner', '整改负责人')
    w.deadline = requireText(v, 'deadline', '整改截止日期')
    w.nextAt = requireText(v, 'nextAt', '复验安排')
    w.nextAction = requireText(v, 'nextAction', '整改动作')
    w.status = '处理中'
    w.stage = 2
    w.version++
    addActivity(s, id, '验收未通过', String(v.reason), v.shared === true)
    if (v.followup) {
      const sub = addWork(s, w.customerId, { ...v, title: String(v.title || `${w.title}整改`) })
      w.dependencies.push(sub.id)
    }
    return { target: id, detail: '验收未通过，已退回处理；已通过点位保持原验收结果' }
  }
  if (action === 'nonrenewal') {
    const reason = requireText(v, 'reason', '未续约原因')
    assertRule(w.kind === 'renewal', 'WRONG_KIND', '仅续约事项适用')
    assertRule(v.disposition, 'DISPOSITION_REQUIRED', '请确认后续合同与点位处置')
    if (v.followup)
      addWork(s, w.customerId, { ...v, title: String(v.title || '未续约合同退租处置') }, 'return')
    w.status = '已结束'
    w.stage = 4
    w.resolution = '未达成'
    w.version++
    return { target: id, detail: `续约以“未达成”结束：${reason}。客户生命周期不随单份合同自动改变` }
  }
  if (action === 'visit') {
    assertRule(w.kind === 'visit', 'WRONG_KIND', '仅回访事项可以保存回访结果')
    const feedback = requireText(v, 'feedback', '回访反馈')
    const promise = String(v.promise || '').trim()
    assertRule(!promise || v.followup, 'PROMISE_WITHOUT_WORK', '有客户承诺时必须创建后续事项')
    let next = ''
    if (v.followup) {
      const follow = addWork(s, w.customerId, { ...v, title: String(v.title || promise || '回访后续行动') })
      next = follow.id
    }
    if (v.closeVisit) {
      w.status = '已结束'
      w.stage = 4
      w.resolution = '成功'
    }
    const customer = s.customers.find((x) => x.id === w.customerId)
    if (customer) {
      customer.lastContact = requireText(v, 'actualAt', '实际联系时间')
      customer.nextContact = String(v.nextAt || customer.nextContact)
      customer.version++
    }
    addActivity(
      s,
      id,
      '回访结果',
      `${String(v.actualAt)} · ${String(v.channel || '沟通')} · ${feedback}${promise ? `；客户承诺：${promise}；后续事项 ${next}` : ''}`,
    )
    w.version++
    return {
      target: id,
      detail: `回访记录已保存${v.closeVisit ? '，回访事项已结束' : ''}${next ? `；承诺由 ${next} 独立跟进` : ''}`,
    }
  }
  if (action === 'accept') {
    assertRule(w.status === '待验收', 'NOT_READY', '请先将事项提交到待验收状态')
    assertRule(
      w.dependencies.every((d) => workById(s, d).status === '已结束'),
      'BLOCKED_DEPENDENCY',
      '依赖事项仍未结束，请先处理阻塞',
    )
    const evidence = String(v.evidence || '')
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean)
    const missing = closeRequirements(s, w, evidence)
    assertRule(missing.length === 0, 'MISSING_EVIDENCE', '不能成功关闭，缺少：' + missing.join('、'))
    requireText(v, 'reason', '验收结论')
    assertRule(v.confirm, 'CONFIRM_REQUIRED', '请确认验收条件已逐项核对')
    w.evidenceIds = Array.from(new Set([...w.evidenceIds, ...evidence]))
    w.status = '已结束'
    w.stage = 4
    w.resolution = '成功'
    w.version++
    if (w.kind === 'delivery')
      s.sites
        .filter((site) => site.customerId === w.customerId && site.workId === w.id)
        .forEach((site) => {
          site.trial = true
          site.training = true
          site.accepted = true
        })
    if (w.kind === 'return') {
      const grant = s.grants.find((g) => g.workId === id)
      if (grant) grant.revoked = true
    }
    addActivity(s, id, '验收通过', String(v.reason), true)
    return { target: id, detail: '事项已按“成功”关闭，验收来源和处理记录已保留' }
  }
}
