<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { planResults } from '@/services/customer/selectors'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import SearchField from '@/components/ui/SearchField.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
const store = useCustomerStore(),
  actions = useCustomerActions(),
  route = useRoute(),
  query = ref(''),
  state = ref('')
const plans = computed(() =>
  store.snapshot.plans.filter(
    (p) =>
      (!route.query.customer || p.customerId === route.query.customer) &&
      (!query.value || `${p.title} ${store.customerName(p.customerId)}`.includes(query.value)) &&
      (!state.value || p.state === state.value),
  ),
)
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="经营计划"
      breadcrumb="客户运营"
      description="围绕客户阶段目标，组织里程碑、事项与结果复盘"
      ><div class="customer-heading-actions">
        <button class="btn btn-primary" @click="actions.open('create-plan')">新建经营计划</button>
      </div></PageHeading
    >
    <section class="card data-panel">
      <div class="query-bar">
        <SearchField v-model="query" label="搜索经营计划" placeholder="搜索计划、客户…" /><select
          v-model="state"
          class="select"
          aria-label="计划状态"
        >
          <option value="">全部状态</option>
          <option>进行中</option>
          <option>已结案</option></select
        ><button
          class="btn"
          @click="
            () => {
              query = ''
              state = ''
            }
          "
        >
          重置
        </button>
      </div>
    </section>
    <CustomerSection v-for="plan in plans" :key="plan.id" :title="plan.title" icon="calendar"
      ><template #action
        ><RouterLink :to="`/customers/plans/${plan.id}`" class="btn btn-primary"
          >进入计划</RouterLink
        ></template
      >
      <div class="customer-view-toolbar">
        <div class="row wrap">
          <RouterLink :to="`/customers/accounts/${plan.customerId}`" class="btn-link">{{
            store.customerName(plan.customerId)
          }}</RouterLink
          ><StatusBadge :text="plan.state" :tone="plan.state === '已结案' ? 'neutral' : 'primary'" /><span
            class="customer-help"
            >{{ plan.owner }} · {{ plan.start }} 至 {{ plan.end }}</span
          >
        </div>
        <span class="customer-help">{{ plan.id }} · {{ plan.milestones.length }} 个里程碑</span>
      </div>
      <div class="customer-mini-metrics">
        <div v-for="target in planResults(store.snapshot, plan)" :key="target.title">
          <small>{{ target.title }}</small
          ><strong
            >{{ target.actual }} / {{ target.target }} <small>{{ target.unit }}</small></strong
          ><small>{{ target.source }}</small>
        </div>
      </div>
      <p class="customer-help">
        {{ plan.conclusion || '执行进度与目标达成分别展示。计划结案不自动关闭未结束事项。' }}
      </p></CustomerSection
    ><EmptyState v-if="!plans.length"
      ><button
        class="btn"
        @click="
          () => {
            query = ''
            state = ''
          }
        "
      >
        清空筛选
      </button></EmptyState
    >
  </div>
</template>
