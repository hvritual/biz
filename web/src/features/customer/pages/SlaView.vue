<script setup lang="ts">
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  actions = useCustomerActions()
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="服务时限与升级策略"
      breadcrumb="客户运营"
      description="首次响应、服务恢复、事项期限与下一次行动分开管理"
      ><div class="customer-heading-actions">
        <button class="btn btn-primary" @click="actions.open('sla', 'sla')">调整服务策略</button>
      </div></PageHeading
    >
    <div class="metric-grid">
      <MetricCard
        label="首次响应时限"
        :value="store.snapshot.sla.responseMinutes"
        unit="分钟"
        icon="clock"
        caption="从受理时点开始"
      /><MetricCard
        label="恢复目标时限"
        :value="store.snapshot.sla.recoveryHours"
        unit="小时"
        icon="operations"
        tone="orange"
        caption="到实际恢复验证完成"
      /><MetricCard
        label="未恢复服务事项"
        :value="store.openWork.filter((w) => w.kind === 'service').length"
        unit="项"
        icon="warning"
        tone="purple"
        caption="不是超时计算结果"
      /><MetricCard
        label="当前工作日历"
        value="独立策略"
        icon="calendar"
        tone="green"
        :caption="store.snapshot.sla.calendar"
      />
    </div>
    <div class="customer-split">
      <CustomerSection title="计时与暂停条件" icon="clock"
        ><div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>时钟</th>
                <th>开始</th>
                <th>暂停</th>
                <th>停止</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>首次响应</td>
                <td>服务受理</td>
                <td>默认不暂停</td>
                <td>首次有效响应</td>
              </tr>
              <tr>
                <td>服务恢复</td>
                <td>确认故障影响</td>
                <td>{{ store.snapshot.sla.pause }}</td>
                <td>恢复验证通过</td>
              </tr>
              <tr>
                <td>事项截止</td>
                <td>创建事项指定日期</td>
                <td>不随服务计时暂停</td>
                <td>事项结束</td>
              </tr>
              <tr>
                <td>下一次行动</td>
                <td>指定行动安排</td>
                <td>不因等待客户而消失</td>
                <td>执行或有原因改期</td>
              </tr>
            </tbody>
          </table>
        </div>
        <CustomerAlert
          title="工作流等待不等于暂停全部时钟"
          description="审批通过的服务暂停只作用于指定恢复计时。客户经理下一次跟进及合同期限继续保留。"
          tone="warning" /></CustomerSection
      ><CustomerSection title="超时升级与跟进" icon="bell"
        ><div class="customer-checklist">
          <p>1. 按约定日历检查响应与恢复时限。</p>
          <p>2. 超时通知事项负责人及客户负责人。</p>
          <p>3. 保留原事项，升级处理不重复创建故障。</p>
          <p>4. 恢复验证通过后记录停止时点。</p>
        </div>
        <RouterLink to="/customers/work/CS-103" class="btn-link" style="margin-top: 20px"
          >查看 CS-103 服务恢复验证</RouterLink
        ></CustomerSection
      >
    </div>
    <CustomerAlert
      title="当前是策略配置与界面预览"
      description="未启动生产 SLA 计时器、工作日历服务或消息服务；不将未运行的计时标记为达标。"
    />
  </div>
</template>
