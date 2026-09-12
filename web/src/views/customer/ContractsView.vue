<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import SearchField from '@/components/ui/SearchField.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import SourceRecords from '@/components/customer/SourceRecords.vue'
const store = useCustomerStore(),
  route = useRoute(),
  router = useRouter(),
  actions = useCustomerActions(),
  query = ref(''),
  selected = ref('')
const contracts = computed(() =>
  store.snapshot.sources.filter(
    (s) =>
      s.kind === 'contract' &&
      (!route.query.customer || s.customerId === route.query.customer) &&
      (!query.value || `${s.id}${s.title}`.includes(query.value)),
  ),
)
function renew(id: string, customerId: string) {
  const existing = store.snapshot.work.find(
    (w) => w.kind === 'renewal' && w.evidenceIds.includes(id) && w.status !== '已结束',
  )
  if (existing) void router.push(`/customers/work/${existing.id}`)
  else
    actions.open('create-work', customerId, [], {
      kind: 'renewal',
      customerId,
      sourceId: id,
      title: `${id} 合同续约`,
      nextAction: '确认续约意向、范围与商务条件',
      criteria: '同一原合同的续约合同已生效',
    })
}
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="合同与续约"
      breadcrumb="租赁管理"
      description="从客户经营视角查看合同，推进续约，不另建合同账本"
    />
    <div class="metric-grid">
      <MetricCard
        label="有效合同投影"
        :value="contracts.filter((c) => c.verified).length"
        icon="file"
        caption="当前筛选下已核验记录"
      /><MetricCard
        label="续约推进中"
        :value="store.openWork.filter((w) => w.kind === 'renewal').length"
        icon="refresh"
        tone="orange"
        caption="跨客户未结束续约事项"
      /><MetricCard
        label="续约成功"
        :value="store.snapshot.work.filter((w) => w.kind === 'renewal' && w.resolution === '成功').length"
        icon="checks"
        tone="green"
        caption="有生效合同依据"
      /><MetricCard
        label="未达成"
        :value="store.snapshot.work.filter((w) => w.kind === 'renewal' && w.resolution === '未达成').length"
        icon="warning"
        tone="purple"
        caption="不计入成功续约"
      />
    </div>
    <CustomerSection title="客户合同记录" icon="file"
      ><div class="query-bar">
        <SearchField v-model="query" label="搜索合同" placeholder="搜索合同编号、标题…" /><button
          class="btn"
          @click="query = ''"
        >
          重置
        </button>
      </div>
      <div class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>合同</th>
              <th>客户</th>
              <th>合同有效期</th>
              <th class="amount">金额（CNY）</th>
              <th>状态</th>
              <th>关联事项</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in contracts" :key="c.id">
              <td>
                <strong>{{ c.id }}</strong>
                <p class="subline">{{ c.title }}</p>
              </td>
              <td>{{ store.customerName(c.customerId) }}</td>
              <td>
                {{ c.facts.start || '待确认' }}
                <p class="subline">至 {{ c.facts.end || '待确认' }}</p>
              </td>
              <td class="amount">
                {{ typeof c.facts.amount === 'number' ? c.facts.amount.toLocaleString() : '—' }}
              </td>
              <td><StatusBadge :text="c.state" :tone="c.verified ? 'success' : 'neutral'" /></td>
              <td>{{ store.snapshot.work.filter((w) => w.evidenceIds.includes(c.id)).length }} 项</td>
              <td>
                <div class="table-actions">
                  <button class="btn-link" @click="selected = c.id">查看</button
                  ><button
                    v-if="c.verified && !c.facts.renewal"
                    class="btn-link"
                    @click="renew(c.id, c.customerId)"
                  >
                    推进续约
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <CustomerAlert
        title="优先复用本次合同周期的续约事项"
        description="重复点击推进续约会进入已有事项，而不是每天生成新的续约工作。合同生效状态仍以来源模块为准。" /></CustomerSection
    ><CustomerSection v-if="selected" title="合同来源详情" icon="link"
      ><template #action><button class="btn-link" @click="selected = ''">收起</button></template
      ><SourceRecords :records="contracts.filter((c) => c.id === selected)"
    /></CustomerSection>
  </div>
</template>
