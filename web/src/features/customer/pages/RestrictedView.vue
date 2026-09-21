<script setup lang="ts">
import { UiButton, UiTextarea } from '@/ui/base'

import { ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import PageHeading from '@/ui/common/PageHeading.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import CustomerSection from '@/features/customer/components/CustomerSection.vue'
import CustomerAlert from '@/features/customer/components/CustomerAlert.vue'
const store = useCustomerStore(),
  reason = ref(''),
  saved = ref(false),
  error = ref('')
function save() {
  try {
    if (!reason.value.trim()) throw new Error('请填写申请原因')
    store.saveDraft('finance-access-request', { reason: reason.value })
    saved.value = true
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  }
}
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="财务数据访问受限"
      breadcrumb="权限状态预览"
      description="明确区分无权限、无数据和接口失败"
    /><CustomerAlert
      title="当前场景没有财务字段查看权限"
      description="当前账号无权查看财务金额；你仍可查看已授权的事项基本信息。"
      tone="warning"
    />
    <div class="customer-split">
      <CustomerSection title="可见事项范围" icon="checks"
        ><dl class="customer-info-list">
          <dt>关联事项</dt>
          <dd>CS-105 · 9 月应收回款跟进</dd>
          <dt>客户</dt>
          <dd>星悦酒店集团</dd>
          <dt>应收金额</dt>
          <dd>— 无权限</dd>
          <dt>已核销金额</dt>
          <dd>— 无权限</dd>
          <dt>未核销余额</dt>
          <dd>— 无权限</dd>
          <dt>经营利润</dt>
          <dd>— 未授权字段不返回此视图</dd>
        </dl></CustomerSection
      ><CustomerSection title="申请权限或联系管理员" icon="lock"
        ><div class="customer-empty" style="padding: 15px">
          <AppIcon name="shield" :size="42" /><strong>需要财务数据范围授权</strong>
          <p class="customer-help">拥有事项处理权限，不自动拥有金额和利润查看权限。</p>
        </div>
        <form class="page-stack" @submit.prevent="save">
          <label class="field"
            ><span class="required">申请原因</span
            ><UiTextarea
              v-model="reason"
              class="textarea"
              required
              placeholder="说明需要查看的范围与业务用途"
            />
          </label>
          <p v-if="error" class="form-error" role="alert">{{ error }}</p>
          <CustomerAlert
            v-if="saved"
            title="申请草稿已在本地保存"
            description="未向管理员或生产权限服务发送；需要接入真实审批服务后提交。"
            tone="success"
          /><UiButton class="btn btn-primary" type="submit">保存申请草稿</UiButton>
        </form></CustomerSection
      >
    </div>
    <RouterLink to="/customers/work" class="btn" style="align-self: flex-start">返回可见事项</RouterLink>
  </div>
</template>
