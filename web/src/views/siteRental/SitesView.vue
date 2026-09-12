<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import {
  rentalState,
  reviewPeriod,
  ruleForSite,
  modeNames,
  summary,
  siteWorkIds,
  currentDeployments,
  periodEnd,
  money,
} from '@/services/siteRental/model'
import { quoteRental } from '@/services/siteRental/quote'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SiteFormDialog from '@/components/siteRental/SiteFormDialog.vue'
import '@/styles/siteRental.css'
const store = useCustomerStore(),
  route = useRoute(),
  router = useRouter(),
  state = computed(() => rentalState(store.snapshot)),
  period = ref(reviewPeriod),
  search = ref(''),
  customer = ref(String(route.query.customer || '')),
  mode = ref(''),
  view = ref('全部点位'),
  applied = ref({ search: '', customer: String(route.query.customer || ''), mode: '' }),
  open = ref(false),
  page = ref(1),
  pageSize = ref(10)
const rows = computed(() =>
  state.value.profiles.filter(
    (x) =>
      (!applied.value.customer || x.customerId === applied.value.customer) &&
      (!applied.value.search || `${x.name}${x.id}${x.address}`.includes(applied.value.search)) &&
      (!applied.value.mode ||
        ruleForSite(state.value, x.id, period.value, store.snapshot.sources)?.mode === applied.value.mode) &&
      (view.value !== '待验收' || x.phase === '安装验收中') &&
      (view.value !== '共享额度' ||
        ruleForSite(state.value, x.id, period.value, store.snapshot.sources)?.scope === 'shared') &&
      (view.value !== '我的点位' || x.owner === '张敏'),
  ),
)
const shown = computed(() => rows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const sites = computed(() => state.value.profiles.filter((x) => x.kind === 'site'))
function query() {
  applied.value = { search: search.value, customer: customer.value, mode: mode.value }
  page.value = 1
}
function reset() {
  search.value = ''
  customer.value = ''
  mode.value = ''
  query()
}
watch(view, () => (page.value = 1))
watch(
  () => route.query.action,
  (a) => {
    if (a === 'new-site') {
      open.value = true
      void router.replace({ path: route.path, query: { customer: route.query.customer } })
    }
  },
  { immediate: true },
)
function rent(id: string) {
  return ruleForSite(state.value, id, period.value, store.snapshot.sources)
}
function cost(id: string) {
  const rule = rent(id)
  if (!rule) return '本期无有效规则'
  if (rule.scope === 'shared') return '计费组核算'
  const q = quoteRental(store.snapshot, state.value, rule, period.value)
  return money(q.lines.find((x) => x.key === id)?.totalCents)
}
function usage(id: string) {
  const u = state.value.usage.find((x) => x.siteId === id && x.period === period.value)
  return u?.quality === '完整' ? `${(u.raw - u.excluded).toLocaleString()} 杯` : '待核对'
}
</script>
<template>
  <div class="rental-area page-stack">
    <PageHeading
      title="点位租赁"
      breadcrumb="点位管理"
      description="以点位管理履约与数据，以客户管理经营关系"
      banner
    />
    <div class="metric-grid">
      <MetricCard label="实际服务点位" :value="sites.length" icon="site" tone="blue" /><MetricCard
        label="服务中点位"
        :value="sites.filter((x) => x.phase === '服务中').length"
        icon="success"
        tone="green"
      /><MetricCard
        label="共享额度点位"
        :value="sites.filter((x) => rent(x.id)?.scope === 'shared').length"
        icon="layers"
        tone="purple"
      /><MetricCard
        label="待起租验收"
        :value="sites.filter((x) => x.phase === '安装验收中').length"
        icon="clock"
        tone="orange"
      />
    </div>
    <div class="rental-toolbar">
      <div class="customer-tabs" aria-label="点位工作视图">
        <button
          v-for="tab in ['全部点位', '我的点位', '待验收', '共享额度']"
          :key="tab"
          :class="{ active: view === tab }"
          @click="view = tab"
        >
          {{ tab }}
        </button>
      </div>
      <div class="rental-actions">
        <RouterLink to="/sites/groups" class="btn">计费规则与共享组</RouterLink
        ><button class="btn btn-primary" @click="open = true">
          <AppIcon name="plus" :size="16" />新建点位
        </button>
      </div>
    </div>
    <form class="card rental-filters" @submit.prevent="query">
      <label class="rental-search"
        ><AppIcon name="search" :size="16" /><input
          v-model="search"
          aria-label="搜索点位"
          placeholder="搜索点位、编号或地址" /></label
      ><select v-model="customer" aria-label="筛选客户">
        <option value="">全部客户</option>
        <option v-for="c in store.snapshot.customers" :key="c.id" :value="c.id">{{ c.name }}</option></select
      ><select v-model="mode" aria-label="筛选计费模式">
        <option value="">全部计费模式</option>
        <option v-for="(name, id) in modeNames" :key="id" :value="id">{{ name }}</option></select
      ><input v-model="period" aria-label="查看账期" type="month" /><button
        class="btn btn-primary"
        type="submit"
      >
        查询</button
      ><button class="btn" type="button" @click="reset">重置</button>
    </form>
    <section class="card">
      <div class="rental-table-caption">
        <span>点位主档 · {{ rows.length }} 项</span><span>示例账期 {{ period }} · CNY · Asia/Shanghai</span>
      </div>
      <div class="table-scroll">
        <table class="data-table rental-site-table">
          <thead>
            <tr>
              <th>点位 / 所属客户</th>
              <th>运营与服务</th>
              <th>投放 / 责任人</th>
              <th>计费约定</th>
              <th>本期可计费用量</th>
              <th>预计费用</th>
              <th>待处理 / 下一步</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="site in shown" :key="site.id">
              <td>
                <RouterLink :to="`/sites/${site.id}`" class="rental-name">{{ site.name }}</RouterLink
                ><small>{{ store.customerName(site.customerId) }}</small
                ><small>{{ site.parent }} · {{ site.id }}</small>
              </td>
              <td>
                <StatusBadge
                  :text="site.kind === 'group' ? '组织分组' : site.phase"
                  :tone="site.phase === '服务中' ? 'success' : 'warning'"
                /><small>{{ site.operation }} · {{ site.service }}</small>
              </td>
              <td>
                <strong>{{ currentDeployments(state, site.id, periodEnd(period)).length }} 台</strong
                ><small>{{ site.owner }}</small>
              </td>
              <td>
                <template v-if="rent(site.id)"
                  ><StatusBadge :text="modeNames[rent(site.id)!.mode]" tone="primary" /><small>{{
                    summary(rent(site.id)!)
                  }}</small
                  ><small>{{
                    rent(site.id)!.scope === 'shared' ? '同合同共享额度' : '点位独立计算'
                  }}</small></template
                ><span v-else class="secondary">本期无有效规则 · 待核对</span>
              </td>
              <td class="rental-number">{{ usage(site.id) }}</td>
              <td>
                <strong>{{ cost(site.id) }}</strong
                ><small v-if="rent(site.id)?.scope === 'shared'">只计用量贡献</small
                ><small v-else>非正式应收</small>
              </td>
              <td>
                <strong
                  >{{
                    siteWorkIds(store.snapshot, site.id).filter(
                      (id) => store.snapshot.work.find((w) => w.id === id)?.status !== '已结束',
                    ).length
                  }}
                  项未结束</strong
                ><small>{{ site.nextAction }}</small
                ><small>{{ site.nextAt || '尚未安排' }}</small>
              </td>
              <td><RouterLink :to="`/sites/${site.id}`" class="btn-link">进入点位</RouterLink></td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState
        v-if="!rows.length"
        title="没有符合条件的点位"
        description="调整筛选条件，或为客户建立实际服务点位。"
      /><AppPagination v-model:page="page" v-model:page-size="pageSize" :total="rows.length" />
    </section>
    <p class="rental-help">
      共享额度只在指定计费组核算；分组节点不承载用量，不重复统计。列表不是设备或订单管理页。
    </p>
    <SiteFormDialog :open="open" @close="open = false" @saved="(id) => router.push(`/sites/${id}`)" />
  </div>
</template>
