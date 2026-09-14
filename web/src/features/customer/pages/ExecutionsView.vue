<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  actions = useCustomerActions(),
  selected = ref('RUN-240')
const current = computed(() => store.snapshot.executions.find((e) => e.id === selected.value))
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="自动化执行记录"
      breadcrumb="客户运营"
      description="看清每次外部操作的结果；部分失败先对账，恢复不重复建单"
      ><div class="customer-heading-actions">
        <RouterLink class="btn" to="/customers/automation">返回规则中心</RouterLink>
      </div></PageHeading
    ><CustomerSection title="执行队列" icon="file"
      ><div class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>执行编号 / 时间</th>
              <th>规则</th>
              <th>关联客户</th>
              <th>关联事项</th>
              <th>结果</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in store.snapshot.executions" :key="e.id" :class="{ selected: e.id === selected }">
              <td>
                <strong>{{ e.id }}</strong>
                <p class="subline">{{ e.time }}</p>
              </td>
              <td>{{ store.snapshot.rules.find((r) => r.id === e.ruleId)?.name }}</td>
              <td>{{ store.customerName(e.customerId) }}</td>
              <td>
                <RouterLink :to="`/customers/work/${e.workId}`" class="btn-link">{{ e.workId }}</RouterLink>
              </td>
              <td><StatusBadge :text="e.state" :tone="e.state === '部分失败' ? 'danger' : 'success'" /></td>
              <td><button class="btn-link" @click="selected = e.id">查看执行链</button></td>
            </tr>
          </tbody>
        </table>
      </div></CustomerSection
    >
    <div v-if="current" class="customer-split">
      <CustomerSection :title="`${current.id} · 对账与恢复`" icon="refresh"
        ><template #action
          ><button
            class="btn btn-primary"
            :disabled="current.state !== '部分失败'"
            @click="actions.open('recover', current.id)"
          >
            恢复未完成步骤
          </button></template
        ><CustomerAlert
          :title="current.state === '部分失败' ? '事项已存在，通知步骤失败' : '执行记录已完成核对'"
          :description="
            current.state === '部分失败'
              ? '恢复前先读取既有事项，不允许将整条规则当作新请求重新执行。'
              : '原失败事件保留，仅追加恢复结果，不抹去历史失败。'
          "
          :tone="current.state === '部分失败' ? 'warning' : 'success'"
        />
        <dl class="customer-info-list" style="margin-top: 22px">
          <dt>业务幂等键</dt>
          <dd>{{ current.businessKey }}</dd>
          <dt>既有事项</dt>
          <dd>
            <RouterLink :to="`/customers/work/${current.workId}`" class="btn-link">{{
              current.workId
            }}</RouterLink>
            · 已回读
          </dd>
          <dt>失败步骤</dt>
          <dd>{{ current.failedStep || '无待恢复步骤' }}</dd>
          <dt>恢复策略</dt>
          <dd>已有成功步骤保持；仅补充未完成的本地提醒。</dd>
          <dt>原始记录</dt>
          <dd>执行历史不可覆盖或删除</dd>
        </dl></CustomerSection
      ><CustomerSection title="逐步执行回执" icon="activity"
        ><div v-for="(entry, index) in current.history" :key="index" class="customer-activity">
          <span class="activity-dot" />
          <div>
            <small>步骤 {{ index + 1 }}</small>
            <p>{{ entry }}</p>
          </div>
        </div></CustomerSection
      >
    </div>
  </div>
</template>
