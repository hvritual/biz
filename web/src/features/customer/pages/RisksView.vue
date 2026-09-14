<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerIdentity from '@/components/customer/CustomerIdentity.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  actions = useCustomerActions(),
  active = ref('服务风险')
const customer = computed(() => store.snapshot.customers.find((c) => c.id === 'CUS-0186')!)
const signals = [
  {
    name: '服务风险',
    fact: 'WO-078 已处理，但客户事项 CS-103 尚待恢复验证',
    action: 'CS-103',
    source: '运维工单 / 运行验证 / 客户确认',
  },
  {
    name: '到期风险',
    fact: '合同 HT-2026-018 将于 2026-09-30 到期，续约尚未成功',
    action: 'CS-102',
    source: '合同有效期 / 续约事项',
  },
  {
    name: '回款风险',
    fact: '9 月应收 12,000 元，当前核销投影仍有 3,000 元待核对',
    action: 'CS-105',
    source: '应收 AR-0901 / 核销记录',
  },
  {
    name: '使用变化',
    fact: '苏州高铁点位需要先完成整改验收，不能据此直接推断客户流失',
    action: 'CS-104',
    source: '逐点验收 / 投放有效期',
  },
]
const signal = computed(() => signals.find((x) => x.name === active.value)!)
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="客户风险与研判"
      breadcrumb="客户经营"
      description="把风险原因、来源证据与后续行动连接起来，不输出不透明的健康分"
    /><CustomerIdentity :customer="customer" />
    <nav class="customer-tabs">
      <button
        v-for="item in signals"
        :key="item.name"
        :class="{ active: active === item.name }"
        @click="active = item.name"
      >
        {{ item.name }}
      </button>
    </nav>
    <div class="customer-split">
      <CustomerSection :title="signal.name + ' · 证据与判断'" icon="warning"
        ><CustomerAlert
          :title="signal.fact"
          :description="`数据来源：${signal.source} · 本地预览快照`"
          tone="warning"
        />
        <dl class="customer-info-list" style="margin-top: 24px">
          <dt>统计范围</dt>
          <dd>星悦酒店集团 · 本次有效业务关系</dd>
          <dt>数据完整性</dt>
          <dd>已知业务记录可见；缺失数据必须先核查，不默认为零。</dd>
          <dt>已知影响</dt>
          <dd>需确认原服务问题及合同周期，不能直接判定客户流失。</dd>
          <dt>重复事项</dt>
          <dd>
            <RouterLink :to="`/customers/work/${signal.action}`" class="btn-link"
              >已存在 {{ signal.action }}</RouterLink
            >，建议关联而非新建
          </dd>
          <dt>处理结论</dt>
          <dd>{{ store.snapshot.drafts['risk:CUS-0186']?.result || '待研判' }}</dd>
          <dt>结论说明</dt>
          <dd>{{ store.snapshot.drafts['risk:CUS-0186']?.reason || '尚未提交' }}</dd>
        </dl>
        <div class="customer-inline-actions" style="margin-top: 24px">
          <button
            class="btn btn-primary"
            @click="actions.open('triage', customer.id, [], { workId: signal.action })"
          >
            记录研判与下一步</button
          ><RouterLink :to="`/customers/work/${signal.action}`" class="btn">进入已有事项</RouterLink>
        </div></CustomerSection
      >
      <div class="page-stack">
        <CustomerSection title="风险维度分开解释" icon="shield"
          ><div v-for="s in signals" :key="s.name" class="customer-record">
            <div>
              <strong>{{ s.name }}</strong>
              <p>{{ s.source }}</p>
            </div>
            <StatusBadge text="需关注" tone="warning" /></div></CustomerSection
        ><CustomerSection title="研判规则" icon="help"
          ><p class="customer-help">
            先确认范围与数据完整性，再排除节假日、停业和采集延迟。自动化只触发待研判工作，不自动更改客户生命周期。
          </p></CustomerSection
        >
      </div>
    </div>
  </div>
</template>
