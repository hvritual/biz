<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import { departmentStatusLabel, type EnterpriseDepartment } from '@/services/enterprise/departmentRuntime'
import type { EnterpriseTenantMember } from '@/services/enterprise/memberRuntime'

const props = defineProps<{
  department: EnterpriseDepartment | null
  departments: EnterpriseDepartment[]
  members: EnterpriseTenantMember[]
  busy: boolean
}>()
const emit = defineEmits<{ edit: [department: EnterpriseDepartment] }>()

const selectedMembers = computed(() => {
  if (!props.department) return []
  return props.members.filter((item) => item.departmentId === props.department!.departmentId && item.status !== 'TENANT_MEMBER_STATUS_REMOVED')
})

function memberLabel(userId: string) {
  if (!userId) return '未设置'
  const member = props.members.find((item) => item.userId === userId)
  return member ? (member.name || member.email || member.userId) : userId
}

function parentLabel(parentId: string) {
  if (!parentId) return '顶级部门'
  return props.departments.find((item) => item.departmentId === parentId)?.name ?? parentId
}
</script>

<template>
  <section class="card organization-detail" aria-label="部门服务端详情">
    <template v-if="department">
      <header class="detail-head">
        <div>
          <span class="eyebrow">{{ parentLabel(department.parentId) }}</span>
          <h2>{{ department.name }}</h2>
          <p>{{ department.departmentId }} · v{{ department.version }}</p>
        </div>
        <button class="btn" :disabled="busy" @click="emit('edit', department)"><AppIcon name="edit" :size="15" />编辑部门</button>
      </header>
      <div class="detail-grid">
        <div><span>状态</span><strong>{{ departmentStatusLabel(department.status) }}</strong></div>
        <div><span>负责人</span><strong>{{ memberLabel(department.leaderUserId) }}</strong></div>
        <div><span>直属成员</span><strong>{{ selectedMembers.length }}</strong></div>
        <div><span>排序</span><strong>{{ department.sort }}</strong></div>
        <div><span>联系邮箱</span><strong>{{ department.email || '未设置' }}</strong></div>
        <div><span>联系电话</span><strong>{{ department.phone || '未设置' }}</strong></div>
      </div>
      <div v-if="department.status === 'TENANT_DEPARTMENT_STATUS_DISABLED'" class="organization-rule-note">
        已停用部门保留既有成员归属；服务端会拒绝新的成员转入或从其他部门转入。
      </div>
      <div class="member-section">
        <div class="section-head"><div><strong>直属成员</strong><span>来自成员服务端读模型</span></div></div>
        <div class="table-scroll">
          <table>
            <thead><tr><th>成员</th><th>职位</th><th>状态</th><th>角色</th></tr></thead>
            <tbody>
              <tr v-for="member in selectedMembers" :key="member.userId">
                <td><strong>{{ member.name || member.email }}</strong><small>{{ member.userId }}</small></td>
                <td>{{ member.position || '—' }}</td>
                <td>{{ member.status === 'TENANT_MEMBER_STATUS_ACTIVE' ? '已启用' : member.status }}</td>
                <td>{{ member.roles.map((role) => role.roleName).join('、') || '未分配角色' }}</td>
              </tr>
              <tr v-if="!selectedMembers.length"><td colspan="4" class="empty-cell">当前部门暂无直属成员。</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
    <div v-else class="empty-state detail-empty">选择一个部门查看服务端详情，或新建第一个部门。</div>
  </section>
</template>

<style scoped>
.organization-detail { min-width: 0; overflow: hidden; }
.detail-empty { min-height: 280px; display: grid; place-items: center; }
.empty-state { padding: 28px 18px; color: var(--color-text-muted); text-align: center; }
.detail-head { padding: 20px; display: flex; justify-content: space-between; gap: 16px; border-bottom: 1px solid var(--color-border); }
.detail-head h2 { margin: 4px 0; font-size: 21px; }
.detail-head p, .eyebrow { margin: 0; color: var(--color-text-muted); font-size: 12px; }
.detail-grid { padding: 18px 20px; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.detail-grid div { min-width: 0; display: grid; gap: 4px; }
.detail-grid span { color: var(--color-text-muted); font-size: 12px; }
.detail-grid strong { overflow-wrap: anywhere; }
.organization-rule-note { margin: 0 20px 18px; padding: 12px 14px; border-radius: var(--radius-md); background: var(--color-primary-soft); color: var(--color-text-secondary); font-size: 13px; line-height: 1.6; }
.member-section { border-top: 1px solid var(--color-border); }
.section-head { padding: 16px 18px; border-bottom: 1px solid var(--color-border); }
.section-head div { display: grid; gap: 2px; }
.section-head span { color: var(--color-text-muted); font-size: 12px; font-weight: 400; }
.table-scroll { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; min-width: 620px; }
th, td { text-align: left; padding: 12px 16px; border-bottom: 1px solid var(--color-border); font-size: 13px; vertical-align: top; }
th { color: var(--color-text-muted); font-weight: 600; }
td strong, td small { display: block; }
td small { margin-top: 3px; color: var(--color-text-muted); }
.empty-cell { text-align: center; color: var(--color-text-muted); padding: 24px; }
@media (max-width: 600px) {
  .detail-head { flex-direction: column; align-items: stretch; }
  .detail-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
