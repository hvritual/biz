import type { CustomerSnapshot, AutomationRule, ShareGrant } from '@/types/customer'
import type { Command, CommandResult } from './commands'
import { addActivity, addWork, stamp } from './commands'
import { evaluateRule } from './automation'
import { assertRule, requireText, workById, clientProjection } from './policy'
export const sharedFields = ['事项标题', '交付范围', '处理进度', '共享附件']
export function governanceCommand(s: CustomerSnapshot, c: Command): CommandResult | undefined {
  const { action, id, values: v } = c
  if (action === 'create-rule') {
    const name = requireText(v, 'name', '规则名称'),
      days = Number(v.days)
    assertRule(Number.isInteger(days) && days >= 1 && days <= 365, 'BAD_DAYS', '阈值应为 1 至 365 天')
    const rule: AutomationRule = {
      id: `RULE-${crypto.randomUUID().slice(0, 6)}`,
      name,
      trigger: requireText(v, 'trigger', '触发条件'),
      days,
      scope: '合作中客户',
      owner: requireText(v, 'owner', '分配对象'),
      enabled: false,
      version: 1,
      tested: false,
    }
    s.rules.push(rule)
    return { target: rule.id, detail: '规则已保存为未启用草稿，请先执行无副作用测试' }
  }
  if (['rule-test', 'rule-publish', 'rule-toggle'].includes(action)) {
    const rule = s.rules.find((r) => r.id === id)
    assertRule(rule, 'NOT_FOUND', '规则不存在')
    if (action === 'rule-test') {
      rule.tested = true
      const matches = evaluateRule(s, rule)
      s.drafts[`test:${id}`] = {
        result: `命中 ${matches.length} 个对象；${matches.filter((m) => m.existing).length} 个已有事项；预计创建 ${matches.filter((m) => !m.existing).length} 项；实际创建 0 项；通知发送 0 次`,
        version: String(rule.version),
        at: stamp(),
      }
      return { target: id, detail: '试运行完成：业务记录未改变，未创建事项或发送通知' }
    }
    if (action === 'rule-publish') {
      assertRule(rule.tested, 'TEST_REQUIRED', '请先测试当前规则版本')
      requireText(v, 'reason', '发布说明')
      assertRule(v.confirm, 'CONFIRM_REQUIRED', '请确认新版本的影响范围')
      rule.enabled = true
      rule.version++
      return { target: id, detail: `规则 v${rule.version} 已在本地预览发布，历史执行回执不变` }
    }
    assertRule(!(!rule.enabled && !rule.tested), 'TEST_REQUIRED', '启用前需完成规则测试')
    rule.enabled = !rule.enabled
    return {
      target: id,
      detail: rule.enabled ? '规则已启用（本地预览，不启动后台定时器）' : '规则已停用，在途事项不删除',
    }
  }
  if (action === 'recover') {
    const execution = s.executions.find((e) => e.id === id)
    assertRule(execution, 'NOT_FOUND', '执行记录不存在')
    if (execution.state === '已恢复') return { target: id, detail: '该执行已经恢复，不重复创建事项或通知' }
    requireText(v, 'reason', '恢复说明')
    assertRule(v.reconciled, 'RECONCILE_REQUIRED', '请先确认已核对既有业务记录')
    const matches = s.work.filter((w) => w.businessKey === execution.businessKey)
    assertRule(matches.length <= 1, 'AMBIGUOUS_RECEIPT', '存在多个同业务键记录，必须人工对账，禁止自动补建')
    const created = matches.length === 0
    let work = matches[0]
    if (!work)
      work = addWork(
        s,
        execution.customerId,
        {
          title: '到期合同续约跟进',
          owner: '张敏',
          deadline: '2026-09-25',
          nextAt: '2026-09-12T10:00',
          nextAction: '确认客户续约意向',
        },
        'renewal',
      )
    work.businessKey = execution.businessKey
    execution.workId = work.id
    if (!s.notifications.some((n) => n.id === `RECOVER:${id}`))
      s.notifications.push({
        id: `RECOVER:${id}`,
        title: '续约事项提醒已补齐',
        description: work.title,
        workId: work.id,
        read: false,
        category: '执行恢复',
      })
    execution.history.push(`${stamp()} 对账命中 ${work.id}，仅补齐本地站内通知记录；原失败回执保留`)
    execution.state = '已恢复'
    execution.failedStep = ''
    return { target: id, detail: `恢复完成：复用 ${work.id}，新增事项 ${created ? 1 : 0} 项，已补齐本地提醒` }
  }
  if (action === 'workflow-publish') {
    requireText(v, 'reason', '版本变更说明')
    assertRule(
      v.scope === '仅新建事项',
      'MIGRATION_REQUIRED',
      '本轮只允许新建事项使用新版本；迁移在途事项需独立评估',
    )
    assertRule(v.confirm, 'PROTECTED_RULE', '必须保留责任、证据、权限及审计约束')
    s.workflowVersion++
    return {
      target: 'workflow',
      detail: `模板 v${s.workflowVersion} 已发布；已有事项继续使用原版本，不自动迁移`,
    }
  }
  if (action === 'sla') {
    const responseMinutes = Number(v.responseMinutes),
      recoveryHours = Number(v.recoveryHours)
    assertRule(
      responseMinutes > 0 && responseMinutes <= 1440 && recoveryHours > 0 && recoveryHours <= 720,
      'BAD_SLA',
      '响应时限为 1–1440 分钟，恢复时限为 1–720 小时',
    )
    s.sla = {
      responseMinutes,
      recoveryHours,
      calendar: requireText(v, 'calendar', '工作日历'),
      pause: requireText(v, 'pause', '暂停规则'),
    }
    return { target: 'sla', detail: '服务时限策略已保存；事项截止时间与下一次行动独立计算' }
  }
  if (action === 'share') {
    const work = workById(s, requireText(v, 'workId', '共享事项'))
    const contact = s.contacts.find((p) => p.id === v.contactId && p.customerId === work.customerId)
    assertRule(contact, 'CONTACT_SCOPE', '接收人必须属于同一客户')
    const expires = requireText(v, 'expires', '有效期')
    assertRule(new Date(`${expires}T23:59:59`) >= new Date(), 'BAD_EXPIRY', '有效期不能早于当前时间')
    const fields = String(v.fields || sharedFields.join(','))
      .split(',')
      .map((x) => x.trim())
    assertRule(
      fields.every((f) => sharedFields.includes(f)),
      'PRIVATE_FIELDS',
      '内部评论、利润、谈判信息不能共享',
    )
    const attachments = String(v.attachments || '')
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean)
    assertRule(
      attachments.every((a) =>
        s.sources.some(
          (source) =>
            source.id === a &&
            source.customerId === work.customerId &&
            (!source.facts.workId || source.facts.workId === work.id) &&
            ['acceptance', 'placement', 'confirmation'].includes(source.kind),
        ),
      ),
      'PRIVATE_ATTACHMENT',
      '仅允许共享授权客户下的交付、验收或确认记录',
    )
    const grant: ShareGrant = {
      id: `SH-${crypto.randomUUID().slice(0, 6)}`,
      customerId: work.customerId,
      contactId: contact.id,
      workId: work.id,
      fields,
      attachments,
      expires,
      revoked: false,
    }
    s.grants.push(grant)
    contact.authorized = true
    return { target: grant.id, detail: '共享范围已保存，可查看受限客户侧投影；未向外部实际发送邀请' }
  }
  if (action === 'revoke-share') {
    const grant = s.grants.find((g) => g.id === id)
    assertRule(grant, 'NOT_FOUND', '授权不存在')
    requireText(v, 'reason', '撤销原因')
    grant.revoked = true
    return { target: id, detail: '授权已撤销，客户侧入口立即失效；历史操作记录保留' }
  }
  if (action === 'client-accept' || action === 'client-reject') {
    const projection = clientProjection(s, id)
    const work = workById(s, projection.workId)
    assertRule(
      work.status !== '已结束',
      'CLOSED_WORK',
      '事项已结束，不能覆盖原验收结果；请联系服务方重新打开',
    )
    const reason = requireText(v, 'reason', action === 'client-accept' ? '确认说明' : '退回原因')
    if (action === 'client-accept') {
      assertRule(v.confirm, 'CONFIRM_REQUIRED', '请核对本次授权范围并确认')
      assertRule(projection.sites.length > 0, 'NO_SHARED_SCOPE', '未授权交付范围，无法执行逐点验收')
      const sourceId = `CLIENT-${crypto.randomUUID().slice(0, 6)}`
      s.sources.push({
        id: sourceId,
        customerId: work.customerId,
        kind: 'confirmation',
        title: '客户侧交付确认',
        state: '已确认',
        verified: true,
        facts: { confirmed: true, workId: work.id, grantId: id },
      })
      work.evidenceIds.push(sourceId)
      work.status = '待验收'
      work.version++
      addActivity(s, work.id, '客户提交确认', reason, true)
      return { target: work.id, detail: '客户确认已记录，服务方仍需核对投放及验收业务来源后结束事项' }
    }
    work.status = '处理中'
    work.stage = 2
    work.nextAction = `客户退回：${reason}`
    work.version++
    addActivity(s, work.id, '客户验收退回', reason, true)
    s.notifications.push({
      id: `N-${crypto.randomUUID()}`,
      title: '客户退回交付验收',
      description: reason,
      workId: work.id,
      read: false,
      category: '客户反馈',
    })
    return { target: work.id, detail: '反馈已进入原事项整改流程，已通过点位与原验收记录保留' }
  }
}
