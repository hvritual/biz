<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { outcomeMetrics } from '@/services/customer/selectors'
import { downloadCsv } from '@/utils/format'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import WorkTable from '@/components/customer/WorkTable.vue'
const store = useCustomerStore(),
  type = ref('renewal')
const metrics = computed(() => outcomeMetrics(store.snapshot))
const rows = computed(() => store.snapshot.work.filter((w) => w.kind === type.value))
function exportResults() {
  downloadCsv('客户经营结果-示例.csv', [
    ['指标', '数值', '口径'],
    ['到期合同续约率', metrics.value.renewalRate, '2026年9月到期原合同，已验证续约合同'],
    ['回款已核销', metrics.value.paid, 'CNY，有效核销投影'],
    ['通过点位', metrics.value.deliveryPassed, '当前逐点验收'],
    ['关闭事项', metrics.value.closed, '包括成功与未达成，不等于经营成功'],
  ])
}
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="客户经营结果"
      breadcrumb="客户运营"
      description="从合同、交付与核销来源核对经营结果，而不是只统计任务关闭"
      ><div class="customer-heading-actions">
        <button class="btn" @click="exportResults">导出当前口径</button>
      </div></PageHeading
    >
    <section class="card data-panel">
      <div class="row-between wrap">
        <div class="row wrap">
          <strong>2026 年 9 月到期批次</strong><span class="pill">当前租户 · 示例快照</span>
        </div>
        <span class="customer-help">金额 CNY · 时区 Asia/Shanghai · 结果随本地验收更新</span>
      </div>
    </section>
    <div class="metric-grid">
      <MetricCard
        label="到期合同续约率"
        :value="metrics.renewalRate"
        icon="refresh"
        :caption="`${metrics.renewed} / ${metrics.cohort} 份到期原合同`"
      /><MetricCard
        label="回款已核销"
        :value="metrics.paid.toLocaleString()"
        unit="元"
        icon="order"
        tone="green"
        :caption="`关联应收 ${metrics.due.toLocaleString()} 元 · CNY`"
      /><MetricCard
        label="投放验收通过"
        :value="`${metrics.deliveryPassed} / ${metrics.deliveryTotal}`"
        unit="点位"
        icon="site"
        tone="orange"
        caption="不是交付事项关闭数量"
      /><MetricCard
        label="成功 / 已结束事项"
        :value="`${metrics.success} / ${metrics.closed}`"
        icon="checks"
        tone="purple"
        caption="执行统计，不等于经营成功率"
      />
    </div>
    <div class="customer-split">
      <CustomerSection title="目标达成与来源" icon="chart"
        ><div class="result-bars">
          <div
            v-for="row in [
              {
                title: '到期合同续约',
                actual: metrics.renewed,
                total: metrics.cohort,
                unit: '份',
                type: 'renewal',
              },
              {
                title: '点位验收通过',
                actual: metrics.deliveryPassed,
                total: metrics.deliveryTotal,
                unit: '个',
                type: 'delivery',
              },
              {
                title: '本期应收核销',
                actual: metrics.paid,
                total: metrics.due,
                unit: '元',
                type: 'payment',
              },
            ]"
            :key="row.title"
          >
            <div class="row-between">
              <strong>{{ row.title }}</strong
              ><button class="btn-link" @click="type = row.type">
                {{ row.actual.toLocaleString() }} / {{ row.total.toLocaleString() }} {{ row.unit }} · 下钻
              </button>
            </div>
            <div class="progress-track">
              <div
                class="progress-fill"
                :style="{ width: `${row.total ? (row.actual / row.total) * 100 : 0}%` }"
              />
            </div>
          </div></div></CustomerSection
      ><CustomerSection title="数据口径" icon="help"
        ><dl class="customer-info-list">
          <dt>续约分母</dt>
          <dd>选定月份内到期的原合同，不按客户数。</dd>
          <dt>续约成功</dt>
          <dd>同一原合同对应的续约事项成功且生效合同已验证。</dd>
          <dt>有效回款</dt>
          <dd>财务核销来源，不使用付款截图或跟进任务数。</dd>
          <dt>完成率分母为零</dt>
          <dd>显示 —，不输出虚构的 0% 或 100%。</dd>
        </dl></CustomerSection
      >
    </div>
    <CustomerSection title="结果明细与事项下钻" icon="checks"
      ><template #action
        ><select v-model="type" class="select" aria-label="结果明细类型">
          <option value="renewal">续约明细</option>
          <option value="delivery">交付明细</option>
          <option value="payment">回款明细</option>
          <option value="service">服务明细</option>
        </select></template
      ><WorkTable :items="rows" /></CustomerSection
    ><CustomerAlert
      title="数据延迟、缺失和无权限不能显示为零"
      description="本页面明确展示本地示例投影；生产数据平台接入后应保留数据状态、更新时间与完整性提示。"
    />
  </div>
</template>
<style scoped>
.result-bars {
  display: grid;
  gap: 30px;
  padding: 8px 0;
}
.result-bars .row-between {
  font-size: 14px;
  margin-bottom: 12px;
}
.result-bars .progress-track {
  height: 12px;
}
</style>
