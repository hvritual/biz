<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workKindNames, workflows } from '@/services/customer/seed'
import type { WorkKind } from '@/types/customer'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  actions = useCustomerActions(),
  kind = ref<WorkKind>('renewal')
const versions = computed(() =>
  Array.from(
    new Set([...store.snapshot.work.map((w) => w.workflowVersion), store.snapshot.workflowVersion]),
  ).sort(),
)
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="工作流模板与版本"
      breadcrumb="客户运营"
      description="按事项类型复用工作流，保留关键业务校验与在途版本"
      ><div class="customer-heading-actions">
        <RouterLink class="btn" to="/customers/sla">服务时限策略</RouterLink
        ><button class="btn btn-primary" @click="actions.open('workflow-publish', 'workflow')">
          发布新版本
        </button>
      </div></PageHeading
    >
    <nav class="customer-tabs">
      <button v-for="(label, k) in workKindNames" :key="k" :class="{ active: kind === k }" @click="kind = k">
        {{ label }}
      </button>
    </nav>
    <div class="card customer-flow">
      <div
        v-for="(stage, index) in workflows[kind]"
        :key="stage"
        class="customer-flow-step"
        :class="{ current: index === 3, done: index < 3 }"
      >
        <span
          ><AppIcon v-if="index < 3" name="check" :size="15" /><template v-else>{{
            index + 1
          }}</template></span
        >{{ stage }}
      </div>
    </div>
    <div class="customer-split">
      <CustomerSection title="状态流转与验证" icon="organization"
        ><div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>流转</th>
                <th>必要条件</th>
                <th>允许人员</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>处理中 → 待验收</td>
                <td>依赖已结束，下一步和责任明确</td>
                <td>可编辑事项的成员</td>
              </tr>
              <tr>
                <td>待验收 → 已结束 / 成功</td>
                <td>同客户的有效业务来源、结论与确认</td>
                <td>有处理权限的责任人</td>
              </tr>
              <tr>
                <td>待验收 → 处理中</td>
                <td>未通过原因、整改人、期限与复验安排</td>
                <td>验收处理人员</td>
              </tr>
              <tr>
                <td>已结束 → 处理中</td>
                <td>重开原因、新轮次责任与期限</td>
                <td>可编辑事项的成员</td>
              </tr>
            </tbody>
          </table>
        </div>
        <CustomerAlert
          title="新版本仅用于新建事项"
          description="发布不会静默迁移现有记录。已有事项固定到原流程版本，历史验收与操作记录保持。" /></CustomerSection
      ><CustomerSection title="不可关闭的底线校验" icon="lock"
        ><div class="customer-checklist">
          <label
            v-for="rule in [
              '租户与对象范围一致',
              '有效业务来源及证据核验',
              '乐观并发版本校验',
              '有原因的退回、取消与重新打开',
              '原始结果与操作审计保留',
            ]"
            :key="rule"
            ><input type="checkbox" checked disabled />{{ rule }}</label
          >
        </div></CustomerSection
      >
    </div>
    <CustomerSection title="版本与在途事项" icon="file"
      ><div class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>版本</th>
              <th>适用范围</th>
              <th>使用中的事项</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="version in versions" :key="version">
              <td>v{{ version }}</td>
              <td>
                {{
                  version === store.snapshot.workflowVersion ? '后续新建事项' : '固定到此版本的历史及在途事项'
                }}
              </td>
              <td>
                {{
                  store.snapshot.work.filter((w) => w.workflowVersion === version && w.status !== '已结束')
                    .length
                }}
                项
              </td>
              <td>
                <StatusBadge
                  :text="version === store.snapshot.workflowVersion ? '当前版本' : '保留版本'"
                  :tone="version === store.snapshot.workflowVersion ? 'primary' : 'neutral'"
                />
              </td>
            </tr>
          </tbody>
        </table></div
    ></CustomerSection>
  </div>
</template>
