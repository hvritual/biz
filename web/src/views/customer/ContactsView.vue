<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import PageHeading from '@/components/ui/PageHeading.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerIdentity from '@/components/customer/CustomerIdentity.vue'
import CustomerTabs from '@/components/customer/CustomerTabs.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  route = useRoute(),
  actions = useCustomerActions()
const customer = computed(() =>
  store.snapshot.customers.find((c) => c.id === String(route.query.customer || 'CUS-0186'))!,
)
const contacts = computed(() => store.snapshot.contacts.filter((c) => c.customerId === customer.value.id))
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="联系人与协作团队"
      breadcrumb="客户经营"
      description="客户对接人、内部责任人与客户侧授权分别管理"
      ><div class="customer-heading-actions">
        <button class="btn btn-primary" @click="actions.open('contact', customer.id)">新增联系人</button>
      </div></PageHeading
    ><CustomerIdentity :customer="customer" /><CustomerTabs :id="customer.id" active="contacts" />
    <div class="customer-split">
      <CustomerSection title="客户侧联系人" icon="users"
        ><div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>联系人</th>
                <th>职务与职责</th>
                <th>联系方式</th>
                <th>访问状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="contact in contacts" :key="contact.id">
                <td>
                  <div class="row">
                    <AvatarMark :name="contact.name" :size="32" /><strong>{{ contact.name }}</strong>
                  </div>
                </td>
                <td>
                  {{ contact.role }}
                  <p class="subline">{{ contact.responsibility }}</p>
                </td>
                <td>
                  {{ contact.phone }}
                  <p class="subline">{{ contact.email }}</p>
                </td>
                <td>
                  <StatusBadge
                    :text="contact.authorized ? '已配置授权' : '仅联系人'"
                    :tone="contact.authorized ? 'primary' : 'neutral'"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <CustomerAlert
          title="联系信息不代表访问权限"
          description="客户侧必须有对象、字段、附件及有效期授权；联系人资料与登录身份不混用。"
        /><RouterLink class="btn-link" to="/customers/sharing" style="margin-top: 16px"
          >管理共享范围</RouterLink
        ></CustomerSection
      >
      <div class="page-stack">
        <CustomerSection title="内部协作分工" icon="organization"
          ><div
            v-for="(role, name) in {
              张敏: '客户关系与续约推进',
              李川: '点位交付与退租回收',
              陈晓: '服务协调与恢复验证',
              王宁: '应收核对与回款跟进',
            }"
            :key="name"
            class="customer-record"
          >
            <div class="row">
              <AvatarMark :name="name" :size="30" />
              <div>
                <strong>{{ name }}</strong>
                <p>{{ role }}</p>
              </div>
            </div>
          </div>
          <button class="btn-link" style="margin-top: 12px" @click="actions.open('handover', customer.id)">
            移交客户与可编辑事项
          </button></CustomerSection
        ><CustomerSection title="责任边界" icon="shield"
          ><p class="customer-help">
            客户负责人负责长期关系；事项负责人负责一件具体工作。移交客户不默认接管其他团队的事项。
          </p></CustomerSection
        >
      </div>
    </div>
  </div>
</template>
