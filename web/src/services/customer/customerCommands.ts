import type { CustomerSnapshot, Customer, Lifecycle } from '@/types/customer'
import type { Command, CommandResult } from './commands'
import { operators } from './seed'
import { planResults } from './selectors'
import { addWork, stamp } from './commands'
import { assertRule, requireText, requireVersion, canArchive } from './policy'
export function customerCommand(s: CustomerSnapshot, c: Command): CommandResult | undefined {
  const { action, id, values: v, version } = c
  if (action === 'create-customer') {
    const name = requireText(v, 'name', '客户名称')
    assertRule(
      !s.customers.some((x) => x.name.replace(/\s/g, '') === name.replace(/\s/g, '')),
      'DUPLICATE_CUSTOMER',
      '发现同名客户，请使用已有客户；不会重复建档',
    )
    const owner = requireText(v, 'owner', '客户负责人')
    assertRule(operators.includes(owner), 'INVALID_OWNER', '负责人不在当前预览成员范围')
    assertRule(
      ['潜在客户', '试用中'].includes(String(v.lifecycle || '潜在客户')),
      'BAD_LIFECYCLE',
      '新建客户只允许潜在或试用状态，合作状态需结合合同确认',
    )
    const customer: Customer = {
      id: `CUS-${String(Math.max(186, ...s.customers.map((x) => Number(x.id.slice(4)))) + 1).padStart(4, '0')}`,
      name,
      category: requireText(v, 'category', '客户类型'),
      mode: String(v.mode || '设备租赁 + 服务'),
      lifecycle: (v.lifecycle || '潜在客户') as Lifecycle,
      owner,
      area: String(v.area || '华东'),
      tenantLink: String(v.tenantLink || ''),
      version: 1,
      archived: false,
      sites: 0,
      devices: 0,
      lastContact: '—',
      nextContact: String(v.nextAt || '—'),
      risk: '待评估',
    }
    s.customers.unshift(customer)
    if (v.title) addWork(s, customer.id, v, 'visit')
    return {
      target: customer.id,
      detail: '客户已建档；租户关联不自动共享内部数据',
      navigate: `/customers/accounts/${customer.id}`,
    }
  }
  if (action === 'edit-customer') {
    const customer = s.customers.find((x) => x.id === id)
    assertRule(customer, 'NOT_FOUND', '客户不存在')
    requireVersion(customer.version, version)
    const name = requireText(v, 'name', '客户名称')
    assertRule(
      !s.customers.some((x) => x.id !== id && x.name.trim() === name),
      'DUPLICATE_CUSTOMER',
      '客户名称与已有客户重复',
    )
    customer.name = name
    customer.owner = requireText(v, 'owner', '负责人')
    customer.category = requireText(v, 'category', '客户类型')
    customer.mode = String(v.mode || customer.mode)
    customer.area = String(v.area || customer.area)
    customer.version++
    return { target: id, detail: '客户资料已保存，版本已更新' }
  }
  if (action === 'contact') {
    const name = requireText(v, 'name', '姓名'),
      email = requireText(v, 'email', '邮箱')
    assertRule(/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email), 'INVALID_EMAIL', '请输入有效邮箱')
    s.contacts.push({
      id: `CT-${crypto.randomUUID().slice(0, 6)}`,
      customerId: id,
      name,
      email,
      phone: requireText(v, 'phone', '联系方式'),
      role: requireText(v, 'role', '职务'),
      responsibility: String(v.responsibility || '日常联系'),
      authorized: false,
    })
    return { target: id, detail: '联系人已保存。客户侧查看权限需单独授权' }
  }
  if (action === 'handover') {
    const customer = s.customers.find((x) => x.id === id)
    assertRule(customer, 'NOT_FOUND', '客户不存在')
    requireVersion(customer.version, version)
    const owner = requireText(v, 'owner', '接收人')
    requireText(v, 'reason', '移交原因')
    assertRule(v.confirm, 'CONFIRM_REQUIRED', '请确认移交范围')
    const before = customer.owner
    customer.owner = owner
    customer.version++
    let moved = 0
    if (v.includeWork)
      s.work
        .filter((w) => w.customerId === id && w.owner === before && w.status !== '已结束' && w.writable)
        .forEach((w) => {
          w.owner = owner
          w.version++
          moved++
        })
    return { target: id, detail: `客户已移交 ${owner}，同步移交 ${moved} 项可编辑事项；其他责任人的事项不变` }
  }
  if (action === 'archive' || action === 'restore') {
    const customer = s.customers.find((x) => x.id === id)
    assertRule(customer, 'NOT_FOUND', '客户不存在')
    requireVersion(customer.version, version)
    requireText(v, 'reason', action === 'archive' ? '归档原因' : '恢复原因')
    if (action === 'archive') {
      assertRule(v.confirm, 'CONFIRM_REQUIRED', '请确认归档范围和历史保留规则')
      const blockers = canArchive(s, id)
      assertRule(!blockers.length, 'ARCHIVE_BLOCKED', blockers.join('；'))
    }
    customer.archived = action === 'archive'
    customer.version++
    return {
      target: id,
      detail: action === 'archive' ? '客户已归档，历史事项与数据保留' : '客户已恢复，原历史记录保持不变',
    }
  }
  if (action === 'create-plan') {
    const customerId = requireText(v, 'customerId', '客户'),
      title = requireText(v, 'title', '计划名称'),
      owner = requireText(v, 'owner', '负责人'),
      start = requireText(v, 'start', '开始日期'),
      end = requireText(v, 'end', '结束日期')
    assertRule(end >= start, 'BAD_DATES', '结束日期不能早于开始日期')
    const planId = `PL-${String(s.plans.length + 9).padStart(3, '0')}`
    s.plans.push({
      id: planId,
      customerId,
      title,
      owner,
      start,
      end,
      state: '进行中',
      conclusion: '',
      version: 1,
      targets: [
        {
          title: requireText(v, 'target', '目标名称'),
          baseline: 0,
          actual: 0,
          target: Number(v.targetValue || 1),
          unit: String(v.unit || '项'),
          source: '等待关联验收结果',
        },
      ],
      milestones: [
        { title: '需求确认', date: start, status: '待开始', workIds: [] },
        { title: '结果复盘', date: end, status: '待开始', workIds: [] },
      ],
    })
    return {
      target: planId,
      detail: '经营计划已创建，请为里程碑关联执行事项',
      navigate: `/customers/plans/${planId}`,
    }
  }
  if (action === 'milestone' || action === 'recap') {
    const plan = s.plans.find((p) => p.id === id)
    assertRule(plan, 'NOT_FOUND', '计划不存在')
    requireVersion(plan.version, version)
    if (action === 'milestone') {
      const workId = requireText(v, 'workId', '关联事项')
      assertRule(
        s.work.some((w) => w.id === workId && w.customerId === plan.customerId),
        'SCOPE_MISMATCH',
        '事项必须属于计划客户',
      )
      const date = requireText(v, 'date', '里程碑日期')
      assertRule(date >= plan.start && date <= plan.end, 'BAD_DATES', '里程碑日期应在计划周期内')
      plan.milestones.push({
        title: requireText(v, 'title', '里程碑名称'),
        date,
        status: '待开始',
        workIds: [workId],
      })
      plan.version++
      return { target: id, detail: '里程碑已添加，关联事项保持同一条记录' }
    }
    const reason = requireText(v, 'reason', '复盘结论'),
      result = requireText(v, 'result', '达成结果')
    if (result === '全部达成')
      assertRule(
        planResults(s, plan).every((t) => t.actual >= t.target),
        'UNMET_TARGETS',
        '仍有未达成的经营目标，不能按全部达成结案',
      )
    const open = s.work.filter((w) => w.planId === id && w.status !== '已结束')
    assertRule(v.disposition || !open.length, 'OPEN_WORK', '请确认未结束事项继续由原负责人推进')
    if (v.followup)
      addWork(
        s,
        plan.customerId,
        {
          ...v,
          title: String(v.title || '经营计划复盘改进'),
          nextAction: String(v.nextAction || reason),
          criteria: '改进结果已验证',
        },
        'improvement',
      )
    plan.state = '已结案'
    plan.conclusion = `${result}：${reason}`
    plan.version++
    return { target: id, detail: `计划已按“${result}”结案，${open.length} 项未结束事项继续推进，不自动关闭` }
  }
  if (action === 'read-notice') {
    const notice = s.notifications.find((n) => n.id === id)
    assertRule(notice, 'NOT_FOUND', '通知不存在')
    notice.read = true
    return { target: notice.workId, detail: '通知已读，关联事项状态未改变' }
  }
  if (action === 'save-view') {
    const name = requireText(v, 'name', '视图名称')
    const view = {
      name,
      search: String(v.search || ''),
      kind: String(v.kind || ''),
      owner: String(v.owner || ''),
      density: String(v.density || '标准'),
      columns: String(v.columns || '客户,负责人,期限,下一步').split(','),
    }
    s.views = s.views.filter((x) => x.name !== name)
    s.views.push(view)
    return { target: id, detail: `视图“${name}”已保存，仅在当前租户及预览账户下生效` }
  }
  if (action === 'triage') {
    requireText(v, 'reason', '研判说明')
    const outcome = requireText(v, 'result', '研判结果')
    if (outcome === '创建经营检查事项') {
      const w = addWork(s, id, v)
      return { target: w.id, detail: `风险证据已关联 ${w.id}`, navigate: `/customers/work/${w.id}` }
    }
    if (outcome === '关联已有事项')
      assertRule(
        s.work.some((w) => w.id === v.workId && w.customerId === id),
        'WORK_REQUIRED',
        '请选择同一客户的已有事项',
      )
    s.drafts[`risk:${id}`] = { result: outcome, reason: String(v.reason), time: stamp() }
    return { target: id, detail: `研判已保存：${outcome}。原始数据证据保持不变` }
  }
}
