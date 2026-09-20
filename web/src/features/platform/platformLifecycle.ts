export type LifecycleTone = 'success' | 'warning' | 'danger' | 'neutral' | 'primary'

export type LifecycleMetric = {
  label: string
  value: string
  caption: string
  icon: string
  tone?: string
}

export type LifecycleRow = {
  id: string
  name: string
  status: string
  tone: LifecycleTone
  cells: string[]
  summary: string
  evidence: string[]
}

export type LifecyclePageConfig = {
  key: string
  title: string
  description: string
  primaryAction: string
  authority: string
  metrics: LifecycleMetric[]
  stages: string[]
  columns: string[]
  rows: LifecycleRow[]
}

const commonAuthority = '当前页面用于平台业务管理。尚未开放的操作仅展示流程；正式操作以实际处理结果为准。'

export const platformLifecyclePages: Record<string, LifecyclePageConfig> = {
  features: {
    key: 'features',
    title: '商业功能',
    description: '把基础能力组合成可售卖、可定价、可被套餐引用的商业功能。',
    primaryAction: '新建商业功能',
    authority: commonAuthority,
    metrics: [
      { label: '商业功能', value: '18', caption: '覆盖 7 个业务域', icon: 'layers' },
      { label: '可售卖', value: '12', caption: '允许套餐或增购项引用', icon: 'success', tone: 'green' },
      { label: '基础能力', value: '4', caption: '默认随基础套餐开通', icon: 'crown', tone: 'purple' },
      { label: '待发布', value: '2', caption: '仍处于定价与依赖校验', icon: 'clock', tone: 'orange' },
    ],
    stages: ['基础能力', '商业定义', '定价', '套餐引用', '发布'],
    columns: ['所属模块', '定价', '套餐引用', '销售状态'],
    rows: [
      { id: 'feature-customer', name: '客户经营基础', status: '可售卖', tone: 'success', cells: ['客户管理', '基础价 ¥1,200', '4 个套餐', '在售'], summary: '客户档案、联系人、客户状态与基础经营视图。', evidence: ['客户查看、创建与修改', '包含客户额度', '依赖企业基础能力'] },
      { id: 'feature-marketing', name: '营销活动', status: '可售卖', tone: 'success', cells: ['营销', '¥0.10 / 营销出杯', '2 个套餐', '在售'], summary: '客户绑定、营销活动与按营销渠道出杯计费。', evidence: ['支持营销活动查看、创建与发布', '需要绑定客户', '营销出杯按量计费'] },
      { id: 'feature-analytics', name: '高级经营分析', status: '待发布', tone: 'warning', cells: ['经营分析', '升级价 ¥800', '1 个草稿套餐', '草稿'], summary: '面向经营者的高级指标、利润和趋势分析。', evidence: ['提供高级经营分析', '依赖基础报表', '发布前需完成价格审批'] },
      { id: 'feature-api', name: '开放 API', status: '停售', tone: 'neutral', cells: ['平台集成', '¥500 / 月', '历史 2 个套餐', '停售'], summary: '租户对外 API 与集成能力，现阶段停止新售。', evidence: ['保留开放接口读写能力', '历史租户继续保留', '禁止新套餐引用'] },
    ],
  },
  subscriptions: {
    key: 'subscriptions',
    title: '租户订阅',
    description: '集中管理租户套餐订阅、生效周期、续费策略和订阅状态。',
    primaryAction: '创建订阅',
    authority: commonAuthority,
    metrics: [
      { label: '有效订阅', value: '486', caption: 'ACTIVE / TRIAL', icon: 'company' },
      { label: '30 天内续费', value: '37', caption: '需要跟进的续费窗口', icon: 'calendar', tone: 'orange' },
      { label: '宽限期', value: '8', caption: '仍保留受限访问', icon: 'clock', tone: 'purple' },
      { label: '已暂停', value: '5', caption: '欠费或平台风控', icon: 'warning', tone: 'orange' },
    ],
    stages: ['下单 / 开通', '生效', '续费', '套餐变更', '终止'],
    columns: ['当前套餐', '有效期', '续费策略', '订阅状态'],
    rows: [
      { id: 'sub-shanghai', name: '上海咖啡科技有限公司', status: '有效', tone: 'success', cells: ['租赁专业版 V3', '2026-01-01 ~ 2026-12-31', '自动续费', '有效'], summary: '主订阅正常，当前无未完成降级。', evidence: ['订阅编号 SUB-2026-0188', '当前套餐：租赁专业版 V3', '自动续费'] },
      { id: 'sub-hangzhou', name: '杭州智饮运营', status: '待续费', tone: 'warning', cells: ['租赁基础版 V2', '2025-10-01 ~ 2026-10-01', '人工续费', '待续费'], summary: '距离到期不足 30 天，需要确认续费目标套餐。', evidence: ['订阅编号 SUB-2025-0932', '人工续费', '续费提醒已发送'] },
      { id: 'sub-suzhou', name: '苏州园区咖啡', status: '宽限期', tone: 'warning', cells: ['专业版 V2', '已于 2026-09-10 到期', '7 天宽限', '宽限期'], summary: '写操作将按宽限策略受限，数据仍保留。', evidence: ['宽限至 2026-09-17', '部分修改操作受限', '历史数据继续保留'] },
      { id: 'sub-demo', name: '华东渠道演示租户', status: '试用', tone: 'primary', cells: ['试用套餐 V1', '2026-09-01 ~ 2026-09-30', '不自动续费', '试用'], summary: '试用租户，不参与正式账单。', evidence: ['试用来源', '不自动续费', '等待转正式套餐'] },
    ],
  },
  authorization: {
    key: 'authorization',
    title: '授权诊断',
    description: '按业务规则检查企业状态、功能权益、角色、数据范围与额度结果。',
    primaryAction: '新建诊断',
    authority: commonAuthority,
    metrics: [
      { label: '今日诊断', value: '1,248', caption: '权限决策样本', icon: 'activity' },
      { label: '允许', value: '1,086', caption: '当前允许比例 87.0%', icon: 'success', tone: 'green' },
      { label: '权益拒绝', value: '91', caption: '租户未开通对应能力', icon: 'shield', tone: 'orange' },
      { label: '额度拒绝', value: '24', caption: '额度已用完', icon: 'warning', tone: 'orange' },
    ],
    stages: ['业务请求', '企业状态', '功能权益', '角色权限', '数据范围', '额度', '判断结果'],
    columns: ['功能能力', '角色', '最终结果', '原因'],
    rows: [
      { id: 'auth-001', name: '上海咖啡科技 / Alice Chen', status: '允许', tone: 'success', cells: ['创建客户', '运营负责人', '允许', '权益与额度均满足'], summary: '请求通过全部门禁，可创建客户。', evidence: ['企业状态正常', '已开通客户管理', '客户额度已用 76 / 100'] },
      { id: 'auth-002', name: '杭州智饮 / 李四', status: '拒绝', tone: 'danger', cells: ['发布营销活动', '门店运营', '拒绝', '未开通营销功能'], summary: '角色具备操作权限，但租户套餐未开通营销能力。', evidence: ['角色具备操作权限', '当前套餐未开通营销功能', '按当前权益结果拒绝'] },
      { id: 'auth-003', name: '南京租赁 / 王五', status: '拒绝', tone: 'warning', cells: ['邀请成员', '企业管理员', '拒绝', '成员额度已用完'], summary: '租户和角色通过，但成员额度已经使用完。', evidence: ['成员额度 20 / 20', '可通过增购扩容', '扩容后可重新提交'] },
      { id: 'auth-004', name: '苏州园区 / API Client', status: '受限', tone: 'warning', cells: ['开放接口写入', '接口账号', '拒绝', '企业处于宽限期'], summary: '租户处于宽限期，策略仅允许读取。', evidence: ['企业处于宽限期', '允许读取', '写入受限'] },
    ],
  },
  quotas: {
    key: 'quotas',
    title: '额度管理',
    description: '用授予、消费、释放、增购和账本管理租户客户/成员/设备等可计数权益。',
    primaryAction: '新增额度授予',
    authority: commonAuthority,
    metrics: [
      { label: '额度类型', value: '6', caption: '客户 / 成员 / 设备 / 点位等', icon: 'database' },
      { label: '高使用率租户', value: '21', caption: '使用率 ≥ 85%', icon: 'warning', tone: 'orange' },
      { label: '今日消费', value: '328', caption: '来自业务写操作', icon: 'activity', tone: 'purple' },
      { label: '今日释放', value: '46', caption: '删除或解绑后回收', icon: 'refresh', tone: 'green' },
    ],
    stages: ['额度定义', '授予', '占用', '释放', '增购 / 过期', '额度流水'],
    columns: ['额度类型', '已用 / 总额', '来源', '状态'],
    rows: [
      { id: 'quota-customer', name: '上海咖啡科技有限公司', status: '正常', tone: 'success', cells: ['客户额度', '76 / 100', '专业版 V3', 'AVAILABLE'], summary: '客户额度剩余 24，可继续创建客户。', evidence: ['套餐授予 100', '已使用 76', '剩余 24'] },
      { id: 'quota-member', name: '杭州智饮运营', status: '临界', tone: 'warning', cells: ['成员额度', '19 / 20', '基础版 + Add-on', 'NEAR_LIMIT'], summary: '仅剩 1 个成员额度，建议提前增购。', evidence: ['套餐包含 10', '增购 10', '已使用 19'] },
      { id: 'quota-device', name: '南京租赁服务', status: '已耗尽', tone: 'danger', cells: ['设备额度', '500 / 500', '租赁专业版 V2', 'EXHAUSTED'], summary: '新设备绑定将被运行时门禁拒绝。', evidence: ['授予 500', '已使用 500', '下一步可购买额度包'] },
      { id: 'quota-site', name: '苏州园区咖啡', status: '正常', tone: 'success', cells: ['点位额度', '42 / 80', '专业版 V2', 'AVAILABLE'], summary: '点位额度余量充足。', evidence: ['授予 80', '已使用 42', '剩余 38'] },
    ],
  },
  'usage-billing': {
    key: 'usage-billing',
    title: '用量计费',
    description: '管理按量事件、用量汇总、费率计算和账单生成，重点覆盖营销出杯计费。',
    primaryAction: '创建计量规则',
    authority: commonAuthority,
    metrics: [
      { label: '本月计量事件', value: '1.82M', caption: '去重后的用量事件', icon: 'activity' },
      { label: '营销出杯', value: '428K', caption: 'marketing.cup', icon: 'coffee', tone: 'purple' },
      { label: '待出账金额', value: '¥42.8K', caption: '按当前固定费率估算', icon: 'file', tone: 'orange' },
      { label: '异常计量项', value: '3', caption: '需要人工核验', icon: 'warning', tone: 'orange' },
    ],
    stages: ['用量事件', '计量项', '汇总', '费率计算', '账单记录'],
    columns: ['计量项', '本期用量', '费率', '计费金额'],
    rows: [
      { id: 'meter-marketing-sh', name: '上海咖啡科技有限公司', status: '计量中', tone: 'primary', cells: ['marketing.cup', '234,500 杯', '¥0.10 / 杯', '¥23,450'], summary: '营销渠道出杯按固定单价累计，月末进入正式账单。', evidence: ['来源：营销渠道', '重复事件已去重', '按固定单价计费'] },
      { id: 'meter-marketing-hz', name: '杭州智饮运营', status: '计量中', tone: 'primary', cells: ['marketing.cup', '128,060 杯', '¥0.10 / 杯', '¥12,806'], summary: '当前聚合无异常。', evidence: ['计量状态正常', '按日汇总', '等待出账'] },
      { id: 'meter-api', name: '开放 API 使用量', status: '观察', tone: 'warning', cells: ['api.request', '2.8M 次', '套餐内含', '¥0'], summary: '当前仅计量用于容量规划，不产生额外费用。', evidence: ['仅用于容量统计', '不产生额外费用', '保留 180 天'] },
      { id: 'meter-anomaly', name: '南京租赁服务', status: '异常', tone: 'danger', cells: ['marketing.cup', '18,420 杯', '¥0.10 / 杯', '待核验'], summary: '发现重复事件比例异常，暂缓出账。', evidence: ['重复事件比例 3.8%', '当前暂停出账', '需要人工核验'] },
    ],
  },
  changes: {
    key: 'changes',
    title: '套餐变更',
    description: '统一管理升级、降级、续费和权益变化，确保价格与变更条件清晰可见。',
    primaryAction: '发起套餐变更',
    authority: commonAuthority,
    metrics: [
      { label: '处理中', value: '14', caption: '等待确认或审批', icon: 'refresh' },
      { label: '升级', value: '9', caption: '只收新增模块价格', icon: 'up', tone: 'green' },
      { label: '待降级合规', value: '3', caption: '额度使用量高于目标套餐', icon: 'warning', tone: 'orange' },
      { label: '今日生效', value: '6', caption: '处理结果已确认', icon: 'success', tone: 'purple' },
    ],
    stages: ['变更申请', '权益变化', '价格计算', '条件校验', '审批 / 确认', '生效'],
    columns: ['当前 → 目标', '权益变化', '本次价格', '状态'],
    rows: [
      { id: 'change-upgrade', name: '上海咖啡科技有限公司', status: '待确认', tone: 'warning', cells: ['专业版 V3 → 旗舰版 V1', '+ 营销 / 高级分析', '¥1,300', '待确认'], summary: '升级费用只包含新增商业功能，不重复收取已有模块。', evidence: ['新增营销 ¥500', '新增高级分析 ¥800', '已有功能不重复收费'] },
      { id: 'change-downgrade', name: '杭州智饮运营', status: '待合规', tone: 'danger', cells: ['专业版 V2 → 基础版 V2', '成员 100 → 50', '下周期生效', '待满足条件'], summary: '当前成员 82，高于目标额度 50，不能直接降级。', evidence: ['当前成员 82', '目标额度 50', '生效前需减少 32'] },
      { id: 'change-renew', name: '南京租赁服务', status: '已批准', tone: 'success', cells: ['专业版 V2 → 专业版 V3', '模块不变 / 价格更新', '¥6,800 / 年', '已批准'], summary: '续费并迁移到新版本套餐。', evidence: ['套餐版本保持可追溯', '续费已批准', '下一周期生效'] },
      { id: 'change-addon', name: '苏州园区咖啡', status: '已生效', tone: 'success', cells: ['专业版 V2 + 增购项', '+100 客户额度', '¥600', '已生效'], summary: '通过增购项增加客户额度，不改变主套餐。', evidence: ['增购客户额度 +100', '额度流水增加 100', '处理结果已确认'] },
    ],
  },
  'add-ons': {
    key: 'add-ons',
    title: '增购项',
    description: '管理可独立购买的功能与额度包，作为套餐之外的商业权益来源。',
    primaryAction: '新建增购项',
    authority: commonAuthority,
    metrics: [
      { label: '在售增购项', value: '11', caption: '功能包与额度包', icon: 'plus' },
      { label: '额度包', value: '6', caption: '客户 / 成员 / 设备等', icon: 'database', tone: 'green' },
      { label: '功能包', value: '5', caption: '营销 / API / 高级分析等', icon: 'layers', tone: 'purple' },
      { label: '本月购买', value: '84', caption: '租户自主增购与平台代开', icon: 'chart', tone: 'orange' },
    ],
    stages: ['商品定义', '定价', '购买', '权益授予', '到期 / 续购'],
    columns: ['类型', '价格', '授予内容', '销售状态'],
    rows: [
      { id: 'addon-customer100', name: '客户额度 +100', status: '在售', tone: 'success', cells: ['额度包', '¥600 / 年', '客户额度 +100', '在售'], summary: '适用于客户额度不足的租赁经销商。', evidence: ['客户额度增加 100', '与当前订阅周期一致', '支持续购'] },
      { id: 'addon-member10', name: '成员额度 +10', status: '在售', tone: 'success', cells: ['额度包', '¥240 / 年', '成员额度 +10', '在售'], summary: '为租户追加成员席位。', evidence: ['成员额度增加 10', '仅当前企业可用', '释放后额度可再次使用'] },
      { id: 'addon-marketing', name: '营销功能包', status: '在售', tone: 'success', cells: ['功能包', '¥500 开通 + 按量', '开通营销能力', '在售'], summary: '开通营销能力，并启用营销出杯 Meter。', evidence: ['包含营销功能', '营销出杯按量计费', '需要绑定客户'] },
      { id: 'addon-api', name: '开放 API', status: '停售', tone: 'neutral', cells: ['功能包', '¥500 / 月', '开放接口能力', '停售'], summary: '历史租户继续使用，不接受新购买。', evidence: ['历史授权继续保留', '停止新购', '等待替代方案'] },
    ],
  },
  overrides: {
    key: 'overrides',
    title: '专项授权',
    description: '管理平台赠送、临时开通、强制禁用等专项授权，并保留完整原因与期限。',
    primaryAction: '创建专项授权',
    authority: commonAuthority,
    metrics: [
      { label: '生效中', value: '23', caption: '当前生效的专项授权', icon: 'shield' },
      { label: '7 天内到期', value: '6', caption: '需要续期或回收确认', icon: 'clock', tone: 'orange' },
      { label: '赠送开通', value: '15', caption: '客户补偿 / 商务特批', icon: 'success', tone: 'green' },
      { label: '强制禁用', value: '2', caption: '平台风险控制', icon: 'lock', tone: 'purple' },
    ],
    stages: ['申请', '审批', '生效', '过期 / 撤销', '操作记录'],
    columns: ['目标权益', '授权结果', '有效期', '状态'],
    rows: [
      { id: 'override-marketing', name: '上海咖啡科技有限公司', status: '生效中', tone: 'success', cells: ['营销功能', '允许', '2026-09-01 ~ 2026-10-01', '生效中'], summary: '商务补偿临时赠送营销能力。', evidence: ['原因：客诉补偿', '平台管理员已批准', '到期后自动失效'] },
      { id: 'override-api', name: '杭州智饮运营', status: '即将到期', tone: 'warning', cells: ['开放接口读取', '允许', '至 2026-09-20', '即将到期'], summary: '临时 API 读取能力将在 5 天后到期。', evidence: ['临时集成用途', '不包含写入能力', '已安排到期提醒'] },
      { id: 'override-block', name: '风险租户 0031', status: '强制禁用', tone: 'danger', cells: ['发布营销活动', '拒绝', '长期 / 人工解除', '生效中'], summary: '平台风险策略覆盖套餐授权，禁止发布营销活动。', evidence: ['风险事件 RISK-031', '强制禁用优先', '仅可人工解除'] },
      { id: 'override-expired', name: '南京租赁服务', status: '已过期', tone: 'neutral', cells: ['高级经营分析', '允许', '已于 2026-09-12 到期', '已过期'], summary: '试用高级分析结束，已回到套餐原始权益。', evidence: ['已自动到期', '当前权益已重新计算', '操作记录继续保留'] },
    ],
  },
  expiry: {
    key: 'expiry',
    title: '到期与宽限',
    description: '定义订阅到期后的提醒、宽限、只读、暂停与终止策略，避免简单粗暴地直接关闭。',
    primaryAction: '配置到期策略',
    authority: commonAuthority,
    metrics: [
      { label: '30 天内到期', value: '37', caption: '需进入续费提醒', icon: 'calendar' },
      { label: '宽限期租户', value: '8', caption: '按策略保留受限访问', icon: 'clock', tone: 'orange' },
      { label: '只读状态', value: '5', caption: '数据可读 / 写操作禁止', icon: 'eye', tone: 'purple' },
      { label: '待终止', value: '2', caption: '宽限结束等待最终确认', icon: 'warning', tone: 'orange' },
    ],
    stages: ['即将到期', '宽限期', '只读 / 暂停', '已过期', '已终止'],
    columns: ['到期时间', '宽限策略', '写操作策略', '当前状态'],
    rows: [
      { id: 'expiry-hz', name: '杭州智饮运营', status: '即将到期', tone: 'warning', cells: ['2026-10-01', '7 天', '到期前正常', '即将到期'], summary: '已进入 30 天续费提醒窗口。', evidence: ['续费负责人已收到提醒', '当前访问不受限', '人工续费'] },
      { id: 'expiry-sz', name: '苏州园区咖啡', status: '宽限期', tone: 'warning', cells: ['2026-09-10', '7 天 / 至 09-17', '禁止新建与修改', '宽限期'], summary: '保留数据读取，禁止写操作与 API write。', evidence: ['当前仅可读取', '数据继续保留', '账单提醒已生效'] },
      { id: 'expiry-risk', name: '历史测试租户 018', status: '已暂停', tone: 'danger', cells: ['2026-08-31', '已结束', '全部业务写入禁止', '已暂停'], summary: '宽限期结束，等待续费或终止确认。', evidence: ['宽限期已结束', '账单负责人仍可登录', '业务操作已受限'] },
      { id: 'expiry-closed', name: '已退租客户样例', status: '已终止', tone: 'neutral', cells: ['2026-06-30', '完成', '不可执行', '已终止'], summary: '订阅已终止，历史商业事实与审计继续保留。', evidence: ['订阅已终止', '按保留策略保存数据', '操作记录继续保留'] },
    ],
  },
  audit: {
    key: 'audit',
    title: '商业审计',
    description: '追踪平台侧套餐、订阅、权益、额度、计费与专项授权的关键商业变更。',
    primaryAction: '导出审计记录',
    authority: commonAuthority,
    metrics: [
      { label: '今日事件', value: '2,846', caption: '平台商业域变更记录', icon: 'file' },
      { label: '高风险操作', value: '7', caption: '强制 DENY / 终止 / 退款相关', icon: 'warning', tone: 'orange' },
      { label: '人工专项授权', value: '12', caption: '必须包含原因与操作者', icon: 'shield', tone: 'purple' },
      { label: '证据完整率', value: '99.8%', caption: 'before / after / reason / actor', icon: 'checks', tone: 'green' },
    ],
    stages: ['事件', '操作人', '对象', '变更前后', '依据', '保留'],
    columns: ['事件类型', '对象', '操作者', '结果'],
    rows: [
      { id: 'audit-001', name: '2026-09-15 19:42:18', status: '成功', tone: 'success', cells: ['套餐升级已批准', '上海咖啡科技', '平台管理员', '成功'], summary: '批准专业版 V3 → 旗舰版 V1 升级。', evidence: ['变更前：租赁专业版 V3', '变更后：旗舰版 V1', '原因：客户主动升级'] },
      { id: 'audit-002', name: '2026-09-15 18:07:31', status: '成功', tone: 'success', cells: ['成员额度增购已生效', '杭州智饮运营', '商务运营', '成功'], summary: '购买成员额度 +10 并写入 Quota Ledger。', evidence: ['本次增购已确认', '额度增加 10', '成员额度更新为 20'] },
      { id: 'audit-003', name: '2026-09-15 16:22:04', status: '高风险', tone: 'danger', cells: ['营销发布已强制禁用', '风险租户 0031', '风险管理员', '成功'], summary: '风险事件触发 marketing.publish 强制 DENY。', evidence: ['风险事件 RISK-031', '当前结果：拒绝', '需要人工解除'] },
      { id: 'audit-004', name: '2026-09-15 14:56:42', status: '冲突', tone: 'warning', cells: ['套餐变更', '南京租赁服务', '平台管理员', '冲突'], summary: '基于过期版本提交变更，被乐观锁阻止。', evidence: ['提交版本已过期', '当前版本已更新', '未产生成功结果'] },
    ],
  },
}

export const platformLifecycleKeys = Object.keys(platformLifecyclePages)
