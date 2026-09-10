<script setup lang="ts">
import { computed } from 'vue'
import type { Member } from '@/types/enterprise'
import { statusLabels } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
const props = defineProps<{ members: Member[]; selected: string[] }>()
const emit = defineEmits<{
  'update:selected': [ids: string[]]
  detail: [member: Member]
  edit: [member: Member]
  more: [member: Member]
}>()
const store = useEnterpriseStore()
const allSelected = computed(
  () => props.members.length > 0 && props.members.every((m) => props.selected.includes(m.id)),
)
function toggle(id: string) {
  emit(
    'update:selected',
    props.selected.includes(id) ? props.selected.filter((i) => i !== id) : [...props.selected, id],
  )
}
function togglePage() {
  emit('update:selected', allSelected.value ? [] : props.members.map((m) => m.id))
}
</script>
<template>
  <div class="table-scroll">
    <table class="data-table member-table">
      <thead>
        <tr>
          <th class="check-cell">
            <input
              type="checkbox"
              aria-label="选择当前页全部成员"
              :checked="allSelected"
              :indeterminate="selected.length > 0 && !allSelected"
              @change="togglePage"
            />
          </th>
          <th>成员信息</th>
          <th>所属部门</th>
          <th>角色</th>
          <th>账号状态</th>
          <th>最后登录时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="member in members" :key="member.id" :class="{ selected: selected.includes(member.id) }">
          <td class="check-cell">
            <input
              type="checkbox"
              :aria-label="'选择 ' + member.name"
              :checked="selected.includes(member.id)"
              @change="toggle(member.id)"
            />
          </td>
          <td>
            <button class="member-identity" @click="emit('detail', member)">
              <AvatarMark :name="member.name" /><span
                ><strong>{{ member.name }}</strong
                ><small>{{ member.email }}</small></span
              >
            </button>
          </td>
          <td>{{ store.departmentName(member.departmentId) }}</td>
          <td>
            <span
              v-for="role in member.roleIds"
              :key="role"
              :class="['role-tag', { plain: role === 'role-2' }]"
              >{{ store.roleName(role) }}</span
            >
          </td>
          <td>
            <StatusBadge
              :text="statusLabels[member.status]"
              :tone="
                member.status === 'active'
                  ? 'success'
                  : member.status === 'invited'
                    ? 'warning'
                    : member.status === 'suspended'
                      ? 'danger'
                      : 'neutral'
              "
            />
          </td>
          <td class="muted numeric">{{ member.lastLogin || '尚未登录' }}</td>
          <td>
            <div class="table-actions">
              <button class="btn-link" :aria-label="'编辑 ' + member.name" @click="emit('edit', member)">
                编辑</button
              ><button
                class="icon-button"
                :aria-label="member.name + ' 更多操作'"
                @click="emit('more', member)"
              >
                <AppIcon name="more" :size="19" />
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
<style scoped>
.member-table {
  min-width: 960px;
}
.member-identity {
  display: flex;
  align-items: center;
  gap: 12px;
  text-align: left;
  padding: 0;
}
.member-identity strong {
  font-size: 13px;
  font-weight: 550;
}
.member-identity small {
  display: block;
  font-size: 11px;
  color: var(--color-text-muted);
  margin-top: 3px;
}
.member-identity:hover strong {
  color: var(--color-primary);
}
.role-tag {
  display: inline-block;
  font-size: 11px;
  color: var(--color-primary);
  background: var(--color-primary-soft);
  padding: 4px 7px;
  border-radius: 5px;
  margin: 2px;
}
.role-tag.plain {
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
}
.member-table td {
  height: 62px;
}
.member-table td:nth-child(3) {
  font-size: 12px;
}
.table-actions {
  gap: 8px;
}
@media (max-height: 830px) and (min-width: 768px) {
  .member-table td {
    height: 52px;
  }
}
</style>
