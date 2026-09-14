<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import { departmentStatusLabel, type EnterpriseDepartment } from '@/services/enterprise/departmentRuntime'
import type { EnterpriseTenantMember } from '@/services/enterprise/memberRuntime'

const props = defineProps<{
  departments: EnterpriseDepartment[]
  members: EnterpriseTenantMember[]
  selectedId: string
  query: string
}>()
const emit = defineEmits<{ select: [departmentId: string] }>()

const memberCount = computed(() => {
  const counts: Record<string, number> = {}
  for (const member of props.members) {
    if (!member.departmentId || member.status === 'TENANT_MEMBER_STATUS_REMOVED') continue
    counts[member.departmentId] = (counts[member.departmentId] ?? 0) + 1
  }
  return counts
})

function memberLabel(userId: string) {
  if (!userId) return '未设置'
  const member = props.members.find((item) => item.userId === userId)
  return member ? (member.name || member.email || member.userId) : userId
}

function flattenDepartments(values: EnterpriseDepartment[]) {
  const ordered = [...values].sort((a, b) => a.sort - b.sort || a.name.localeCompare(b.name) || a.departmentId.localeCompare(b.departmentId))
  const ids = new Set(ordered.map((item) => item.departmentId))
  const children = new Map<string, EnterpriseDepartment[]>()
  for (const item of ordered) {
    const parent = item.parentId && ids.has(item.parentId) ? item.parentId : ''
    const rows = children.get(parent) ?? []
    rows.push(item)
    children.set(parent, rows)
  }
  const result: Array<{ department: EnterpriseDepartment; depth: number }> = []
  const seen = new Set<string>()
  const walk = (parentId: string, depth: number) => {
    for (const item of children.get(parentId) ?? []) {
      if (seen.has(item.departmentId)) continue
      seen.add(item.departmentId)
      result.push({ department: item, depth })
      walk(item.departmentId, depth + 1)
    }
  }
  walk('', 0)
  for (const item of ordered) {
    if (!seen.has(item.departmentId)) result.push({ department: item, depth: 0 })
  }
  return result
}

const visibleRows = computed(() => {
  const rows = flattenDepartments(props.departments)
  const value = props.query.trim().toLowerCase()
  if (!value) return rows
  return rows.filter(({ department }) => {
    const leader = memberLabel(department.leaderUserId)
    return `${department.name} ${department.email} ${department.phone} ${leader} ${department.departmentId}`.toLowerCase().includes(value)
  })
})
</script>

<template>
  <section class="card organization-tree" aria-label="服务端部门树">
    <header class="section-head">
      <div><strong>部门树</strong><span>按服务端 sort 与层级排列</span></div>
    </header>
    <div v-if="visibleRows.length" class="tree-list">
      <button
        v-for="row in visibleRows"
        :key="row.department.departmentId"
        class="tree-row"
        :class="{ selected: row.department.departmentId === selectedId }"
        :style="{ '--depth': String(row.depth) }"
        @click="emit('select', row.department.departmentId)"
      >
        <span class="tree-line"><AppIcon name="organization" :size="15" />{{ row.department.name }}</span>
        <span class="tree-meta">
          <i :class="['status-dot', row.department.status === 'TENANT_DEPARTMENT_STATUS_ACTIVE' ? 'success' : 'muted']" />
          {{ departmentStatusLabel(row.department.status) }} · {{ memberCount[row.department.departmentId] ?? 0 }} 人
        </span>
      </button>
    </div>
    <div v-else class="empty-state">暂无匹配的服务端部门。</div>
  </section>
</template>

<style scoped>
.organization-tree { min-width: 0; overflow: hidden; }
.section-head { padding: 16px 18px; border-bottom: 1px solid var(--color-border); }
.section-head div { display: grid; gap: 2px; }
.section-head span { color: var(--color-text-muted); font-size: 12px; font-weight: 400; }
.tree-list { padding: 8px; display: grid; gap: 3px; }
.tree-row { width: 100%; border: 0; background: transparent; border-radius: var(--radius-md); padding: 10px 10px 10px calc(10px + var(--depth) * 18px); text-align: left; cursor: pointer; color: inherit; display: grid; gap: 4px; }
.tree-row:hover, .tree-row.selected { background: var(--color-primary-soft); }
.tree-line { display: flex; align-items: center; gap: 7px; font-weight: 650; }
.tree-meta { display: flex; align-items: center; gap: 6px; padding-left: 22px; color: var(--color-text-muted); font-size: 12px; }
.status-dot { width: 7px; height: 7px; border-radius: 999px; background: var(--color-text-muted); }
.status-dot.success { background: var(--color-success); }
.status-dot.muted { opacity: .45; }
.empty-state { padding: 28px 18px; color: var(--color-text-muted); text-align: center; }
</style>
