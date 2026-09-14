<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'

import { computed, ref, watch } from 'vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import type { EnterpriseDepartment, EnterpriseDepartmentDraft } from '@/services/enterprise/departmentRuntime'
import type { EnterpriseTenantMember } from '@/services/enterprise/memberRuntime'

const props = defineProps<{
  open: boolean
  department: EnterpriseDepartment | null
  departments: EnterpriseDepartment[]
  members: EnterpriseTenantMember[]
  busy: boolean
  error: string
}>()
const emit = defineEmits<{
  close: []
  submit: [draft: EnterpriseDepartmentDraft]
}>()

const draft = ref<EnterpriseDepartmentDraft>(emptyDraft())
const activeMembers = computed(() => props.members.filter((item) => item.status === 'TENANT_MEMBER_STATUS_ACTIVE'))
const parentOptions = computed(() => {
  const forbidden = props.department
    ? new Set([props.department.departmentId, ...descendantIds(props.department.departmentId)])
    : new Set<string>()
  return props.departments.filter((item) => item.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE' && !forbidden.has(item.departmentId))
})

function emptyDraft(): EnterpriseDepartmentDraft {
  return { name: '', parentId: '', leaderUserId: '', email: '', phone: '', sort: 0, enabled: true }
}

function reset() {
  const department = props.department
  draft.value = department
    ? {
        name: department.name,
        parentId: department.parentId,
        leaderUserId: department.leaderUserId,
        email: department.email,
        phone: department.phone,
        sort: department.sort,
        enabled: department.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE',
      }
    : emptyDraft()
}

function descendantIds(departmentId: string) {
  const result: string[] = []
  const queue = [departmentId]
  const seen = new Set(queue)
  while (queue.length) {
    const current = queue.shift()!
    for (const item of props.departments) {
      if (item.parentId !== current || seen.has(item.departmentId)) continue
      seen.add(item.departmentId)
      result.push(item.departmentId)
      queue.push(item.departmentId)
    }
  }
  return result
}

function submit() {
  emit('submit', {
    ...draft.value,
    name: draft.value.name.trim(),
    parentId: draft.value.parentId.trim(),
    leaderUserId: draft.value.leaderUserId.trim(),
    email: draft.value.email.trim(),
    phone: draft.value.phone.trim(),
    sort: Number(draft.value.sort) || 0,
  })
}

watch(() => [props.open, props.department?.departmentId, props.department?.version], reset, { immediate: true })
</script>

<template>
  <UiDialog :open="open" :title="department ? '编辑部门' : '新建部门'" width="660px" @close="!busy && emit('close')">
    <div class="editor-form">
      <label class="field"><span>部门名称 *</span><UiInput v-model="draft.name" maxlength="100" placeholder="例如：客户成功部" /></label>
      <label class="field"><span>上级部门</span><UiSelect v-model="draft.parentId"><UiOption value="">顶级部门</UiOption><UiOption v-for="item in parentOptions" :key="item.departmentId" :value="item.departmentId">{{ item.name }}</UiOption></UiSelect></label>
      <label class="field"><span>负责人</span><UiSelect v-model="draft.leaderUserId"><UiOption value="">未设置</UiOption><UiOption v-for="member in activeMembers" :key="member.userId" :value="member.userId">{{ member.name || member.email || member.userId }}</UiOption></UiSelect></label>
      <label class="field"><span>排序</span><UiInput v-model.number="draft.sort" type="number" min="0" step="1" /></label>
      <label class="field"><span>联系邮箱</span><UiInput v-model="draft.email" type="email" maxlength="320" placeholder="department@example.com" /></label>
      <label class="field"><span>联系电话</span><UiInput v-model="draft.phone" maxlength="40" placeholder="可选" /></label>
      <label class="switch-field"><UiInput v-model="draft.enabled" type="checkbox" /><span><strong>启用部门</strong><small>停用后保留历史归属，但禁止新的成员转入。</small></span></label>
      <p v-if="error" class="notice-box organization-error" role="alert">{{ error }}</p>
    </div>
    <template #footer>
      <UiButton class="btn" :disabled="busy" @click="emit('close')">取消</UiButton>
      <UiButton class="btn btn-primary" :disabled="busy" @click="submit">{{ busy ? '提交中…' : '提交并回读确认' }}</UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.editor-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.field { display: grid; gap: 6px; min-width: 0; }
.field span { font-size: 13px; font-weight: 600; }
.field input, .field select { width: 100%; min-width: 0; }
.switch-field { grid-column: 1 / -1; display: flex; align-items: flex-start; gap: 10px; padding: 12px; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
.switch-field input { margin-top: 3px; }
.switch-field span { display: grid; gap: 3px; }
.switch-field small { color: var(--color-text-muted); line-height: 1.5; }
.editor-form .notice-box { grid-column: 1 / -1; }
@media (max-width: 600px) { .editor-form { grid-template-columns: 1fr; } }
</style>
