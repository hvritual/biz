<script setup lang="ts">
import { UiButton } from '@/ui/base'

import { provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useUiStore } from '@/stores/ui'
import { customerActionsKey } from '@/composables/customerActions'
import { actionDefinitions } from '@/services/customer/forms'
import type { ActionId } from '@/services/customer/commands'
import type { FormValues } from '@/types/customer'
import CustomerActionDialog from '@/features/customer/components/CustomerActionDialog.vue'
import CustomerAlert from '@/features/customer/components/CustomerAlert.vue'
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
      title="演示数据读取失败"
      :description="store.loadError"
      tone="danger"
      ><UiButton class="btn" @click="store.resetPreview">重置演示数据</UiButton></CustomerAlert
    ><template v-else
      ><CustomerAlert
        v-if="store.lastReceipt"
        class="customer-banner-result"
        title="操作已保存"
        :description="store.lastReceipt"
        tone="success"
        ><UiButton class="btn-link" @click="store.lastReceipt = ''">收起</UiButton></CustomerAlert
      ><RouterView />
      <p class="customer-preview-note">
        演示内容 · 部分操作仅用于界面体验，不会影响真实合同、财务或设备
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
