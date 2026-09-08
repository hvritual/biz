<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import { useToast } from "@/shared/ui/useToast";
import { useMemberStore } from "../model/memberStore";
import { type Member, type MemberInput } from "../model/types";
import { useRoleStore } from "@/features/enterprise/model/roleStore";
import { useOrganizationStore } from "@/features/enterprise/model/organization";
const catalogRoles = useRoleStore();
const catalogOrganization = useOrganizationStore();
const roleNames = computed(() =>
  catalogRoles.roles.filter((r) => r.enabled).map((r) => r.name),
);
const departments = computed(() => catalogOrganization.items.map((d) => d.name));
const props = defineProps<{
  open: boolean;
  member?: Member;
  invite?: boolean;
}>();
const emit = defineEmits<{ close: [] }>();
const store = useMemberStore();
const toast = useToast();
const error = ref("");
let version = 0;
const form = reactive<MemberInput>({
  name: "",
  email: "",
  phone: "",
  department: departments.value[0]!,
  role: "成员",
});
watch(
  () => props.open,
  (open) => {
    if (open) {
      error.value = "";
      version = props.member?.version ?? 0;
      Object.assign(
        form,
        props.member
          ? {
              name: props.member.name,
              email: props.member.email,
              phone: props.member.phone,
              department: props.member.department,
              role: props.member.role,
            }
          : {
              name: "",
              email: "",
              phone: "",
              department: departments.value[0],
              role: "成员",
            },
      );
    }
  },
);
function save() {
  try {
    if (props.member) store.edit(props.member.id, { ...form }, version);
    else store.add({ ...form }, props.invite);
    toast.show(
      props.invite
        ? "已创建演示邀请记录，未发送真实邮件。"
        : "成员信息已更新（仅本地演示数据）。",
    );
    emit("close");
  } catch (e) {
    error.value = e instanceof Error ? e.message : "保存失败";
  }
}
</script>
<template>
  <BaseDialog
    :open="open"
    :title="member ? '修改成员信息' : invite ? '邀请成员' : '新增成员'"
    description="管理成员身份、组织关系与访问角色"
    wide
    @close="emit('close')"
    ><form id="member-editor-form" class="form-grid" @submit.prevent="save">
      <div class="form-section-title">基本信息</div>
      <label class="field"
        ><span>成员姓名 <em>*</em></span
        ><input
          v-model="form.name"
          required
          maxlength="30"
          placeholder="请输入真实姓名" /></label
      ><label class="field"
        ><span>邮箱 <em>*</em></span
        ><input
          v-model="form.email"
          type="email"
          required
          placeholder="name@example.com"
        /><small>邀请与账号通知使用此邮箱</small></label
      ><label class="field"
        ><span>手机号</span
        ><input v-model="form.phone" placeholder="请输入手机号（选填）" /></label
      ><label class="field"
        ><span>所属部门 <em>*</em></span
        ><select v-model="form.department">
          <option v-for="d in departments" :key="d">{{ d }}</option>
        </select></label
      >
      <div class="form-section-title">角色与数据范围</div>
      <label class="field"
        ><span>成员角色 <em>*</em></span
        ><select v-model="form.role" :disabled="!!member">
          <option v-for="r in roleNames" :key="r">{{ r }}</option></select
        ><small v-if="member">角色变更请使用独立的“调整角色”操作</small></label
      >
      <div class="field">
        <span>默认数据范围</span>
        <div class="read-only-value">所属部门数据</div>
        <small>高级授权可在角色变更中调整</small>
      </div>
      <div class="info-banner full-width">
        界面演示：不创建真实账号，不发送短信或邮件。正式授权需要接入服务端权限校验。
      </div>
      <p v-if="error" class="form-error full-width" role="alert">{{ error }}</p>
    </form>
    <template #footer
      ><BaseButton @click="emit('close')">取消</BaseButton
      ><BaseButton
        type="submit"
        form="member-editor-form"
        variant="primary"
        :icon="invite ? 'Send' : 'Check'"
        >{{ member ? "保存修改" : invite ? "创建演示邀请" : "新增成员" }}</BaseButton
      ></template
    ></BaseDialog
  >
</template>
