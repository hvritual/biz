<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Member, MemberAction, DataScope } from '@/types/enterprise'
import { scopeLabels } from '@/types/enterprise'
import { memberActionError } from '@/services/memberPolicy'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import UiDialog from '@/components/ui/UiDialog.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
const props = defineProps<{ open: boolean; action: MemberAction; member: Member | null }>()
const emit = defineEmits<{ close: [] }>(),
  store = useEnterpriseStore(),
  ui = useUiStore()
const titles: Record<MemberAction, string> = {
  create: '新增成员',
  invite: '邀请成员',
  edit: '修改成员信息',
  role: '角色变更与权限调整',
  reset: '密码重置',
  suspend: '禁用成员',
  activate: '重新启用成员',
  remove: '移除成员与交接',
}
const draft = ref<Member | null>(null),
  reason = ref(''),
  error = ref(''),
  confirmed = ref(false),
  busy = ref(false)
const structural = computed(() => ['create', 'invite', 'edit'].includes(props.action))
const danger = computed(() => ['suspend', 'remove'].includes(props.action))
const blocking = computed(() =>
  props.member
    ? memberActionError(props.action, props.member, store.members, store.roles, draft.value?.roleIds)
    : null,
)
watch(
  () => [props.open, props.action, props.member] as const,
  () => {
    error.value = ''
    reason.value = ''
    confirmed.value = false
    draft.value = props.member
      ? (JSON.parse(JSON.stringify(props.member)) as Member)
      : {
          id: crypto.randomUUID(),
          name: '',
          email: '',
          phone: '',
          employeeId: '',
          departmentId: 'operations',
          position: '业务专员',
          roleIds: ['role-2'],
          scope: 'department',
          status: props.action === 'invite' ? 'invited' : 'active',
          online: false,
          joinedAt: new Date().toISOString().slice(0, 10),
          lastLogin: null,
          version: 0,
          mfa: false,
          note: '',
        }
  },
  { immediate: true },
)
function toggleRole(id: string) {
  if (!draft.value) return
  const ids = draft.value.roleIds
  draft.value.roleIds = ids.includes(id) ? ids.filter((i) => i !== id) : [...ids, id]
}
async function submit() {
  if (!draft.value) return
  error.value = ''
  if (blocking.value) {
    error.value = blocking.value
    return
  }
  if ((danger.value || props.action === 'reset') && !confirmed.value) {
    error.value = '请先确认操作影响。'
    return
  }
  busy.value = true
  try {
    if (structural.value || props.action === 'role')
      store.saveMember(draft.value, props.action, props.member?.version ?? 0)
    else if (['suspend', 'activate', 'remove'].includes(props.action))
      store.changeStatus(
        draft.value.id,
        props.action as 'suspend' | 'activate' | 'remove',
        props.member!.version,
        reason.value,
      )
    else if (props.action === 'reset') {
      store.audit(
        '成员管理',
        '创建密码重置请求（预览）',
        draft.value.name,
        '未请求',
        '待接入身份服务',
        reason.value,
        'high',
      )
    }
    const message =
      props.action === 'reset'
        ? '已记录预览重置请求；未发送邮件，也未改变真实密码。'
        : props.action === 'invite'
          ? '已创建预览邀请记录；未实际发送邀请邮件。'
          : '变更已保存到当前企业的本地预览数据。'
    ui.toast(message, props.action === 'reset' || props.action === 'invite' ? 'info' : 'success')
    emit('close')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '操作失败，请重试。'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <UiDialog
    :open="open"
    :title="titles[action]"
    :width="structural || action === 'role' ? '820px' : '560px'"
    @close="emit('close')"
    ><div v-if="draft" class="page-stack">
      <div v-if="member" class="member-summary">
        <AvatarMark :name="member.name" :size="44" />
        <div>
          <h3>{{ member.name }}</h3>
          <p>{{ store.departmentName(member.departmentId) }} · {{ member.email }}</p>
        </div>
      </div>
      <div v-if="blocking" class="notice-box danger"><AppIcon name="shield" />{{ blocking }}</div>
      <form v-if="structural" id="member-form" @submit.prevent="submit">
        <section class="form-section">
          <h3>基本信息</h3>
          <div class="form-grid">
            <label class="field"
              ><span class="required">姓名</span
              ><input
                v-model="draft.name"
                class="input"
                maxlength="40"
                required
                placeholder="请输入成员姓名" /></label
            ><label class="field"
              ><span class="required">邮箱</span
              ><input
                v-model="draft.email"
                class="input"
                type="email"
                required
                placeholder="name@example.com"
              /><small v-if="action === 'edit'">预览资料修改不等于变更已验证的登录凭据。</small></label
            ><label class="field"
              ><span>手机号</span
              ><input v-model="draft.phone" class="input" maxlength="20" placeholder="选填" /></label
            ><label class="field"
              ><span>员工编号</span
              ><input v-model="draft.employeeId" class="input" maxlength="40" placeholder="企业内部编号"
            /></label>
          </div>
        </section>
        <section class="form-section">
          <h3>组织与岗位</h3>
          <div class="form-grid">
            <label class="field"
              ><span class="required">所属部门</span
              ><select v-model="draft.departmentId" class="select">
                <option v-for="d in store.departments" :key="d.id" :value="d.id">{{ d.name }}</option>
              </select></label
            ><label class="field"
              ><span>岗位</span><input v-model="draft.position" class="input" maxlength="40" /></label
            ><label class="field"
              ><span>加入日期</span><input v-model="draft.joinedAt" class="input" type="date" /></label
            ><label class="field"
              ><span>数据范围</span
              ><select v-model="draft.scope" class="select">
                <option v-for="(label, key) in scopeLabels" :key="key" :value="key">{{ label }}</option>
              </select></label
            >
          </div>
        </section>
        <section v-if="action !== 'edit'" class="form-section">
          <h3>角色与权限</h3>
          <div class="role-options">
            <label
              v-for="r in store.roles.filter((r) => r.enabled)"
              :key="r.id"
              :class="['role-option', { chosen: draft.roleIds.includes(r.id) }]"
              ><input
                type="checkbox"
                :checked="draft.roleIds.includes(r.id)"
                @change="toggleRole(r.id)"
              /><span>{{ r.name }}</span></label
            >
          </div>
        </section>
        <section class="form-section">
          <label class="field"
            ><span>备注</span
            ><textarea
              v-model="draft.note"
              class="textarea"
              rows="2"
              maxlength="500"
              placeholder="补充成员职责或说明"
            />
          </label>
        </section>
      </form>
      <template v-else-if="action === 'role'"
        ><div class="notice-box">
          <AppIcon
            name="shield"
          />角色权限与数据范围分别配置。变更后按最新授权重新计算访问范围，不要求成员重新登录来激活权限。
        </div>
        <h3>选择目标角色</h3>
        <div class="role-options">
          <label
            v-for="r in store.roles.filter((r) => r.enabled)"
            :key="r.id"
            :class="['role-option', { chosen: draft.roleIds.includes(r.id) }]"
            ><input
              type="checkbox"
              :checked="draft.roleIds.includes(r.id)"
              @change="toggleRole(r.id)"
            /><span>{{ r.name }}</span></label
          >
        </div>
        <label class="field"
          ><span>目标数据范围</span
          ><select v-model="draft.scope" class="select">
            <option v-for="(label, key) in scopeLabels" :key="key" :value="key">{{ label }}</option>
          </select></label
        >
        <div class="change-preview">
          <div>
            <small>变更前</small><strong>{{ member?.roleIds.map(store.roleName).join('、') }}</strong>
            <p>{{ scopeLabels[member!.scope] }}</p>
          </div>
          <AppIcon name="arrow" />
          <div>
            <small>变更后</small
            ><strong>{{ draft.roleIds.map(store.roleName).join('、') || '尚未选择' }}</strong>
            <p>{{ scopeLabels[draft.scope as DataScope] }}</p>
          </div>
        </div></template
      ><template v-else-if="action === 'reset'"
        ><div class="notice-box warning">
          <AppIcon name="key" /><span
            >通过身份服务向成员的已验证邮箱发送一次性重置链接。管理员不查看或保存明文密码。本预览仅记录请求，不执行真实重置。</span
          >
        </div>
        <label class="field"><span>接收邮箱</span><input class="input" :value="draft.email" readonly /></label
        ><label class="option-line"
          ><input v-model="confirmed" type="checkbox" />我已确认成员身份，并了解此处不会发送真实邮件</label
        >
        <p class="muted">
          正式接入要求：链接单次使用、短时有效；重置成功后撤销旧会话。SSO 账号应跳转身份提供方处理。
        </p></template
      ><template v-else
        ><div :class="['notice-box', danger ? 'danger' : 'warning']">
          <AppIcon :name="danger ? 'lock' : 'shield'" /><span>{{
            action === 'suspend'
              ? '禁用将立即阻止该成员访问当前企业并撤销本租户会话。历史数据与业务责任关系保留，不自动转移。'
              : action === 'remove'
                ? '移除结束当前企业的成员关系，不删除全局账号或企业历史数据。须先完成客户、点位与工单责任交接。'
                : '启用前重新核验成员身份、现有角色和数据范围。已移除成员不能通过启用恢复，须重新邀请。'
          }}</span>
        </div>
        <label v-if="danger" class="option-line"
          ><input v-model="confirmed" type="checkbox" />{{
            action === 'remove'
              ? '已完成业务交接，并确认保留历史审计记录'
              : '确认禁用当前企业访问，不删除历史数据'
          }}</label
        ><label class="field"
          ><span class="required">操作原因</span
          ><textarea
            v-model="reason"
            class="textarea"
            maxlength="300"
            placeholder="请记录本次操作的原因，用于审计追踪"
          />
        </label>
        <p class="muted">
          当前角色：{{ draft.roleIds.map(store.roleName).join('、') }} · 数据范围：{{
            scopeLabels[draft.scope]
          }}
        </p></template
      >
      <div v-if="action === 'invite'" class="notice-box">
        <AppIcon name="mail" />邀请记录将在本地预览中创建。邮件投递、有效期和激活凭证需由服务端提供。
      </div>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </div>
    <template #footer
      ><button class="btn" @click="emit('close')">取消</button
      ><button
        :class="['btn', danger ? 'btn-danger' : 'btn-primary']"
        :disabled="busy || Boolean(blocking)"
        @click="submit"
      >
        {{
          busy
            ? '正在处理…'
            : action === 'reset'
              ? '记录重置请求'
              : action === 'invite'
                ? '创建邀请'
                : action === 'suspend'
                  ? '确认禁用'
                  : action === 'activate'
                    ? '确认启用'
                    : action === 'remove'
                      ? '确认移除'
                      : '保存变更'
        }}
      </button></template
    ></UiDialog
  >
</template>
<style scoped>
.member-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 16px;
  border-radius: 10px;
  background: var(--color-surface-soft);
}
.member-summary p {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 3px;
}
.role-options {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}
.role-option {
  display: flex;
  gap: 9px;
  align-items: center;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 12px;
  font-size: 13px;
}
.role-option.chosen {
  background: var(--color-primary-soft);
  border-color: var(--color-primary);
  color: var(--color-primary);
}
.change-preview {
  display: grid;
  grid-template-columns: 1fr 24px 1fr;
  gap: 18px;
  align-items: center;
  padding: 18px;
  background: var(--color-surface-soft);
  border-radius: 10px;
}
.change-preview small {
  display: block;
  color: var(--color-text-muted);
  margin-bottom: 6px;
}
.change-preview strong {
  display: block;
  font-size: 14px;
}
.change-preview p {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-top: 6px;
}
@media (max-width: 767px) {
  .role-options {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .role-option {
    padding: 10px;
    font-size: 12px;
  }
}
</style>
