<script setup lang="ts">
import { ref } from 'vue'
import type { Member, MemberAction } from '@/types/enterprise'
import { scopeLabels, statusLabels } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import UiDialog from '@/components/ui/UiDialog.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
const props = defineProps<{ member: Member | null }>()
const emit = defineEmits<{ close: []; action: [action: MemberAction, member: Member] }>(),
  store = useEnterpriseStore(),
  tab = ref('基本信息')
</script>
<template>
  <UiDialog :open="Boolean(member)" title="成员详情" width="590px" drawer @close="emit('close')"
    ><div v-if="props.member" class="page-stack">
      <div class="detail-profile">
        <AvatarMark :name="props.member.name" :size="66" tone="solid" />
        <div class="flex-1">
          <h2>{{ props.member.name }}</h2>
          <p class="muted">
            {{ props.member.position }} · {{ store.departmentName(props.member.departmentId) }}
          </p>
        </div>
        <StatusBadge
          :text="statusLabels[props.member.status]"
          :tone="props.member.status === 'active' ? 'success' : 'neutral'"
        />
      </div>
      <div class="row wrap">
        <button class="btn" @click="emit('action', 'edit', props.member)">
          <AppIcon name="edit" :size="15" />编辑资料</button
        ><button class="btn" @click="emit('action', 'role', props.member)">
          <AppIcon name="shield" :size="15" />调整角色</button
        ><button class="btn" @click="emit('action', 'reset', props.member)">
          <AppIcon name="key" :size="15" />重置密码
        </button>
      </div>
      <div class="tabs">
        <button
          v-for="t in ['基本信息', '角色权限', '登录安全', '生命周期记录']"
          :key="t"
          :class="['tab', { active: tab === t }]"
          @click="tab = t"
        >
          {{ t }}
        </button>
      </div>
      <template v-if="tab === '基本信息'"
        ><h3>个人信息</h3>
        <dl class="detail-list">
          <dt>姓名</dt>
          <dd>{{ props.member.name }}</dd>
          <dt>邮箱</dt>
          <dd>{{ props.member.email }}</dd>
          <dt>手机号码</dt>
          <dd>{{ props.member.phone || '未填写' }}</dd>
          <dt>员工编号</dt>
          <dd>{{ props.member.employeeId || '未填写' }}</dd>
          <dt>所属部门</dt>
          <dd>{{ store.departmentName(props.member.departmentId) }}</dd>
          <dt>岗位</dt>
          <dd>{{ props.member.position }}</dd>
          <dt>加入时间</dt>
          <dd>{{ props.member.joinedAt }}</dd>
          <dt>最后登录</dt>
          <dd>{{ props.member.lastLogin || '尚未登录' }}</dd>
          <dt>数据范围</dt>
          <dd>{{ scopeLabels[props.member.scope] }}</dd>
          <dt>备注</dt>
          <dd>{{ props.member.note || '暂无备注' }}</dd>
        </dl></template
      ><template v-else-if="tab === '角色权限'"
        ><h3>当前分配角色</h3>
        <div
          v-for="r in store.roles.filter((r) => props.member!.roleIds.includes(r.id))"
          :key="r.id"
          class="card panel-pad"
        >
          <div class="row-between">
            <h3>{{ r.name }}</h3>
            <span class="pill">{{ r.builtin ? '内置角色' : '自定义角色' }}</span>
          </div>
          <p class="secondary role-description">{{ r.description }}</p>
          <div class="chip-list">
            <span v-for="permission in r.permissions" :key="permission" class="pill">{{ permission }}</span>
          </div>
        </div></template
      ><template v-else-if="tab === '登录安全'"
        ><div class="notice-box">
          <AppIcon name="shield" />安全状态来自预览数据，不代表已完成真实安全检测。
        </div>
        <dl class="detail-list">
          <dt>多因素认证</dt>
          <dd>{{ props.member.mfa ? '已开启（预览）' : '未开启' }}</dd>
          <dt>账号状态</dt>
          <dd>{{ statusLabels[props.member.status] }}</dd>
          <dt>最后登录</dt>
          <dd>{{ props.member.lastLogin || '尚未登录' }}</dd>
          <dt>密码</dt>
          <dd>管理员不可读取明文密码</dd>
        </dl>
        <button class="btn" @click="emit('action', 'reset', props.member)">发起密码重置</button></template
      ><template v-else
        ><div class="timeline">
          <div
            v-for="log in store.logs.filter((l) => l.target === props.member!.name)"
            :key="log.id"
            class="timeline-item"
          >
            <strong>{{ log.action }}</strong>
            <p class="muted">{{ log.after }}</p>
            <small>{{ log.time }} · {{ log.actor }}</small>
          </div>
          <div class="timeline-item">
            <strong>加入当前企业</strong><small>{{ props.member.joinedAt }}</small>
          </div>
        </div></template
      >
    </div></UiDialog
  >
</template>
<style scoped>
.detail-profile {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 6px;
}
.detail-profile h2 {
  font-size: 23px;
}
.detail-profile p {
  font-size: 12px;
  margin-top: 6px;
}
.role-description {
  font-size: 13px;
  margin: 10px 0 16px;
}
</style>
