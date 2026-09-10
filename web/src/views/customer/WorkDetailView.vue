<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workKindNames } from '@/services/customer/seed'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import WorkSteps from '@/components/customer/WorkSteps.vue'
import WorkTable from '@/components/customer/WorkTable.vue'
import ActivityList from '@/components/customer/ActivityList.vue'
import SourceRecords from '@/components/customer/SourceRecords.vue'
import DeliveryDetail from '@/components/customer/DeliveryDetail.vue'
import ServiceDetail from '@/components/customer/ServiceDetail.vue'
import PaymentDetail from '@/components/customer/PaymentDetail.vue'
import RenewalDetail from '@/components/customer/RenewalDetail.vue'
import ReturnDetail from '@/components/customer/ReturnDetail.vue'
import GeneralWorkDetail from '@/components/customer/GeneralWorkDetail.vue'
const store = useCustomerStore(),
  route = useRoute(),
  actions = useCustomerActions(),
  tab = ref('详情')
const work = computed(() => store.snapshot.work.find((w) => w.id === route.params.id))
const component = computed(
  () =>
    ({
      delivery: DeliveryDetail,
      service: ServiceDetail,
      payment: PaymentDetail,
      renewal: RenewalDetail,
      return: ReturnDetail,
      visit: GeneralWorkDetail,
      improvement: GeneralWorkDetail,
    })[work.value?.kind || 'visit'],
)
const activity = computed(() => store.snapshot.activities.filter((a) => a.target === work.value?.id))
const dependencies = computed(() =>
  store.snapshot.work.filter((w) => work.value?.dependencies.includes(w.id)),
)
</script>
<template>
  <div v-if="work" class="page-stack">
    <PageHeading
      :title="`${work.id} · ${work.title}`"
      breadcrumb="客户事项"
      :description="`${store.customerName(work.customerId)} · ${workKindNames[work.kind]} · 流程 v${work.workflowVersion} · 第 ${work.cycle} 轮处理`"
      ><div class="customer-heading-actions">
        <RouterLink class="btn" to="/customers/work">返回事项</RouterLink
        ><button class="btn" :disabled="!work.writable" @click="actions.open('subtask', work.id)">
          <AppIcon name="plus" :size="16" />子事项</button
        ><button
          v-if="work.status === '已结束'"
          class="btn btn-primary"
          @click="actions.open('reopen', work.id)"
        >
          重新打开</button
        ><template v-else
          ><button class="btn" :disabled="!work.writable" @click="actions.open('transition', work.id)">
            流转事项</button
          ><button
            class="btn btn-primary"
            :disabled="!work.writable || work.status !== '待验收'"
            @click="actions.open('accept', work.id)"
          >
            事项验收
          </button></template
        >
      </div></PageHeading
    >
    <CustomerAlert
      v-if="!work.writable"
      title="此事项仅可查看"
      description="当前预览身份没有该事项的编辑权限；批量分配会保留跳过原因。"
      tone="warning"
    />
    <CustomerAlert
      v-if="work.status === '已结束'"
      :title="`事项已结束 · ${work.resolution}`"
      :description="`处理轮次 ${work.cycle} · 责任人 ${work.owner} · 记录版本 v${work.version}。原始业务来源、执行记录和验收结果继续保留。`"
      :tone="work.resolution === '成功' ? 'success' : 'warning'"
    />
    <WorkSteps :work="work" />
    <nav class="customer-tabs" aria-label="事项详情标签">
      <button
        v-for="name in ['详情', '依赖与子事项', '业务来源', '评论与活动']"
        :key="name"
        :class="{ active: tab === name }"
        @click="tab = name"
      >
        {{ name }}<span v-if="name === '依赖与子事项'">（{{ dependencies.length }}）</span>
      </button>
    </nav>
    <div
      class="customer-split"
      :class="{ 'customer-full-detail': work.kind === 'delivery' && tab === '详情' }"
    >
      <div class="page-stack">
        <component :is="component" v-if="tab === '详情'" :key="work.id" :work="work" /><CustomerSection
          v-else-if="tab === '依赖与子事项'"
          title="依赖与阻塞"
          icon="link"
          ><template #action
            ><button class="btn-link" @click="actions.open('dependency', work.id)">添加依赖</button></template
          ><CustomerAlert
            title="依赖未结束时，不能进入验收或成功关闭"
            description="不复制工单和业务记录，跨部门协作引用同一事项。"
            tone="warning"
          /><WorkTable
            v-if="dependencies.length"
            :items="dependencies"
            :view="{
              name: '',
              search: '',
              kind: '',
              owner: '',
              columns: ['负责人', '期限'],
              density: '标准',
            }"
          />
          <p v-else class="customer-help" style="margin-top: 20px">
            当前没有依赖。添加依赖时会校验自身引用及循环依赖。
          </p></CustomerSection
        ><CustomerSection v-else-if="tab === '业务来源'" title="关联业务来源" icon="file"
          ><template #action
            ><button class="btn-link" @click="actions.open('link-source', work.id)">
              关联记录
            </button></template
          ><SourceRecords
            :records="
              store.snapshot.sources.filter((s) => work!.evidenceIds.includes(s.id))
            " /></CustomerSection
        ><CustomerSection v-else title="评论与活动记录" icon="activity"
          ><template #action
            ><button class="btn-link" @click="actions.open('comment', work.id)">添加评论</button></template
          ><ActivityList :items="activity"
        /></CustomerSection>
      </div>
      <aside class="page-stack">
        <CustomerSection title="事项信息" icon="help"
          ><template #action
            ><StatusBadge
              :text="work.status"
              :tone="
                work.status === '已结束' ? (work.resolution === '成功' ? 'success' : 'warning') : 'primary'
              "
          /></template>
          <dl class="customer-info-list">
            <dt>事项负责人</dt>
            <dd class="row"><AvatarMark :name="work.owner" :size="25" />{{ work.owner }}</dd>
            <dt>客户负责人</dt>
            <dd>{{ store.snapshot.customers.find((c) => c.id === work!.customerId)?.owner }}</dd>
            <dt>协作人员</dt>
            <dd>{{ work.collaborators.join('、') || '未指定' }}</dd>
            <dt>优先级</dt>
            <dd><StatusBadge :text="work.priority" tone="danger" /></dd>
            <dt>截止时间</dt>
            <dd>{{ work.deadline }}</dd>
            <dt>下次行动</dt>
            <dd>{{ work.nextAt.replace('T', ' ') }}</dd>
            <dt>所属客户</dt>
            <dd>
              <RouterLink :to="`/customers/accounts/${work.customerId}`" class="btn-link">{{
                store.customerName(work.customerId)
              }}</RouterLink>
            </dd>
            <dt>处理轮次</dt>
            <dd>第 {{ work.cycle }} 轮 · 版本 {{ work.version }}</dd>
          </dl>
          <div class="divider" />
          <p class="customer-help">下一步行动</p>
          <p style="line-height: 1.8; margin-top: 8px; font-size: 14px">{{ work.nextAction }}</p>
          <button
            v-if="work.status !== '已结束'"
            class="btn-link"
            style="margin-top: 12px"
            :disabled="!work.writable"
            @click="actions.open('reschedule', work.id)"
          >
            调整行动安排
          </button></CustomerSection
        >
        <CustomerSection title="验收与结果" icon="checks"
          ><p class="customer-help">
            成功需核对业务来源；不通过则退回整改。事项“已结束”不等于经营结果“成功”。
          </p>
          <div class="customer-checklist" style="margin-top: 16px">
            <button
              v-if="work.status !== '已结束'"
              class="btn"
              :disabled="!work.writable"
              @click="actions.open('reject', work.id)"
            >
              验收不通过 / 退回处理</button
            ><button class="btn" @click="tab = '业务来源'">核对业务来源</button
            ><button v-if="dependencies.length" class="btn" @click="tab = '依赖与子事项'">
              查看 {{ dependencies.length }} 项依赖
            </button>
          </div></CustomerSection
        ><CustomerSection title="最近记录" icon="clock"
          ><ActivityList :items="activity.slice(0, 2)"
        /></CustomerSection>
      </aside>
    </div>
  </div>
  <CustomerAlert v-else title="事项不存在或不属于当前租户" tone="warning"
    ><RouterLink to="/customers/work" class="btn">返回事项工作台</RouterLink></CustomerAlert
  >
</template>
