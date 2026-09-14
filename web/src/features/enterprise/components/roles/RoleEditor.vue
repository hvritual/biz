<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Role, DataScope } from '@/types/enterprise'
import { scopeLabels } from '@/types/enterprise'
import { permissionCatalog } from '@/services/demo/seed'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import UiDialog from '@/components/ui/UiDialog.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
const props = defineProps<{ open: boolean; role: Role | null }>(),
  emit = defineEmits<{ close: [] }>()
const store = useEnterpriseStore(),
  ui = useUiStore(),
  error = ref('')
const draft = ref<Role>({
  id: '',
  name: '',
  description: '',
  builtin: false,
  enabled: true,
  scope: 'department',
  permissions: [],
  updatedAt: '',
})
const readonly = computed(() => Boolean(props.role?.builtin))
const actionNames: Record<string, string> = { read: '查看', manage: '管理', assign: '分配', export: '导出' }
watch(
  () => [props.open, props.role],
  () => {
    error.value = ''
    draft.value = props.role
      ? (JSON.parse(JSON.stringify(props.role)) as Role)
      : {
          id: crypto.randomUUID(),
          name: '',
          description: '',
          builtin: false,
          enabled: true,
          scope: 'department',
          permissions: [],
          updatedAt: '',
        }
  },
  { immediate: true },
)
function toggle(value: string) {
  if (readonly.value) return
  draft.value.permissions = draft.value.permissions.includes(value)
    ? draft.value.permissions.filter((p) => p !== value)
    : [...draft.value.permissions, value]
}
function save() {
  try {
    store.saveRole(draft.value)
    ui.toast('角色配置已保存到本地预览，操作已记录。')
    emit('close')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败。'
  }
}
</script>
<template>
  <UiDialog
    :open="open"
    :title="readonly ? '内置角色详情' : role ? '编辑角色权限' : '新建角色'"
    width="850px"
    @close="emit('close')"
    ><div class="page-stack">
      <div v-if="readonly" class="notice-box">
        <AppIcon name="shield" />内置角色只读。企业所有者拥有受保护的管理能力，不能通过此页面修改或禁用。
      </div>
      <div class="form-grid">
        <label class="field"
          ><span class="required">角色名称</span
          ><input
            v-model="draft.name"
            class="input"
            maxlength="40"
            :readonly="readonly"
            placeholder="例如：华东区域运营" /></label
        ><label class="field"
          ><span>数据范围</span
          ><select v-model="draft.scope" class="select" :disabled="readonly">
            <option v-for="(label, key) in scopeLabels" :key="key" :value="key as DataScope">
              {{ label }}
            </option>
          </select></label
        ><label class="field full-width"
          ><span>角色说明</span
          ><textarea
            v-model="draft.description"
            class="textarea"
            rows="2"
            :readonly="readonly"
            maxlength="300"
            placeholder="描述该角色的职责和授权边界"
          />
        </label>
      </div>
      <div class="row-between">
        <h3>功能权限</h3>
        <span class="muted">已选 {{ draft.permissions.length }} 项</span>
      </div>
      <div class="table-scroll">
        <table class="data-table permissions-table">
          <thead>
            <tr>
              <th>功能模块</th>
              <th v-for="label in actionNames" :key="label">{{ label }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="module in permissionCatalog" :key="module.id">
              <td>{{ module.name }}</td>
              <td v-for="(label, a) in actionNames" :key="a">
                <input
                  v-if="module.actions.includes(a)"
                  type="checkbox"
                  :aria-label="module.name + ' ' + label"
                  :checked="draft.permissions.includes(module.id + '.' + a)"
                  :disabled="readonly"
                  @change="toggle(module.id + '.' + a)"
                /><span v-else class="muted">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="notice-box">
        <AppIcon
          name="help"
        />仅展示该模块实际声明的操作维度。“—”表示不适用，不等同于未勾选。真实权限须由后端鉴权执行。
      </div>
      <label class="option-line"
        ><input v-model="draft.enabled" type="checkbox" :disabled="readonly" />启用此角色</label
      >
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </div>
    <template #footer
      ><button class="btn" @click="emit('close')">{{ readonly ? '关闭' : '取消' }}</button
      ><button v-if="!readonly" class="btn btn-primary" @click="save">保存角色</button></template
    ></UiDialog
  >
</template>
<style scoped>
.permissions-table th:not(:first-child),
.permissions-table td:not(:first-child) {
  text-align: center;
}
.permissions-table td {
  height: 44px;
}
.permissions-table {
  min-width: 480px;
}
</style>
