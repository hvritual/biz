<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import { prepareMemberStatusBatch } from '@/services/memberPolicy'
import UiDialog from '@/components/ui/UiDialog.vue'
const props = defineProps<{
  open: boolean
  action: 'activate' | 'suspend'
  targets: { id: string; version: number }[]
}>()
const emit = defineEmits<{ close: []; saved: [] }>()
const store = useEnterpriseStore(),
  ui = useUiStore()
const reason = ref(''),
  confirmed = ref(false),
  error = ref('')
const label = computed(() => (props.action === 'activate' ? '批量启用' : '批量停用'))
const policyError = computed(() => {
  if (!props.open) return ''
  try {
    prepareMemberStatusBatch(store.members, store.roles, props.targets, props.action)
    return ''
  } catch (e) {
    return (e as Error).message
  }
})
watch(
  () => props.open,
  () => {
    reason.value = ''
    confirmed.value = false
    error.value = ''
  },
)
function submit() {
  if (policyError.value) return
  if (!reason.value.trim()) {
    error.value = '请填写操作原因。'
    return
  }
  if (!confirmed.value) {
    error.value = '请确认已核对成员及影响范围。'
    return
  }
  try {
    store.changeStatuses(props.targets, props.action, reason.value.trim())
    ui.toast(`已${label.value} ${props.targets.length} 位成员（本地预览），未修改真实账号。`)
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
  }
}
</script>
<template>
  <UiDialog :open="open" :title="label + '成员'" width="500px" @close="emit('close')">
    <div class="notice-box">
      将{{ label }}当前页所选的 {{ targets.length }} 位成员。停用仅影响当前企业访问，不会删除历史数据。
    </div>
    <p class="selected-names">
      {{ targets.map((t) => store.members.find((m) => m.id === t.id)?.name).join('、') }}
    </p>
    <form id="member-batch-form" class="page-stack" @submit.prevent="submit">
      <label class="field"
        ><span class="required">操作原因</span
        ><textarea
          v-model="reason"
          class="textarea"
          aria-label="操作原因"
          maxlength="200"
          placeholder="请说明本次操作的原因"
        />
      </label>
      <label class="confirm-check"
        ><input v-model="confirmed" type="checkbox" />我已核对所选成员及操作影响范围</label
      >
      <p v-if="policyError || error" class="form-error" role="alert">{{ policyError || error }}</p>
    </form>
    <template #footer
      ><button class="btn" @click="emit('close')">取消</button
      ><button
        class="btn"
        :class="action === 'suspend' ? 'btn-danger' : 'btn-primary'"
        :disabled="Boolean(policyError)"
        type="submit"
        form="member-batch-form"
      >
        确认{{ label }}
      </button></template
    >
  </UiDialog>
</template>
<style scoped>
.selected-names {
  padding: 16px 0;
  font-size: 13px;
  color: var(--color-text-secondary);
}
.confirm-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}
</style>
