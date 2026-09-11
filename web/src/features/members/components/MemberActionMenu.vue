<script setup lang="ts">
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import type { Member } from "../model/types";
import type { OperationKind } from "./MemberOperationDialog.vue";
defineProps<{ member?: Member }>();
const emit = defineEmits<{
  close: [];
  operation: [kind: OperationKind];
  detail: [];
}>();
</script>
<template>
  <BaseDialog
    :open="!!member"
    :title="`${member?.name ?? ''} · 成员操作`"
    @close="emit('close')"
    ><div class="action-list">
      <button @click="emit('detail')"><AppIcon name="UserRound" />查看成员详情</button
      ><button @click="emit('operation', 'role')">
        <AppIcon name="UserCog" />调整角色与数据权限</button
      ><button
        :disabled="member?.status === 'invited'"
        @click="emit('operation', 'reset')"
      >
        <AppIcon name="KeyRound" />重置密码</button
      ><button
        v-if="member?.status === 'active'"
        class="danger-text"
        @click="emit('operation', 'suspend')"
      >
        <AppIcon name="Power" />禁用成员账号</button
      ><button
        v-else-if="member?.status === 'suspended'"
        @click="emit('operation', 'enable')"
      >
        <AppIcon name="Power" />重新启用账号
      </button>
    </div></BaseDialog
  >
</template>
