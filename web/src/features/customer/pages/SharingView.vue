<script setup lang="ts">
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  actions = useCustomerActions()
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="客户共享与可见性"
      breadcrumb="客户经营"
      description="客户也是平台租户，但不能默认看到你的内部经营资料"
      ><div class="customer-heading-actions">
        <button class="btn btn-primary" @click="actions.open('share', 'CUS-0186')">新建共享授权</button>
      </div></PageHeading
    ><CustomerAlert
      title="授权到对象、字段、附件及有效期"
      description="内部评论、谈判策略和利润不对客户开放。客户确认只覆盖本次被授权的交付范围。"
    /><CustomerSection title="客户侧共享授权" icon="shield"
      ><div class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>授权对象</th>
              <th>接收人</th>
              <th>共享事项</th>
              <th>字段范围</th>
              <th>到期时间</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="grant in store.snapshot.grants" :key="grant.id">
              <td>
                <strong>{{ store.customerName(grant.customerId) }}</strong>
                <p class="subline">{{ grant.id }}</p>
              </td>
              <td>
                {{ store.snapshot.contacts.find((c) => c.id === grant.contactId)?.name }}
                <p class="subline">明确授权的客户联系人</p>
              </td>
              <td>
                <RouterLink :to="`/customers/work/${grant.workId}`" class="btn-link">{{
                  grant.workId
                }}</RouterLink>
              </td>
              <td>
                <span class="customer-help">{{ grant.fields.join('、') }}</span>
              </td>
              <td>{{ grant.expires }}</td>
              <td>
                <StatusBadge
                  :text="grant.revoked ? '已撤销' : '已授权'"
                  :tone="grant.revoked ? 'neutral' : 'success'"
                />
              </td>
              <td>
                <div class="table-actions">
                  <RouterLink :to="`/customers/client/${grant.id}`" class="btn-link">客户视角</RouterLink
                  ><button
                    class="btn-link"
                    :disabled="grant.revoked"
                    @click="actions.open('revoke-share', grant.id)"
                  >
                    撤销
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div></CustomerSection
    >
    <div class="customer-split">
      <CustomerSection title="共享规则" icon="eye"
        ><div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>信息类型</th>
                <th>客户侧可见</th>
                <th>依据</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>事项标题、交付范围、处理进度</td>
                <td>按授权</td>
                <td>明确字段清单</td>
              </tr>
              <tr>
                <td>交付与验收附件</td>
                <td>按附件编号授权</td>
                <td>属于同一客户</td>
              </tr>
              <tr>
                <td>客户可见评论</td>
                <td>仅已授权事项的公开记录</td>
                <td>评论级可见性</td>
              </tr>
              <tr>
                <td>内部评论、利润、谈判策略</td>
                <td>不共享</td>
                <td>经营方内部资料</td>
              </tr>
              <tr>
                <td>设备解绑后的新数据</td>
                <td>不可见</td>
                <td>投放有效期截止</td>
              </tr>
            </tbody>
          </table>
        </div></CustomerSection
      ><CustomerSection title="客户侧协作闭环" icon="checks"
        ><div class="customer-checklist">
          <p>服务方授权范围 → 客户读取受限投影</p>
          <p>客户确认 / 提出问题 → 原事项记录反馈</p>
          <p>未通过范围整改 → 再次确认</p>
          <p>服务方核验业务来源 → 事项成功结束</p>
        </div>
        <CustomerAlert
          title="本地预览不是公开授权链接"
          description="生产登录、邀请发送与服务端字段投影尚未接入；这里验证界面及本地范围规则。"
          style="margin-top: 20px"
      /></CustomerSection>
    </div>
  </div>
</template>
