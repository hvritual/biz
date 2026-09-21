<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'

import { computed, ref, watch } from 'vue'
import type { Role, DataScope } from '@/types/enterprise'
import { scopeLabels } from '@/types/enterprise'
import { permissionCatalog as demoPermissionCatalog } from '@/services/demo/seed'
import {
  clearRolePermissionCatalog,
  replaceRolePermissionCatalog,
  rolePermissionGroups,
} from '@/services/enterprise/rolePermissionCatalog'
import { readActionCatalog } from '@/services/runtime/api'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppIcon from '@/ui/common/AppIcon.vue'

const props = defineProps<{ open: boolean; role: Role | null }>()
const emit = defineEmits<{ close: [] }>()
const store = useEnterpriseStore()
const ui = useUiStore()
const error = ref('')
const busy = ref(false)
const catalogBusy = ref(false)
const catalogReady = ref(false)
const catalogRevision = ref(0)

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
const apiMode = computed(() => store.sourceKind === 'api')
const permissionGroups = computed(() => {
  // rolePermissionCatalog is a presentation cache populated from the server.
  // Track a local revision so Vue invalidates this computed value after refresh.
  void catalogRevision.value
  if (apiMode.value) {
    return rolePermissionGroups().map((group) => ({
      name: group.name,
      items: group.permissions.map((item) => ({
        key: item.permission,
        label: item.label,
        description: item.description,
      })),
    }))
  }
  return demoPermissionCatalog.map((module) => ({
    name: module.name,
    items: module.actions.map((action) => ({
      key: `${module.id}.${action}`,
      label: ({ read: '查看', manage: '管理', assign: '分配', export: '导出' } as Record<string, string>)[action] ?? action,
      description: `${module.name} · ${action}`,
    })),
  }))
})

async function loadServerPermissionCatalog() {
  if (!apiMode.value || !props.open) {
    catalogReady.value = !apiMode.value
    if (!apiMode.value) clearRolePermissionCatalog()
    return
  }
  catalogBusy.value = true
  catalogReady.value = false
  try {
    const catalog = await readActionCatalog()
    replaceRolePermissionCatalog(catalog.permissions ?? [])
    catalogRevision.value += 1
    catalogReady.value = true
  } catch (cause) {
    clearRolePermissionCatalog()
    catalogRevision.value += 1
    error.value = cause instanceof Error ? cause.message : '权限目录读取失败，请稍后重试。'
  } finally {
    catalogBusy.value = false
  }
}

watch(
  () => [props.open, props.role, store.sourceKind] as const,
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
          scope: apiMode.value ? 'custom' : 'department',
          permissions: [],
          updatedAt: '',
        }
    void loadServerPermissionCatalog()
  },
  { immediate: true },
)

function toggle(value: string) {
  if (readonly.value) return
  draft.value.permissions = draft.value.permissions.includes(value)
    ? draft.value.permissions.filter((permission) => permission !== value)
    : [...draft.value.permissions, value]
}

async function save() {
  error.value = ''
  busy.value = true
  try {
    await store.saveRole(draft.value)
    ui.toast(
      store.previewMode
        ? '角色配置已保存。'
        : '角色配置已保存并更新。',
      'success',
    )
    emit('close')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '保存失败。'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <UiDialog
    :open="open"
    :title="readonly ? '内置角色详情' : role ? '编辑角色权限' : '新建角色'"
    width="850px"
    @close="emit('close')"
  >
    <div class="page-stack">
      <div v-if="readonly" class="notice-box">
        <AppIcon name="shield" />内置角色只读。企业所有者拥有受保护的管理能力，不能通过此页面修改或禁用。
      </div>
      <div v-if="apiMode" class="notice-box">
        <AppIcon name="help" />当前可配置权限以系统权限目录为准；角色只能选择已开放的权限项。
      </div>
      <div v-if="apiMode && catalogBusy" class="notice-box">正在读取可配置权限…</div>

      <div class="form-grid">
        <label class="field">
          <span class="required">角色名称</span>
          <UiInput
            v-model="draft.name"
            class="input"
            maxlength="40"
            :readonly="readonly"
            placeholder="例如：华东区域运营"
          />
        </label>
        <label class="field">
          <span>数据范围</span>
          <UiSelect v-model="draft.scope" class="select" :disabled="readonly">
            <template v-if="apiMode">
              <UiOption value="all">全部数据</UiOption>
              <UiOption value="self">仅本人</UiOption>
              <UiOption value="custom">授权点位 / 指定数据</UiOption>
            </template>
            <template v-else>
              <UiOption v-for="(label, key) in scopeLabels" :key="key" :value="key as DataScope">
                {{ label }}
              </UiOption>
            </template>
          </UiSelect>
        </label>
        <label class="field full-width">
          <span>角色说明</span>
          <UiTextarea
            v-model="draft.description"
            class="textarea"
            rows="2"
            :readonly="readonly"
            maxlength="120"
            placeholder="描述该角色的职责和授权边界（最多 120 字）"
          />
        </label>
      </div>

      <div class="row-between">
        <h3>功能权限</h3>
        <span class="muted">已选 {{ draft.permissions.length }} 项</span>
      </div>
      <div class="permission-groups">
        <section v-for="group in permissionGroups" :key="group.name" class="permission-group">
          <h4>{{ group.name }}</h4>
          <label v-for="item in group.items" :key="item.key" class="permission-item">
            <UiInput
              type="checkbox"
              :aria-label="`${group.name} ${item.label}`"
              :checked="draft.permissions.includes(item.key)"
              :disabled="readonly"
              @change="toggle(item.key)"
            />
            <span>
              <strong>{{ item.label }}</strong>
              <small>{{ item.description }}</small>
            </span>
          </label>
        </section>
      </div>

      <label class="option-line">
        <UiInput v-model="draft.enabled" type="checkbox" :disabled="readonly" />启用此角色
      </label>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </div>

    <template #footer>
      <UiButton class="btn" @click="emit('close')">{{ readonly ? '关闭' : '取消' }}</UiButton>
      <UiButton v-if="!readonly" class="btn btn-primary" :disabled="busy || (apiMode && !catalogReady)" @click="save">
        {{ busy ? '正在保存…' : '保存角色' }}
      </UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.permission-groups {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  max-height: 360px;
  overflow: auto;
}
.permission-group {
  border: 1px solid var(--color-border);
  border-radius: 9px;
  padding: 14px;
}
.permission-group h4 {
  font-size: 13px;
  margin-bottom: 10px;
}
.permission-item {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 8px 0;
}
.permission-item strong,
.permission-item small {
  display: block;
}
.permission-item strong {
  font-size: 12px;
}
.permission-item small {
  margin-top: 3px;
  color: var(--color-text-muted);
  font-size: 10px;
}
@media (max-width: 767px) {
  .permission-groups { grid-template-columns: 1fr; }
}
</style>
