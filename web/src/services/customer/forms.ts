import type { ActionId } from './commands'
import type { CustomerSnapshot, FormValues } from '@/types/customer'
import { customerForms } from './customerForms'
import { workForms } from './workForms'
import { governanceForms } from './governanceForms'
export const actionDefinitions = { ...customerForms, ...workForms, ...governanceForms }
export function initialValues(action: ActionId, id: string, s: CustomerSnapshot): FormValues {
  const w = s.work.find((x) => x.id === id),
    c = s.customers.find((x) => x.id === (w?.customerId || id)),
    plan = s.plans.find((x) => x.id === id)
  const v: FormValues = {
    owner: w?.owner || c?.owner || '张敏',
    customerId: c?.id || 'CUS-0186',
    kind: w?.kind || 'visit',
    category: c?.category || '连锁酒店',
    mode: c?.mode || '设备租赁 + 服务',
    lifecycle: '潜在客户',
    area: c?.area || '华东',
    deadline: w?.deadline || '2026-09-25',
    nextAt: w?.nextAt || '2026-09-12T10:00',
    nextAction: w?.nextAction || '',
    confirm: false,
  }
  if (action === 'edit-customer') v.name = c?.name || ''
  if (action === 'create-customer') v.nextAction = '确认客户投放意向与现场条件'
  if (action === 'visit')
    Object.assign(v, {
      channel: '电话回访',
      contact: '周岚',
      actualAt: '2026-09-10T10:30',
      feedback: '大堂咖啡机已恢复稳定使用，客户提出苏州新增点位需求。',
      promise: '9 月 15 日前提供苏州新增点位方案',
      followup: true,
      closeVisit: true,
      owner: '李川',
      deadline: '2026-09-15',
      nextAt: '2026-09-12T14:00',
      nextAction: '确认新增点位数量与设备配置',
    })
  if (action === 'accept') v.evidence = w?.evidenceIds.join(',') || ''
  if (action === 'transition') v.status = '待验收'
  if (action === 'reject')
    Object.assign(v, {
      reason: '高铁点位试运行未通过，需要调整水路后复验。',
      nextAction: '整改现场问题并提交复验记录',
      shared: true,
    })
  if (action === 'handover') Object.assign(v, { owner: '李川', includeWork: true })
  if (action === 'create-plan')
    Object.assign(v, { start: '2026-09-10', end: '2026-12-31', targetValue: '3', unit: '个' })
  if (action === 'milestone') Object.assign(v, { date: plan?.end || '2026-09-30', workId: 'CS-104' })
  if (action === 'recap')
    Object.assign(v, {
      result: '部分达成',
      followup: true,
      owner: '李川',
      title: '经营计划复盘改进',
      nextAction: '落实未达成目标的改进措施',
    })
  if (action === 'nonrenewal')
    Object.assign(v, {
      followup: true,
      title: '未续约合同退租处置',
      owner: '李川',
      nextAction: '确认未续约范围并排期回收',
    })
  if (action === 'triage') Object.assign(v, { result: '关联已有事项', workId: 'CS-103', followup: false })
  if (action === 'create-rule') Object.assign(v, { trigger: '合同到期', days: '60', owner: '客户负责人' })
  if (action === 'workflow-publish') v.scope = '仅新建事项'
  if (action === 'sla')
    Object.assign(v, {
      responseMinutes: String(s.sla.responseMinutes),
      recoveryHours: String(s.sla.recoveryHours),
      calendar: s.sla.calendar,
      pause: s.sla.pause,
    })
  if (action === 'share')
    Object.assign(v, {
      workId: 'CS-104',
      contactId: 'CT-02',
      expires: '2026-10-10',
      fields: '事项标题,交付范围,处理进度,共享附件',
      attachments: 'INST-078,ACC-041',
    })
  if (action === 'save-view')
    Object.assign(v, {
      name: '我的续约跟进',
      search: '',
      kind: 'renewal',
      owner: '张敏',
      density: '标准',
      columns: '客户,负责人,期限,下一步',
    })
  return v
}
