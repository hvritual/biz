<script setup lang="ts">
import { provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useUiStore } from '@/stores/ui'
import { customerActionsKey } from '@/composables/customerActions'
import { actionDefinitions } from '@/services/customer/forms'
import type { ActionId } from '@/services/customer/commands'
import type { FormValues } from '@/types/customer'
import CustomerActionDialog from '@/components/customer/CustomerActionDialog.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import '@/styles/customer.css'
const store = useCustomerStore(),
  ui = useUiStore(),
  route = useRoute(),
  router = useRouter()
const open = ref(false),
  action = ref<ActionId>('create-work'),
  id = ref(''),
  selected = ref<string[]>([]),
  defaults = ref<FormValues>({})
function show(kind: ActionId, target = 'CUS-0186', ids: string[] = [], values: FormValues = {}) {
  ui.closeMenu()
  action.value = kind
  id.value = target
  selected.value = ids
  defaults.value = values
  open.value = true
}
provide(customerActionsKey, { open: show })
watch(
  () => route.fullPath,
  () => {
    const candidate = String(route.query.action || '') as ActionId
    if (actionDefinitions[candidate]) {
      show(candidate, String(route.params.id || route.query.target || 'CUS-0186'))
      const query = { ...route.query }
      delete query.action
      delete query.target
      void router.replace({ path: route.path, query })
    }
  },
  { immediate: true },
)
</script>
<template>
  <div class="customer-area">
    <CustomerAlert
      v-if="store.loadError"
      title="预览数据读取失败"
      :description="store.loadError"
      tone="danger"
      ><button class="btn" @click="store.resetPreview">重置当前租户预览数据</button></CustomerAlert
    ><template v-else
      ><CustomerAlert
        v-if="store.lastReceipt"
        class="customer-banner-result"
        title="本地预览操作已保存"
        :description="store.lastReceipt"
        tone="success"
        ><button class="btn-link" @click="store.lastReceipt = ''">收起回执</button></CustomerAlert
      ><RouterView />
      <p class="customer-preview-note">
        界面审核 · 示例数据 · 本地预览操作，不连接合同、财务或设备生产系统
      </p></template
    ><CustomerActionDialog
      :id="id"
      :open="open"
      :action="action"
      :selected="selected"
      :defaults="defaults"
      @close="open = false"
    />
  </div>
</template>
