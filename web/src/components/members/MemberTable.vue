<script setup lang="ts">
import { computed } from 'vue'
import type { Member } from '@/types/enterprise'
import { scopeLabels, statusLabels } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import MemberRoleTags from './MemberRoleTags.vue'
export type MemberSortKey = 'name' | 'departmentId' | 'status' | 'lastLogin' | 'joinedAt'
const props = defineProps<{
  members: Member[]
  selected: string[]
  detailId?: string | null
  sortKey?: MemberSortKey
  sortDirection?: 'asc' | 'desc'
}>()
const emit = defineEmits<{
  'update:selected': [ids: string[]]
  detail: [member: Member]
  edit: [member: Member]
  more: [member: Member]
  sort: [key: MemberSortKey]
}>()
const store = useEnterpriseStore()
const allSelected = computed(
  () => props.members.length > 0 && props.members.every((m) => props.selected.includes(m.id)),
)
const columns: { label: string; key?: MemberSortKey; className: string }[] = [
  { label: '姓名', key: 'name', className: 'name-column' },
  { label: '手机号 / 邮箱', className: 'contact-column' },
  { label: '所属部门', key: 'departmentId', className: 'department-column' },
  { label: '角色', className: 'role-column' },
  { label: '数据权限', className: 'scope-column' },
  { label: '状态', key: 'status', className: 'status-column' },
  { label: '最后登录', key: 'lastLogin', className: 'login-column' },
  { label: '加入时间', key: 'joinedAt', className: 'joined-column' },
  { label: '操作', className: 'actions-column' },
]
function toggle(id: string) {
  emit(
    'update:selected',
    props.selected.includes(id) ? props.selected.filter((i) => i !== id) : [...props.selected, id],
  )
}
</script>
<template>
  <div class="table-scroll member-table-scroll" tabindex="0" aria-label="成员列表，可横向滚动">
    <table class="data-table member-table">
      <colgroup>
        <col class="check-column" />
        <col v-for="column in columns" :key="column.label" :class="column.className" />
      </colgroup>
      <thead>
        <tr>
          <th class="check-cell">
            <input
              type="checkbox"
              aria-label="选择当前页全部成员"
              :checked="allSelected"
              :indeterminate="selected.length > 0 && !allSelected"
              @change="emit('update:selected', allSelected ? [] : members.map((m) => m.id))"
            />
          </th>
          <th
            v-for="column in columns"
            :key="column.label"
            :class="column.className"
            :aria-sort="
              column.key === sortKey ? (sortDirection === 'desc' ? 'descending' : 'ascending') : undefined
            "
          >
            <button
              v-if="column.key"
              class="column-sort"
              :aria-label="`按${column.label}排序`"
              @click="emit('sort', column.key)"
            >
              {{ column.label
              }}<span class="sort-indicator" :class="{ sorted: column.key === sortKey }" aria-hidden="true"
                ><AppIcon
                  name="down"
                  :size="10"
                  :class="{ ascending: column.key === sortKey && sortDirection !== 'desc' }"
              /></span></button
            ><template v-else>{{ column.label }}</template>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="member in members"
          :key="member.id"
          :class="{ selected: selected.includes(member.id), viewing: detailId === member.id }"
          :data-member-id="member.id"
        >
          <td class="check-cell">
            <input
              type="checkbox"
              :aria-label="'选择 ' + member.name"
              :checked="selected.includes(member.id)"
              @change="toggle(member.id)"
            />
          </td>
          <td>
            <button
              class="member-identity"
              :aria-label="`查看 ${member.name}资料`"
              @click="emit('detail', member)"
            >
              <AvatarMark
                :name="member.name"
                :size="28"
                :tone="member.roleIds.includes('owner') ? 'slate' : 'solid'"
              /><strong :title="member.name">{{ member.name }}</strong>
            </button>
          </td>
          <td class="contact-cell">
            <span :title="`${member.phone} / ${member.email}`">{{ member.email }}</span>
          </td>
          <td class="department-cell">
            <span :title="store.departmentName(member.departmentId)">{{
              store.departmentName(member.departmentId)
            }}</span>
          </td>
          <td><MemberRoleTags :ids="member.roleIds" /></td>
          <td class="scope-cell">
            <span :title="scopeLabels[member.scope]">{{ scopeLabels[member.scope] }}</span>
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
          <td class="muted numeric time-cell">{{ member.lastLogin || '尚未登录' }}</td>
          <td class="muted numeric time-cell">{{ member.joinedAt }}</td>
          <td class="actions-column">
            <div class="table-actions">
              <button
                class="btn-link"
                :aria-label="'查看 ' + member.name"
                :aria-expanded="detailId === member.id"
                aria-controls="member-detail-panel"
                @click="emit('detail', member)"
              >
                查看
              </button>
              <button class="btn-link" :aria-label="'编辑 ' + member.name" @click="emit('edit', member)">
                编辑
              </button>
              <button
                class="icon-button"
                :aria-label="member.name + ' 更多操作'"
                @click="emit('more', member)"
              >
                <AppIcon name="more" :size="17" />
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
  min-width: 860px;
  table-layout: fixed;
  font-size: 12px;
}
.check-column {
  width: 30px;
}
.name-column {
  width: 88px;
}
.department-column {
  width: 82px;
}
.role-column {
  width: 90px;
}
.scope-column {
  width: 80px;
}
.status-column {
  width: 64px;
}
.login-column {
  width: 114px;
}
.joined-column {
  width: 82px;
}
.actions-column {
  width: 92px;
}
.member-table th {
  height: 40px;
  padding: 0 6px;
  font-size: 11px;
}
.member-table td {
  height: 46px;
  padding: 6px;
}
.member-table .check-cell {
  padding-left: 8px;
  padding-right: 0;
}
.member-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  padding: 0;
  width: 100%;
}
.member-identity strong {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  font-weight: 550;
}
.member-identity:hover strong {
  color: var(--color-primary);
}
.contact-cell span,
.department-cell span,
.scope-cell span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.contact-cell {
  color: var(--color-text-secondary);
  font-size: 11px;
}
.department-cell,
.scope-cell {
  font-size: 11px;
}
.time-cell {
  font-size: 10.5px;
  letter-spacing: -0.1px;
}
.member-table :deep(.status-badge) {
  font-size: 11px;
  padding: 2px 5px;
}
.table-actions {
  gap: 5px;
}
.table-actions .btn-link {
  font-size: 11px;
  padding: 3px 0;
}
.table-actions .icon-button {
  width: 22px;
  height: 28px;
  color: var(--color-primary);
}
.column-sort {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  font-size: inherit;
  text-align: left;
}
.sort-indicator {
  color: var(--color-text-muted);
}
.sort-indicator.sorted {
  color: var(--color-primary);
}
.ascending {
  transform: rotate(180deg);
}
.member-table tr.viewing {
  background: var(--color-primary-soft);
}
.member-table tr.viewing td:first-child {
  box-shadow: inset 2px 0 var(--color-primary);
}
.member-table .actions-column {
  position: sticky;
  right: 0;
  background: var(--color-surface);
}
.member-table th.actions-column {
  background: var(--color-surface-soft);
}
.member-table tr.selected .actions-column,
.member-table tr.viewing .actions-column {
  background: var(--color-primary-soft);
}
.member-table tr:hover:not(.selected):not(.viewing) .actions-column {
  background: var(--color-surface-soft);
}
@container member-main (min-width: 1120px) {
  .name-column {
    width: 108px;
  }
  .department-column {
    width: 108px;
  }
  .role-column {
    width: 116px;
  }
  .scope-column {
    width: 100px;
  }
  .status-column {
    width: 90px;
  }
  .login-column {
    width: 146px;
  }
  .joined-column {
    width: 106px;
  }
  .actions-column {
    width: 110px;
  }
  .member-table th {
    font-size: 12px;
  }
  .member-table td {
    height: 49px;
  }
  .time-cell,
  .contact-cell,
  .department-cell,
  .scope-cell {
    font-size: 12px;
  }
  .table-actions {
    gap: 10px;
  }
  .table-actions .btn-link {
    font-size: 12px;
  }
}
</style>
