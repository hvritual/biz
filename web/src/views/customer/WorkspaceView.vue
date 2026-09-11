<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerIdentity from '@/components/customer/CustomerIdentity.vue'
import CustomerTabs from '@/components/customer/CustomerTabs.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import WorkTable from '@/components/customer/WorkTable.vue'
import ActivityList from '@/components/customer/ActivityList.vue'
const store = useCustomerStore(),
  route = useRoute(),
  actions = useCustomerActions()
const customer = computed(() => store.snapshot.customers.find((c) => c.id === route.params.id))
const work = computed(() => store.snapshot.work.filter((w) => w.customerId === customer.value?.id))
const active = computed(() => String(route.query.tab || 'overview'))
const activity = computed(() =>
  store.snapshot.activities
    .filter((a) => a.target === customer.value?.id || work.value.some((w) => w.id === a.target))
    .slice(0, 8),
)
</script>
<template>
  <div v-if="customer" class="page-stack">
    <PageHeading
      title="客户工作区"
      breadcrumb="客户经营"
      description="掌握客户承诺、当前阻塞与可验证的经营结果"
      ><div class="customer-heading-actions">
        <button class="btn" @click="actions.open('edit-customer', customer.id)">
          <AppIcon name="edit" :size="16" />编辑资料</button
        ><button
          class="btn btn-primary"
          @click="actions.open('create-work', customer.id, [], { customerId: customer.id })"
        >
          <AppIcon name="plus" :size="16" />新建事项
        </button>
      </div></PageHeading
    ><CustomerIdentity :customer="customer" /><CustomerTabs :id="customer.id" :active="active" />
    <div v-if="active === 'overview'" class="customer-split">
      <div class="page-stack">
        <CustomerSection title="下一步行动" icon="checks"
          ><template #action
            ><RouterLink :to="`/customers/work?customer=${customer.id}`" class="btn-link"
              >全部事项</RouterLink
            ></template
          ><WorkTable
            :items="work.filter((w) => w.status !== '已结束').slice(0, 3)"
            :view="{
              name: '',
              search: '',
              owner: '',
              kind: '',
              density: '标准',
              columns: ['负责人', '期限'],
            }"
          />
          <p v-if="!work.length" class="customer-help">
            尚无事项，使用新建事项明确首次行动。
          </p></CustomerSection
        >
        <CustomerSection title="客户资料" icon="company"
          ><dl class="customer-info-list">
            <dt>经营模式</dt>
            <dd>{{ customer.mode }}</dd>
            <dt>客户生命周期</dt>
            <dd>{{ customer.lifecycle }} · 不由单份合同状态自动决定</dd>
            <dt>关联客户方租户</dt>
            <dd>
              {{ customer.tenantLink || '尚未关联' }}
              <span class="customer-help">仅业务关系，不自动共享内部数据</span>
            </dd>
            <dt>最近联系</dt>
            <dd>{{ customer.lastContact }}</dd>
            <dt>有效点位 / 设备</dt>
            <dd>
              {{ customer.sites }} 个 / {{ customer.devices }} 台
              <RouterLink class="btn-link" :to="`/customers/accounts/${customer.id}?tab=sites`"
                >查看有效投放</RouterLink
              >
            </dd>
            <dt>客户负责人</dt>
            <dd>
              {{ customer.owner }}
              <button class="btn-link" @click="actions.open('handover', customer.id)">移交责任</button>
            </dd>
          </dl></CustomerSection
        >
        <CustomerSection title="客户承诺" icon="clock"
          ><div
            v-for="w in work.filter((w) => w.kind === 'improvement' && w.status !== '已结束').slice(0, 2)"
            :key="w.id"
            class="customer-record"
          >
            <div>
              <strong>{{ w.title }}</strong>
              <p>{{ w.nextAction }} · {{ w.owner }} · {{ w.deadline }}</p>
            </div>
            <RouterLink :to="`/customers/work/${w.id}`" class="btn-link">推进</RouterLink>
          </div>
          <button
            class="btn-link"
            @click="actions.open('visit', work.find((w) => w.kind === 'visit')?.id || 'CS-107')"
          >
            记录回访与承诺
          </button></CustomerSection
        >
      </div>
      <div class="page-stack">
        <CustomerSection title="需要关注" icon="warning"
          ><CustomerAlert
            :title="customer.risk"
            :description="
              ['服务风险', '回款风险', '到期风险'].includes(customer.risk)
                ? '核对数据证据与当前事项，避免重复创建工作。'
                : '尚未识别到需升级处理的异常；持续关注客户活动。'
            "
            :tone="['服务风险', '回款风险', '到期风险'].includes(customer.risk) ? 'warning' : 'primary'"
          />
          <div
            v-for="w in work.filter((w) => ['service', 'renewal'].includes(w.kind)).slice(0, 2)"
            :key="w.id"
            class="customer-record"
          >
            <div>
              <strong>{{ w.kind === 'service' ? '服务恢复需要验证' : '合同续约需要推进' }}</strong>
              <p>{{ w.id }} · {{ w.title }} · {{ w.status }}</p>
            </div>
            <RouterLink :to="`/customers/work/${w.id}`" class="btn-link">查看</RouterLink>
          </div>
          <RouterLink to="/customers/risks" class="btn-link">查看证据与研判</RouterLink></CustomerSection
        ><CustomerSection title="最近活动" icon="activity"
          ><ActivityList :items="activity.slice(0, 3)"
        /></CustomerSection>
      </div>
    </div>
    <CustomerSection v-else-if="active === 'sites'" title="点位与有效投放" icon="site"
      ><CustomerAlert
        title="按投放关系有效期查看设备数据"
        description="此处是客户经营关联视图，不是通用设备管理。解绑后不得读取设备后续投放数据。"
      />
      <RouterLink :to="`/sites?customer=${customer.id}`" class="btn-link"
        >打开客户全部点位与租赁计费</RouterLink
      >
      <div v-if="customer.id === 'CUS-0186'" class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>点位</th>
              <th>投放关系</th>
              <th>关联设备</th>
              <th>状态</th>
              <th>关联事项</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="site in store.snapshot.sites.filter((x) => x.customerId === customer!.id)"
              :key="site.id"
            >
              <td>
                <RouterLink :to="`/sites/${site.id}`" class="btn-link">{{ site.name }}</RouterLink>
              </td>
              <td>{{ site.placement }}</td>
              <td>{{ site.device }}</td>
              <td>
                <StatusBadge
                  :text="site.accepted ? '投放有效 · 已验收' : '待整改复验'"
                  :tone="site.accepted ? 'success' : 'warning'"
                />
              </td>
              <td><RouterLink to="/customers/work/CS-104" class="btn-link">CS-104 交付</RouterLink></td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="customer-help" style="margin-top: 20px">
        当前示例未提供逐点投放明细，未将缺失明细显示为零。
      </p>
      <RouterLink to="/customers/work/CS-106" class="btn-link"
        >查看退租、终止与历史权限</RouterLink
      ></CustomerSection
    >
    <CustomerSection v-else title="客户活动记录" icon="activity"
      ><ActivityList :items="activity"
    /></CustomerSection>
  </div>
  <CustomerAlert v-else title="客户不存在或不在当前租户可见范围" tone="warning"
    ><RouterLink to="/customers" class="btn">返回客户总览</RouterLink></CustomerAlert
  >
</template>
