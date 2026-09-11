<script setup lang="ts">
import { computed, ref, watch } from "vue";
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import { useToast } from "@/shared/ui/useToast";
import { useMemberStore } from "../model/memberStore";
import { type Member } from "../model/types";
export type OperationKind = "role" | "reset" | "suspend" | "enable";
import { useRoleStore } from "@/features/enterprise/model/roleStore";
const catalogRoles = useRoleStore();
const roleNames = computed(() =>
  catalogRoles.roles.filter((r) => r.enabled).map((r) => r.name),
);
const props = defineProps<{
  open: boolean;
  member?: Member;
  kind: OperationKind;
}>();
const emit = defineEmits<{ close: [] }>();
const store = useMemberStore();
const toast = useToast();
const role = ref("成员");
const scope = ref("所属部门数据");
const reason = ref("");
const confirm = ref(false);
const error = ref("");
let version = 0;
const title = computed(
  () =>
    ({
      role: "角色变更与权限调整",
      reset: "重置成员密码",
      suspend: "禁用成员账号",
      enable: "重新启用成员",
    })[props.kind],
);
watch(
  () => props.open,
  (open) => {
    if (open) {
      role.value = props.member?.role ?? "成员";
      scope.value = props.member?.dataScope ?? "所属部门数据";
      reason.value = "";
      confirm.value = false;
      error.value = "";
      version = props.member?.version ?? 0;
    }
  },
);
function submit() {
  if (!props.member) return;
  try {
    if (props.kind === "role")
      store.changeRole(props.member.id, role.value, scope.value, version);
    else if (props.kind === "reset") store.resetPassword(props.member.id, version);
    else {
      if (!confirm.value) throw new Error("请先确认影响范围。");
      store.setStatus(
        props.member.id,
        props.kind === "suspend" ? "suspended" : "active",
        reason.value,
        version,
      );
    }
    toast.show(
      props.kind === "reset"
        ? "已生成演示重置回执；未发送邮件、未修改真实密码。"
        : "演示变更已生效，操作记录已写入本地日志。",
    );
    emit("close");
  } catch (e) {
    error.value = e instanceof Error ? e.message : "操作失败";
  }
}
</script>
<template>
  <BaseDialog :open="open" :title="title" :wide="kind === 'role'" @close="emit('close')"
    ><template v-if="member"
      ><div class="person-strip">
        <span class="person-avatar">{{ member.name.slice(0, 1) }}</span>
        <div>
          <strong>{{ member.name }}</strong>
          <p>{{ member.email }} <span>·</span> {{ member.department }}</p>
        </div>
        <StatusBadge tone="blue">{{ member.role }}</StatusBadge>
      </div>
      <div v-if="kind === 'role'">
        <div class="compare-roles">
          <div>
            <small>当前角色</small>
            <h3>{{ member.role }}</h3>
            <p>{{ member.dataScope }}</p>
          </div>
          <AppIcon name="ArrowRight" :size="23" />
          <div class="target-role">
            <small>目标角色</small>
            <h3>{{ role }}</h3>
            <p>{{ scope }}</p>
          </div>
        </div>
        <div class="form-grid">
          <label class="field"
            ><span>选择新角色</span
            ><select v-model="role">
              <option v-for="r in roleNames" :key="r">{{ r }}</option>
            </select></label
          ><label class="field"
            ><span>数据访问范围</span
            ><select v-model="scope">
              <option>所属部门数据</option>
              <option>本人负责的数据</option>
              <option>全部数据</option>
            </select></label
          >
        </div>
        <div class="info-banner">
          变更权限与数据范围是独立授权动作。正式环境需校验可授予权限、所有者保护与即时会话失效。
        </div>
      </div>
      <div v-else-if="kind === 'reset'">
        <div class="security-callout">
          <span class="metric-icon tone-blue"
            ><AppIcon name="KeyRound" :size="28"
          /></span>
          <div>
            <h3>向成员已验证邮箱发送重置链接</h3>
            <p>成员自行设置新密码，管理员无法查看原密码或新密码。</p>
          </div>
        </div>
        <div class="checklist">
          <p><AppIcon name="CheckCircle2" />重置链接应单次使用并具有有效期</p>
          <p><AppIcon name="CheckCircle2" />重置完成后撤销既有会话</p>
          <p><AppIcon name="CheckCircle2" />操作日志不记录密码、验证码或重置令牌</p>
        </div>
        <div class="info-banner">本轮只演示重置流程和操作回执，不发送真实重置链接。</div>
      </div>
      <div v-else>
        <div :class="['info-banner', kind === 'suspend' ? 'info-banner--danger' : '']">
          {{
            kind === "suspend"
              ? "禁用后应阻止新的访问并撤销现有会话；成员资料和历史记录保留。"
              : "重新启用前需复核现有角色与数据范围；不会恢复历史登录会话。"
          }}
        </div>
        <label class="field"
          ><span>操作原因 <em>*</em></span
          ><textarea
            v-model="reason"
            maxlength="200"
            placeholder="说明本次操作的业务原因"
          /></label
        ><label class="checkbox-line"
          ><input
            v-model="confirm"
            type="checkbox"
          />我已核对成员身份、角色与数据访问范围</label
        >
      </div>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p></template
    ><template #footer
      ><BaseButton @click="emit('close')">取消</BaseButton
      ><BaseButton
        :variant="kind === 'suspend' ? 'danger' : 'primary'"
        :icon="kind === 'reset' ? 'Send' : kind === 'suspend' ? 'Power' : 'Check'"
        @click="submit"
        >{{
          kind === "role"
            ? "确认角色变更"
            : kind === "reset"
              ? "生成演示重置回执"
              : kind === "suspend"
                ? "确认禁用"
                : "确认重新启用"
        }}</BaseButton
      ></template
    ></BaseDialog
  >
</template>
