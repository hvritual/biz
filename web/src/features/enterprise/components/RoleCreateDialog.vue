<script setup lang="ts">
import { ref, watch } from "vue";
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import { useRoleStore } from "../model/roleStore";
import { useToast } from "@/shared/ui/useToast";
const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();
const name = ref("");
const description = ref("");
const error = ref("");
const store = useRoleStore();
const toast = useToast();
watch(
  () => props.open,
  () => {
    name.value = "";
    description.value = "";
    error.value = "";
  },
);
function submit() {
  try {
    store.add(name.value, description.value);
    toast.show("自定义角色已创建（本地演示）。");
    emit("close");
  } catch (e) {
    error.value = (e as Error).message;
  }
}
</script>
<template>
  <BaseDialog
    :open="open"
    title="新建角色"
    description="按职责创建角色，默认仅拥有工作台查看权限"
    @close="emit('close')"
    ><form id="role-create-form" @submit.prevent="submit">
      <label class="field"
        ><span>角色名称 <em>*</em></span
        ><input
          v-model="name"
          required
          maxlength="30"
          placeholder="例如：区域运维主管" /></label
      ><label class="field separated"
        ><span>角色描述</span
        ><textarea
          v-model="description"
          maxlength="200"
          placeholder="说明角色的业务职责与适用对象"
        />
      </label>
      <div class="info-banner">
        创建后在右侧权限矩阵中配置可查看、可操作和可导出的模块。
      </div>
      <p v-if="error" role="alert" class="form-error">{{ error }}</p>
    </form>
    <template #footer
      ><BaseButton @click="emit('close')">取消</BaseButton
      ><BaseButton variant="primary" type="submit" form="role-create-form"
        >创建角色</BaseButton
      ></template
    ></BaseDialog
  >
</template>
