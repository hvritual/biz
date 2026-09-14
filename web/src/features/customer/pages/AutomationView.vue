<script setup lang="ts">
import { UiButton } from '@/ui/base'

import { computed, ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { evaluateRule } from '@/services/customer/automation'
import PageHeading from '@/ui/common/PageHeading.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import SearchField from '@/ui/common/SearchField.vue'
import CustomerSection from '@/features/customer/components/CustomerSection.vue'
import CustomerAlert from '@/features/customer/components/CustomerAlert.vue'
const store = useCustomerStore(),
  actions = useCustomerActions(),
  query = ref(''),
  selected = ref('RULE-01')
const rules = computed(() => store.snapshot.rules.filter((r) => !query.value || r.name.includes(query.value)))
const current = computed(() => store.snapshot.rules.find((r) => r.id === selected.value))
const matches = computed(() => (current.value ? evaluateRule(store.snapshot, current.value) : []))
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="自动化规则"
      breadcrumb="客户运营"
      description="让系统发现下一步工作，同时保留触发依据、去重规则与执行回执"
      ><div class="customer-heading-actions">
        <RouterLink to="/customers/executions" class="btn">执行记录</RouterLink
        ><UiButton class="btn btn-primary" @click="actions.open('create-rule')">新建规则</UiButton>
      </div></PageHeading
    >
    <div class="metric-grid">
      <MetricCard
        label="启用规则"
        :value="store.snapshot.rules.filter((r) => r.enabled).length"
        icon="activity"
        tone="green"
        caption="本地配置，不启动后台调度"
      /><MetricCard
        label="待测试草稿"
        :value="store.snapshot.rules.filter((r) => !r.tested).length"
        icon="file"
        caption="启用前必须试运行"
      /><MetricCard
        label="待恢复执行"
        :value="store.snapshot.executions.filter((e) => e.state === '部分失败').length"
        icon="warning"
        tone="orange"
        caption="先对账再恢复"
      /><MetricCard
        label="已恢复执行"
        :value="store.snapshot.executions.filter((e) => e.state === '已恢复').length"
        icon="checks"
        tone="purple"
        caption="原失败回执仍保留"
      />
    </div>
    <CustomerSection title="规则列表" icon="activity"
      ><div class="query-bar">
        <SearchField v-model="query" label="搜索自动化规则" placeholder="搜索规则名称…" /><UiButton
          class="btn"
          @click="query = ''"
        >
          重置
        </UiButton>
      </div>
      <div class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>规则名称</th>
              <th>触发条件</th>
              <th>分配对象</th>
              <th>版本</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rules" :key="r.id" :class="{ selected: r.id === selected }">
              <td>
                <UiButton class="btn-link" @click="selected = r.id">{{ r.name }}</UiButton>
                <p class="subline">{{ r.id }}</p>
              </td>
              <td>{{ r.trigger }} · {{ r.days }} 天</td>
              <td>{{ r.owner }}</td>
              <td>v{{ r.version }}</td>
              <td>
                <StatusBadge
                  :text="r.enabled ? '已启用' : r.tested ? '已测试 · 未启用' : '未测试草稿'"
                  :tone="r.enabled ? 'success' : 'neutral'"
                />
              </td>
              <td>
                <div class="table-actions">
                  <UiButton
                    class="btn-link"
                    @click="
                      () => {
                        selected = r.id
                        actions.open('rule-test', r.id)
                      }
                    "
                  >
                    试运行</button
                  ><UiButton class="btn-link" @click="actions.open('rule-toggle', r.id)">
                    {{ r.enabled ? '停用' : '启用' }}
                  </UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div></CustomerSection
    >
    <div v-if="current" class="customer-split">
      <CustomerSection :title="current.name + ' · 规则详情'" icon="filter"
        ><template #action
          ><UiButton class="btn btn-primary" @click="actions.open('rule-publish', current.id)">
            发布版本
          </UiButton></template
        >
        <dl class="customer-info-list">
          <dt>触发</dt>
          <dd>{{ current.trigger }} {{ current.days }} 天 · 截至 2026-09-10</dd>
          <dt>条件</dt>
          <dd>{{ current.scope }}</dd>
          <dt>执行动作</dt>
          <dd>创建或复用客户事项 → 分配责任人 → 记录本地提醒</dd>
          <dt>去重维度</dt>
          <dd>租户 ＋ 合同 / 客户 / 事项 ＋ 业务周期</dd>
          <dt>失败处置</dt>
          <dd>
            <RouterLink to="/customers/executions" class="btn-link"
              >进入执行异常队列，核对实际结果后重试</RouterLink
            >
          </dd>
        </dl>
        <div class="divider" />
        <h3 style="font-size: 14px; margin-bottom: 12px">当前条件预览</h3>
        <div v-for="match in matches" :key="match.businessKey" class="customer-record">
          <div>
            <strong>{{ match.objectId }}</strong>
            <p>{{ match.businessKey }}</p>
          </div>
          <span>{{ match.action }}</span>
        </div>
        <p v-if="!matches.length" class="customer-help">
          当前示例快照无命中对象，不创建任何事项。
        </p></CustomerSection
      >
      <div class="page-stack">
        <CustomerSection title="无副作用测试结果" icon="checks"
          ><CustomerAlert
            :title="store.snapshot.drafts[`test:${current.id}`] ? '试运行已完成' : '等待试运行'"
            :description="
              String(
                store.snapshot.drafts[`test:${current.id}`]?.result ||
                  '测试只评估条件，实际创建 0 项，发送 0 次。',
              )
            "
            :tone="store.snapshot.drafts[`test:${current.id}`] ? 'success' : 'primary'"
          /><UiButton class="btn" style="margin-top: 16px" @click="actions.open('rule-test', current.id)">
            重新试运行
          </UiButton></CustomerSection
        ><CustomerSection title="工作流与客户可见性" icon="shield"
          ><div class="customer-checklist">
            <RouterLink to="/customers/workflows" class="btn-link">管理流程模板与版本</RouterLink
            ><RouterLink to="/customers/sla" class="btn-link">服务时限与升级策略</RouterLink
            ><RouterLink to="/customers/sharing" class="btn-link">管理客户侧共享</RouterLink>
          </div></CustomerSection
        >
      </div>
    </div>
  </div>
</template>
