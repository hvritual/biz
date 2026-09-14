<script setup lang="ts">
import type { WorkItem } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import CustomerSection from './CustomerSection.vue'
import CustomerAlert from './CustomerAlert.vue'
import SourceRecords from './SourceRecords.vue'
defineProps<{ work: WorkItem }>()
const store = useCustomerStore(),
  actions = useCustomerActions()
</script>
<template>
  <div class="page-stack">
    <CustomerSection
      :title="work.kind === 'visit' ? '回访安排与客户反馈' : '事项目标与执行要求'"
      :icon="work.kind === 'visit' ? 'phone' : 'checks'"
      ><p style="font-size: 14px; line-height: 1.9">{{ work.description }}</p>
      <dl class="customer-info-list" style="margin-top: 24px">
        <dt>关联客户</dt>
        <dd>{{ store.customerName(work.customerId) }}</dd>
        <dt>下一步行动</dt>
        <dd>{{ work.nextAction }}</dd>
        <dt>下次行动时间</dt>
        <dd>{{ work.nextAt.replace('T', ' ') }}</dd>
        <dt>截止时间</dt>
        <dd>{{ work.deadline }}</dd>
        <dt>责任人</dt>
        <dd>{{ work.owner }}</dd>
      </dl>
      <div style="margin-top: 20px" class="customer-inline-actions">
        <button
          v-if="work.kind === 'visit'"
          class="btn btn-primary"
          :disabled="work.status === '已结束'"
          @click="actions.open('visit', work.id)"
        >
          记录回访与承诺</button
        ><button class="btn" @click="actions.open('comment', work.id)">记录沟通</button>
      </div></CustomerSection
    ><CustomerSection title="成功标准" icon="checks"
      ><div v-for="criterion in work.criteria" :key="criterion" class="customer-record">{{ criterion }}</div>
      <CustomerAlert
        title="有后续动作的沟通要落为独立事项"
        description="回访活动和执行事项分开。关闭回访不会关闭承诺，更不能替代业务结果验收。" /></CustomerSection
    ><CustomerSection title="关联业务依据" icon="file"
      ><SourceRecords
        :records="store.snapshot.sources.filter((s) => work.evidenceIds.includes(s.id))"
      /><button class="btn-link" @click="actions.open('link-source', work.id)">
        关联业务来源
      </button></CustomerSection
    >
  </div>
</template>
