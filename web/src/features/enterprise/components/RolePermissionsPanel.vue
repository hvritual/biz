<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import { useRoleStore, permissionModules, type Role } from "../model/roleStore";
import { useToast } from "@/shared/ui/useToast";
const props = defineProps<{ role: Role }>();
const store = useRoleStore();
const toast = useToast();
const scope = ref("");
const permissions = reactive<Record<string, boolean[]>>({});
const error = ref("");
watch(
  () => props.role,
  (role) => {
    scope.value = role.scope;
    Object.keys(permissions).forEach((k) => delete permissions[k]);
    Object.entries(role.permissions).forEach(([k, v]) => (permissions[k] = [...v]));
    error.value = "";
  },
  { immediate: true },
);
function save() {
  try {
    store.save(props.role.id, JSON.parse(JSON.stringify(permissions)), scope.value);
    toast.show("角色权限已保存（本地演示）。");
  } catch (e) {
    error.value = (e as Error).message;
  }
}
</script>
<template>
  <aside class="panel">
    <div class="role-overview">
      <span class="metric-icon tone-blue"
        ><AppIcon :name="role.builtin ? 'Crown' : 'ShieldCheck'" :size="27"
      /></span>
      <div>
        <h2>{{ role.name }}</h2>
        <StatusBadge :tone="role.builtin ? 'blue' : 'green'">{{
          role.builtin ? "内置角色" : "自定义角色"
        }}</StatusBadge>
        <p>{{ role.description }}</p>
      </div>
    </div>
    <div class="tabs"><button class="active">权限配置</button></div>
    <div class="panel-content">
      <h3 class="section-title">功能权限</h3>
      <table class="permission-table">
        <thead>
          <tr>
            <th>功能模块</th>
            <th>查看</th>
            <th>操作</th>
            <th>导出</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="module in permissionModules" :key="module">
            <td>{{ module }}</td>
            <td v-for="(action, i) in ['查看', '操作', '导出']" :key="action">
              <input
                v-if="permissions[module]"
                v-model="permissions[module]![i]"
                type="checkbox"
                :disabled="role.builtin"
                :aria-label="`${module}${action}权限`"
              />
            </td>
          </tr>
        </tbody>
      </table>
      <label class="field separated"
        ><span>默认数据范围</span
        ><select v-model="scope" :disabled="role.builtin">
          <option>全部数据</option>
          <option>所属部门数据</option>
          <option>本人负责的数据</option>
        </select></label
      >
      <p class="permission-note">
        {{
          role.builtin
            ? "内置角色为只读。需要调整权限时，请创建自定义角色。"
            : "权限预览不替代服务端授权；上线前需对接权限校验、审计及权限即时失效。"
        }}
      </p>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </div>
    <footer class="permission-footer">
      <small>当前企业 · 本地演示配置</small
      ><BaseButton variant="primary" :disabled="role.builtin" @click="save"
        >保存权限</BaseButton
      >
    </footer>
  </aside>
</template>
