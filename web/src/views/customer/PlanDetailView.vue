<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { planResults } from '@/services/customer/selectors'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import WorkTable from '@/components/customer/WorkTable.vue'
import ActivityList from '@/components/customer/ActivityList.vue'
const store = useCustomerStore(),
  route = useRoute(),
  actions = useCustomerActions()
const plan = computed(() => store.snapshot.plans.find((p) => p.id === route.params.id))
const related = computed(() =>
  store.snapshot.work.filter(
    (w) => w.planId === plan.value?.id || plan.value?.milestones.some((m) => m.workIds.includes(w.id)),
  ),
)
const progress = computed(() =>
  plan.value
    ? Math.round(
        (plan.value.milestones.filter((m) => m.status === '已完成').length / plan.value.milestones.length) *
          100,
      )
    : 0,
)
</script>
<template>
  <div v-if="plan" class="page-stack">
    <PageHeading
      :title="plan.title"
      breadcrumb="客户运营 / 经营计划"
      :description="`${store.customerName(plan.customerId)} · ${plan.id} · ${plan.start} 至 ${plan.end}`"
      ><div class="customer-heading-actions">
        <RouterLink class="btn" to="/customers/plans">返回计划</RouterLink
        ><button class="btn" :disabled="plan.state === '已结案'" @click="actions.open('milestone', plan.id)">
          添加里程碑</button
        ><button
          class="btn btn-primary"
          :disabled="plan.state === '已结案'"
          @click="actions.open('recap', plan.id)"
        >
          结案与复盘
        </button>
      </div></PageHeading
    ><CustomerAlert
      v-if="plan.state === '已结案'"
      title="计划已结案"
      :description="plan.conclusion"
      tone="success"
    /><CustomerSection title="目标与关键结果" icon="chart"
      ><template #action><StatusBadge :text="plan.state" tone="primary" /></template>
      <div class="customer-mini-metrics">
        <div v-for="target in planResults(store.snapshot, plan)" :key="target.title">
          <small>{{ target.title }}</small
          ><strong
            >{{ target.actual }} / {{ target.target }} <small>{{ target.unit }}</small></strong
          >
          <div class="progress-track" style="margin: 12px 0">
            <div
              class="progress-fill"
              :style="{ width: `${Math.min(100, (target.actual / target.target) * 100)}%` }"
            />
          </div>
          <small>{{ target.source }} · 基线 {{ target.baseline }}</small>
        </div>
      </div>
      <p class="customer-help">
        数据随已核验交付、服务及续约事项结果更新，不以已关闭事项数量替代经营结果。
      </p></CustomerSection
    ><CustomerSection title="里程碑与执行进度" icon="calendar"
      ><template #action
        ><span class="customer-help">已完成 {{ progress }}% · 负责人 {{ plan.owner }}</span></template
      >
      <div class="plan-milestones">
        <div v-for="(m, index) in plan.milestones" :key="m.title" class="milestone">
          <span class="milestone-index" :class="{ done: m.status === '已完成' }">{{ index + 1 }}</span
          ><strong>{{ m.title }}</strong
          ><span class="customer-help">{{ m.date }}</span
          ><StatusBadge :text="m.status" :tone="m.status === '已完成' ? 'success' : 'primary'" /><RouterLink
            v-for="id in m.workIds"
            :key="id"
            :to="`/customers/work/${id}`"
            class="btn-link"
            >{{ id }}</RouterLink
          >
        </div>
      </div></CustomerSection
    >
    <div class="customer-split">
      <CustomerSection title="关联事项" icon="checks"
        ><WorkTable
          :items="related"
          :view="{ name: '', search: '', owner: '', kind: '', columns: ['负责人', '期限'], density: '标准' }"
      /></CustomerSection>
      <div class="page-stack">
        <CustomerSection title="风险与阻塞" icon="warning"
          ><CustomerAlert
            title="交付与服务结果影响本期目标"
            description="复验未通过或客户服务未恢复时，先处理执行问题，再推进续约决策。"
            tone="warning"
          /><RouterLink to="/customers/work/CS-103" class="btn-link" style="margin-top: 14px"
            >进入服务恢复验证</RouterLink
          ></CustomerSection
        ><CustomerSection title="计划活动" icon="activity"
          ><ActivityList :items="store.snapshot.activities.filter((a) => a.target === plan!.id).slice(0, 4)"
        /></CustomerSection>
      </div>
    </div>
  </div>
  <CustomerAlert v-else title="计划不存在" tone="warning" />
</template>
<style scoped>
.plan-milestones {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
}
.milestone {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  position: relative;
  padding: 0 14px 0 0;
}
.milestone-index {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--color-primary);
  background: var(--color-primary-soft);
  z-index: 1;
}
.milestone-index.done {
  background: var(--color-success);
  color: var(--color-surface);
}
.milestone::after {
  content: '';
  position: absolute;
  top: 13px;
  left: 36px;
  right: 0;
  height: 2px;
  background: var(--color-border);
}
.milestone strong {
  font-size: 14px;
}
.milestone:last-child::after {
  display: none;
}
@media (max-width: 900px) {
  .plan-milestones {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
