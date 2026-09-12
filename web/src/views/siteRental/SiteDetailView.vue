<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import {
  rentalState,
  reviewPeriod,
  ruleForSite,
  summary,
  currentDeployments,
  periodEnd,
  siteWorkIds,
  money,
} from '@/services/siteRental/model'
import { quoteRental } from '@/services/siteRental/quote'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import WorkTable from '@/components/customer/WorkTable.vue'
import ActivityList from '@/components/customer/ActivityList.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import SiteFormDialog from '@/components/siteRental/SiteFormDialog.vue'
import QuoteSummary from '@/components/siteRental/QuoteSummary.vue'
import '@/styles/siteRental.css'
const store = useCustomerStore(),
  actions = useCustomerActions(),
  route = useRoute(),
  period = ref(reviewPeriod),
  edit = ref(false),
  operationOpen = ref(false),
  reason = ref(''),
  nextAt = ref(''),
  error = ref(''),
  revision = ref(0)
const state = computed(() => rentalState(store.snapshot)),
  site = computed(() => state.value.profiles.find((x) => x.id === route.params.id)),
  tab = computed(() => String(route.query.tab || 'overview'))
const rule = computed(() =>
    site.value ? ruleForSite(state.value, site.value.id, period.value, store.snapshot.sources) : undefined,
  ),
  quote = computed(() =>
    rule.value ? quoteRental(store.snapshot, state.value, rule.value, period.value) : undefined,
  )
const deployments = computed(() => state.value.deployments.filter((x) => x.siteId === site.value?.id)),
  current = computed(() =>
    site.value ? currentDeployments(state.value, site.value.id, periodEnd(period.value)) : [],
  )
