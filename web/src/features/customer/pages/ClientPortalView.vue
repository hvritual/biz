<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { clientProjection } from '@/services/customer/policy'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  route = useRoute(),
  actions = useCustomerActions()
const projection = computed(() => {
  try {
    return { data: clientProjection(store.snapshot, String(route.params.id)), error: '' }
  } catch (e) {
    return { data: undefined, error: (e as Error).message }
  }
})
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="客户侧交付确认"
      breadcrumb="授权客户视角 · 预览"
      description="仅展示本次授权范围内的进度、资料与验收动作"
      ><div class="customer-heading-actions">
        <RouterLink to="/customers/sharing" class="btn">返回服务方共享管理</RouterLink>
      </div></PageHeading
    ><CustomerAlert
      v-if="projection.error"
      title="当前无法访问共享事项"
      :description="projection.error"
      tone="warning"
    /><template v-else-if="projection.data"
      ><CustomerAlert
        title="客户可见内容预览"
        description="此视角由字段白名单生成，不显示内部评论、利润或谈判信息。当前并非真实客户登录会话。"
      /><CustomerSection :title="`${projection.data.customerName} · ${projection.data.title}`" icon="checks"
        ><template #action
          ><StatusBadge :text="projection.data.status || '进度未授权'" tone="primary"
        /></template>
        <p class="customer-help">
          授权 {{ projection.data.grantId }} · 有效期至 {{ projection.data.expires }} · 事项
          {{ projection.data.workId }}
        </p>
        <div v-if="projection.data.sites.length" class="table-scroll" style="margin-top: 20px">
          <table class="data-table">
            <thead>
              <tr>
                <th>本次授权点位</th>
                <th>关联设备</th>
                <th>当前验收记录</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="site in projection.data.sites" :key="site.id">
                <td>{{ site.name }}</td>
                <td>{{ site.device }}</td>
                <td>
                  <StatusBadge
                    :text="site.accepted ? '已通过' : '需要整改 / 确认'"
                    :tone="site.accepted ? 'success' : 'warning'"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="customer-help" style="margin-top: 16px">
          未授权点位明细，不显示完整交付范围。
        </p></CustomerSection
      >
      <div class="customer-split">
        <CustomerSection title="共享交付资料" icon="file"
          ><div v-for="record in projection.data.attachments" :key="record.id" class="customer-record">
            <div>
              <strong>{{ record.id }} · {{ record.title }}</strong>
              <p>仅此记录已授权；其他内部附件不可访问</p>
            </div>
            <StatusBadge :text="record.state" tone="primary" />
          </div>
          <p v-if="!projection.data.attachments.length" class="customer-help">
            本次没有授权附件。
          </p></CustomerSection
        ><CustomerSection title="确认结果与问题反馈" icon="checks"
          ><p class="customer-help">
            仅确认本次授权的交付范围。发现问题时写明点位及整改要求；已通过部分不会被覆盖。
          </p>
          <div class="customer-checklist" style="margin-top: 20px">
            <button
              class="btn btn-primary"
              :disabled="projection.data.status === '已结束'"
              @click="actions.open('client-accept', projection.data.grantId)"
            >
              确认本次交付</button
            ><button
              class="btn"
              :disabled="projection.data.status === '已结束'"
              @click="actions.open('client-reject', projection.data.grantId)"
            >
              存在问题，退回整改
            </button>
          </div></CustomerSection
        >
      </div>
      <CustomerSection title="客户可见活动记录" icon="activity"
        ><div v-for="(activity, index) in projection.data.activities" :key="index" class="customer-activity">
          <span class="activity-dot" />
          <div>
            <small>{{ activity.time }}</small>
            <p>
              <strong>{{ activity.action }}</strong>
            </p>
            <p>{{ activity.detail }}</p>
          </div>
        </div>
        <p v-if="!projection.data.activities.length" class="customer-help">
          暂未发布客户可见的活动记录。
        </p></CustomerSection
      ></template
    >
  </div>
</template>
