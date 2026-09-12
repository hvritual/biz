<script setup lang="ts">
import { computed } from 'vue'
import type { Member } from '@/types/enterprise'
import { scopeLabels, statusLabels } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import MemberRoleTags from './MemberRoleTags.vue'
const props = defineProps<{ member: Member }>()
const emit = defineEmits<{ logs: [] }>()
const store = useEnterpriseStore()
const department = computed(() => store.departments.find((d) => d.id === props.member.departmentId))
const leader = computed(() => store.members.find((m) => m.id === department.value?.leaderId))
const recent = computed(() => store.logs.filter((l) => l.target === props.member.name).slice(0, 3))
const scopeDescription = computed(
  () =>
    ({
      all: '可访问当前企业授权范围内的全部业务数据。',
      department: '仅可访问所属部门的已授权业务数据。',
      department_tree: '可访问本部门及下级部门的已授权数据。',
      self: '仅可访问与本人相关的已授权数据。',
      custom: '按指定范围授权，具体对象以服务端规则为准。',
    })[props.member.scope],
)
</script>
<template>
  <section class="detail-section">
    <h3>个人信息</h3>
    <dl class="member-facts">
      <dt>姓名</dt>
      <dd>{{ member.name }}</dd>
      <dt>手机号</dt>
      <dd>{{ member.phone || '未填写' }}</dd>
      <dt>邮箱</dt>
      <dd>{{ member.email }}</dd>
      <dt>工号</dt>
      <dd>{{ member.employeeId || '未填写' }}</dd>
      <dt>账号状态</dt>
      <dd>
        <StatusBadge
          :text="statusLabels[member.status]"
          :tone="
            member.status === 'active' ? 'success' : member.status === 'suspended' ? 'danger' : 'neutral'
          "
        />
      </dd>
      <dt>加入时间</dt>
      <dd class="numeric">{{ member.joinedAt }}</dd>
      <dt>最后登录</dt>
      <dd class="numeric">{{ member.lastLogin || '尚未登录' }}</dd>
      <dt>备注</dt>
      <dd>{{ member.note || '暂无备注' }}</dd>
    </dl>
  </section>
  <section class="detail-section">
    <h3>组织信息</h3>
    <dl class="member-facts">
      <dt>所属部门</dt>
      <dd class="fact-with-icon">
        <span class="fact-icon"><AppIcon name="organization" :size="16" /></span
        >{{ store.departmentName(member.departmentId) }}
      </dd>
      <dt>部门负责人</dt>
      <dd class="fact-with-icon">
        <AvatarMark v-if="leader" :name="leader.name" :size="24" tone="solid" />{{ leader?.name || '未设置' }}
      </dd>
      <dt>岗位</dt>
      <dd>{{ member.position }}</dd>
    </dl>
  </section>
  <section class="detail-section">
    <h3>角色权限</h3>
    <div
      v-for="role in store.roles.filter((r) => member.roleIds.includes(r.id))"
      :key="role.id"
      class="role-summary"
    >
      <MemberRoleTags :ids="[role.id]" />
      <p>{{ role.description }}</p>
    </div>
  </section>
  <section class="detail-section">
    <h3>数据权限</h3>
    <span class="scope-label">{{ scopeLabels[member.scope] }}</span>
    <p class="detail-hint">{{ scopeDescription }}</p>
  </section>
  <section class="detail-section activity-section">
    <div class="row-between">
      <h3>最近活动</h3>
      <button class="btn-link" @click="emit('logs')">查看更多<AppIcon name="right" :size="12" /></button>
    </div>
    <ol class="member-timeline">
      <li v-for="log in recent" :key="log.id">
        <span>{{ log.time }}</span
        ><strong>{{ log.action }}</strong
        ><small>{{ log.actor }} · {{ log.reason || '界面预览操作' }}</small>
      </li>
      <li>
        <span>{{ member.joinedAt }}</span
        ><strong>加入当前企业</strong><small>成员关系记录 · 示例数据</small>
      </li>
    </ol>
  </section>
</template>
<style scoped>
.detail-section {
  padding: 18px 0;
  border-bottom: 1px solid var(--color-border);
}
.detail-section:first-child {
  padding-top: 16px;
}
.detail-section h3 {
  font-size: 13px;
  margin-bottom: 14px;
}
.member-facts {
  margin: 0;
  display: grid;
  grid-template-columns: 78px minmax(0, 1fr);
  gap: 10px 10px;
  font-size: 12px;
  align-items: center;
}
.member-facts dt {
  color: var(--color-text-muted);
}
.member-facts dd {
  margin: 0;
  overflow-wrap: anywhere;
  min-width: 0;
}
.member-facts :deep(.status-badge) {
  font-size: 11px;
}
.fact-with-icon {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fact-icon {
  display: grid;
  place-items: center;
  width: 25px;
  height: 25px;
  color: var(--color-primary);
  background: var(--color-primary-soft);
  border-radius: 5px;
}
.role-summary {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}
.role-summary + .role-summary {
  margin-top: 10px;
}
.role-summary p,
.detail-hint {
  font-size: 11px;
  color: var(--color-text-muted);
  line-height: 1.8;
}
.role-summary :deep(.member-role-tags) {
  flex-shrink: 0;
}
.scope-label {
  display: inline-flex;
  color: var(--color-success);
  background: var(--color-success-soft);
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
}
.detail-hint {
  margin-top: 8px;
}
.activity-section {
  border-bottom: 0;
}
.activity-section .row-between {
  margin-bottom: 14px;
}
.activity-section h3 {
  margin: 0;
}
.activity-section .btn-link {
  font-size: 11px;
}
.member-timeline {
  list-style: none;
  margin: 0 0 0 3px;
  padding: 0 0 0 17px;
  border-left: 1px solid var(--color-border);
}
.member-timeline li {
  position: relative;
  padding-bottom: 16px;
  display: flex;
  gap: 3px 10px;
  flex-wrap: wrap;
  font-size: 11px;
}
.member-timeline li::before {
  content: '';
  position: absolute;
  left: -21px;
  top: 6px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-surface);
}
.member-timeline span,
.member-timeline small {
  color: var(--color-text-muted);
  font-size: 10px;
}
.member-timeline strong {
  font-weight: 500;
}
.member-timeline small {
  width: 100%;
}
</style>