const work = computed(() =>
  store.snapshot.work.filter(
    (x) =>
      x.customerId === site.value?.customerId && siteWorkIds(store.snapshot, site.value!.id).includes(x.id),
  ),
)
const activity = computed(() =>
  store.snapshot.activities.filter(
    (x) => x.target === site.value?.id || work.value.some((w) => w.id === x.target),
  ),
)
const usage = computed(() =>
  state.value.usage.find((x) => x.siteId === site.value?.id && x.period === period.value),
)
const tabs = [
  ['overview', '概览'],
  ['placements', '设备与投放履历'],
  ['rental', '租约与结算'],
  ['usage', '使用数据'],
  ['service', '服务与事项'],
  ['activity', '活动记录'],
]
watch(
  () => route.params.id,
  () => {
    edit.value = false
    operationOpen.value = false
  },
)
function createWork() {
  if (!site.value) return
  actions.open('create-work', site.value.customerId, [], {
    customerId: site.value.customerId,
    siteId: site.value.id,
    title: `${site.value.name} · 服务跟进`,
    owner: site.value.owner,
    description: `关联点位 ${site.value.id}；不修改设备或计费来源。`,
    nextAction: site.value.nextAction,
    criteria: '核对本点位问题与处理结果，保留客户反馈',
  })
}
function openOperation() {
  store.refreshRental()
  revision.value = store.snapshot.revision
  operationOpen.value = true
  reason.value = ''
  nextAt.value = ''
  error.value = ''
}
function changeOperation() {
  try {
    if (!site.value) return
    store.runRental({
      tenant: store.snapshot.tenant,
      expectedRevision: revision.value,
      key: crypto.randomUUID(),
      action: {
        type: 'operation',
        siteId: site.value.id,
        operation: site.value.operation === '正常运营' ? '临时停用' : '正常运营',
        reason: reason.value,
        nextAt: nextAt.value,
      },
    })
    operationOpen.value = false
  } catch (e) {
    error.value = (e as Error).message
  }
}
</script>
<template>
  <div v-if="site" class="rental-area page-stack">
    <PageHeading
      title="点位工作区"
      breadcrumb="点位管理"
      description="现场、租约、服务和下一步行动，围绕同一个点位关联"
      ><div class="rental-actions">
        <RouterLink to="/sites" class="btn">返回点位列表</RouterLink
        ><button class="btn" @click="edit = true">编辑点位</button
        ><button v-if="site.kind === 'site'" class="btn btn-primary" @click="createWork">
          <AppIcon name="plus" :size="16" />新建关联事项
        </button>
      </div></PageHeading
    >
    <section class="card rental-identity">
      <div class="rental-site-mark"><AppIcon name="site" :size="30" /></div>
      <div class="rental-grow">
        <div class="rental-title-line">
          <h2>{{ site.name }}</h2>
          <StatusBadge
            :text="site.phase"
            :tone="site.phase === '服务中' ? 'success' : 'warning'"
          /><StatusBadge
            :text="site.operation"
            :tone="site.operation === '正常运营' ? 'neutral' : 'warning'"
          />
        </div>
        <p>
          <RouterLink :to="`/customers/accounts/${site.customerId}`" class="btn-link">{{
            store.customerName(site.customerId)
          }}</RouterLink>
          · {{ site.parent }} · {{ site.id }}
        </p>
        <small>{{ site.address }}</small>
      </div>
      <div>
        <span>点位负责人</span><strong>{{ site.owner }}</strong
        ><small>现场联系：{{ site.contact }}</small>
      </div>
    </section>
    <div class="rental-stat-strip">
      <div>
        <span>当前投放</span><strong>{{ current.length }} <small>台</small></strong>
      </div>
      <div>
        <span>本期可计费杯数</span
        ><strong
          >{{ usage?.quality === '完整' ? (usage.raw - usage.excluded).toLocaleString() : '—' }}
          <small>杯</small></strong
        >
      </div>
      <div>
        <span>{{ rule?.scope === 'shared' ? '所属计费组预计费用' : '本点位预计费用' }}</span
        ><strong>{{
          money(
            rule?.scope === 'shared'
              ? quote?.totalCents
              : quote?.lines.find((line) => line.key === site?.id)?.totalCents,
          )
        }}</strong
        ><small>{{ rule?.scope === 'shared' ? '非本点位独立应付' : '示例快照 · 非正式应收' }}</small>
      </div>
      <div>
        <span>未结束事项</span
        ><strong>{{ work.filter((w) => w.status !== '已结束').length }} <small>项</small></strong>
      </div>
    </div>
    <div class="rental-toolbar">
      <nav class="customer-tabs" aria-label="点位详情标签">
        <RouterLink
          v-for="[id, title] in tabs"
          :key="id"
          :class="{ active: tab === id }"
          :to="{ path: route.path, query: { tab: id } }"
          >{{ title }}</RouterLink
        >
      </nav>
      <input v-model="period" type="month" aria-label="点位账期" />
    </div>
    <template v-if="tab === 'overview'"
      ><div class="customer-split">
        <div class="page-stack">
          <CustomerSection title="当前服务与下一步" icon="checks"
            ><div class="rental-next-action">
              <div>
                <StatusBadge :text="site.service" :tone="site.service === '正常' ? 'success' : 'warning'" />
                <h3>{{ site.nextAction }}</h3>
                <p>{{ site.owner }} · {{ site.nextAt || '尚未安排核实时间' }}</p>
              </div>
              <button class="btn" @click="openOperation">
                {{ site.operation === '正常运营' ? '调整运营安排' : '登记恢复运营' }}
              </button>
            </div>
            <p class="rental-help">连接状态、制作能力与运营安排分别判断。暂停运营不会自动停租或关闭事项。</p>
            <WorkTable
              :items="work.filter((w) => w.status !== '已结束').slice(0, 3)"
              :view="{
                name: '',
                search: '',
                kind: '',
                owner: '',
                density: '标准',
                columns: ['负责人', '期限'],
              }" /></CustomerSection
          ><CustomerSection title="现场条件与责任分工" icon="site"
            ><dl class="rental-definition">
              <dt>开放时间</dt>
              <dd>{{ site.hours }}</dd>
              <dt>进场约束</dt>
              <dd>{{ site.access || '待补充' }}</dd>
              <dt>水 / 电 / 网络</dt>
              <dd>{{ site.water }} / {{ site.power }} / {{ site.network }}</dd>
              <dt>清洁责任</dt>
              <dd>{{ site.cleaning }}</dd>
              <dt>补货责任</dt>
              <dd>{{ site.supplies }}</dd>
            </dl></CustomerSection
          >
        </div>
        <div class="page-stack">
          <CustomerSection title="本期租赁约定" icon="file"
            ><template v-if="rule"
              ><h3>{{ summary(rule) }}</h3>
              <dl class="rental-definition">
                <dt>计费范围</dt>
                <dd>{{ rule.scope === 'shared' ? '同合同指定点位共享' : '点位独立计算' }}</dd>
                <dt>合同来源</dt>
                <dd>{{ rule.contractId }}</dd>
                <dt>合同到期</dt>
                <dd>
                  {{ store.snapshot.sources.find((x) => x.id === rule?.contractId)?.facts.end || '待核对' }}
                </dd>
                <dt>结算主体</dt>
                <dd>{{ rule.billingOwner }}</dd>
                <dt>规则版本</dt>
                <dd>v{{ rule.version }} · {{ rule.effectiveFrom }} 起</dd>
              </dl>
              <RouterLink :to="`/sites/groups/${rule.groupId}`" class="btn-link"
                >查看{{ rule.scope === 'shared' ? '共享组与额度' : '计费规则与版本' }} →</RouterLink
              ></template
            ><CustomerAlert
              v-else
              title="尚未配置本期计费规则"
              description="新建或待验收不自动开始计费。" /></CustomerSection
          ><CustomerSection title="现场联系人" icon="phone"
            ><h3>{{ site.contact }}</h3>
            <p>{{ site.phone || '待补充' }}</p>
            <p class="rental-help">联系人不等于有权查看客户内部数据；共享范围需单独授权。</p></CustomerSection
          >
        </div>
      </div></template
    >
    <CustomerSection v-else-if="tab === 'placements'" title="设备投放时间线" icon="device"
      ><CustomerAlert
        title="换机保持点位与历史连续"
        description="按投放有效期归属历史用量。结束的关系仅保留历史，不获得设备后续投放数据；本轮不执行远程换机。"
      />
      <div class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>投放关系</th>
              <th>设备 / 角色</th>
              <th>有效期间</th>
              <th>连接状态</th>
              <th>数据可见范围</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in deployments" :key="d.id">
              <td>{{ d.id }}</td>
              <td>
                <strong>{{ d.device }}</strong
                ><small>{{ d.model }} · {{ d.role }}</small>
              </td>
              <td>{{ d.from }} → {{ d.until || '当前有效' }}</td>
              <td>
                <StatusBadge
                  :text="d.until ? '历史记录' : d.online === null ? '待核实' : d.online ? '在线' : '离线'"
                  :tone="d.online ? 'success' : 'neutral'"
                />
              </td>
              <td>{{ d.until ? `仅保留 ${d.until} 之前的授权历史` : '当前投放期间；还需有效授权' }}</td>
            </tr>
          </tbody>
        </table>
      </div></CustomerSection
    >
    <template v-else-if="tab === 'rental'"
      ><CustomerSection v-if="rule && quote" title="有效租约与本期核算" icon="file"
        ><template #action
          ><RouterLink :to="`/sites/groups/${rule.groupId}`" class="btn-link"
            >规则版本与变更</RouterLink
          ></template
        ><QuoteSummary :quote="quote" :rule="rule" /><RouterLink
          :to="`/sites/statements?group=${rule.groupId}`"
          class="btn"
          >查看计费组对账记录</RouterLink
        ></CustomerSection
      ><CustomerAlert v-else title="本账期未配置有效租约" description="保留点位档案，不用零费用冒充未配置。"
        ><RouterLink to="/sites/groups?action=new-rule" class="btn">配置计费约定</RouterLink></CustomerAlert
      ></template
    >
    <CustomerSection v-else-if="tab === 'usage'" title="用量与数据口径" icon="chart"
      ><template v-if="usage"
        ><div class="rental-stat-strip">
          <div>
            <span>原始出杯</span><strong>{{ usage.raw }} <small>杯</small></strong>
          </div>
          <div>
            <span>合同口径排除</span><strong>{{ usage.excluded }} <small>杯</small></strong>
          </div>
          <div>
            <span>可计费杯数</span
            ><strong
              >{{ usage.quality === '完整' ? usage.raw - usage.excluded : '—' }} <small>杯</small></strong
            >
          </div>
          <div>
            <span>数据状态</span><strong>{{ usage.quality }}</strong>
          </div>
        </div>
        <p>{{ usage.exclusionNote }}</p>
        <p class="rental-help">
          更新时间：{{ usage.asOf }} · 来源版本 {{ usage.revision }}。未提供逐日明细，不生成虚构趋势。
        </p></template
      ><CustomerAlert
        v-else
        title="用量来源尚未提供"
        description="无数据不等于零杯；按杯与保底超量模式不可据此确认对账。"
        tone="warning"
    /></CustomerSection>
    <CustomerSection v-else-if="tab === 'service'" title="关联客户事项" icon="checks"
      ><template #action><button class="btn btn-primary" @click="createWork">创建关联事项</button></template>
      <p class="rental-help">同一事项同时出现在客户、点位和事项工作台，状态与负责人只维护一份。</p>
      <WorkTable :items="work"
    /></CustomerSection>
    <CustomerSection v-else title="点位活动与审计" icon="activity"
      ><ActivityList :items="activity"
    /></CustomerSection>
    <SiteFormDialog :open="edit" :site-id="site.id" @close="edit = false" />
    <UiDialog :open="operationOpen" title="调整点位运营安排" @close="operationOpen = false"
      ><form id="rental-operation" class="rental-form page-stack" @submit.prevent="changeOperation">
        <CustomerAlert
          title="运营停用与停租分开"
          description="本操作只登记运营安排。费用、额度、投放关系及数据授权保持原约定；账期结算时需核对停用影响。"
        /><label>调整原因 <b>*</b><textarea v-model="reason" required /></label
        ><label>下次核实时间 <b>*</b><input v-model="nextAt" type="datetime-local" required /></label>
        <p v-if="error" class="rental-error" role="alert">{{ error }}</p>
      </form>
      <template #footer
        ><button class="btn" @click="operationOpen = false">取消</button
        ><button class="btn btn-primary" type="submit" form="rental-operation">
          确认{{ site.operation === '正常运营' ? '临时停用' : '恢复运营' }}
        </button></template
      ></UiDialog
    >
  </div>
  <CustomerAlert v-else title="点位不存在或不在当前租户范围" tone="warning"
    ><RouterLink to="/sites" class="btn">返回点位列表</RouterLink></CustomerAlert
  >
</template>
