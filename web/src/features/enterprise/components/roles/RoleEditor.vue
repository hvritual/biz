<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect, UiTabTrigger, UiTabs } from '@/ui/base'

import { computed, ref, watch } from 'vue'
import type { Role, DataScope } from '@/types/enterprise'
import { scopeLabels } from '@/types/enterprise'
import { permissionCatalog as demoPermissionCatalog } from '@/services/demo/seed'
import {
  availableRolePermissionKeys,
  clearRolePermissionCatalog,
  replaceRolePermissionCatalog,
  rolePermissionGroups,
} from '@/services/enterprise/rolePermissionCatalog'
import { readActionCatalog } from '@/services/runtime/api'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import Notice from '@/ui/common/Notice.vue'

const props = defineProps<{ open: boolean; role: Role | null }>()
const emit = defineEmits<{ close: [] }>()
const store = useEnterpriseStore()
const ui = useUiStore()
const error = ref('')
const busy = ref(false)
const catalogBusy = ref(false)
const catalogReady = ref(false)
const catalogRevision = ref(0)
const permissionFilter = ref<'all' | 'assigned'>('all')

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
      key: group.key,
      name: group.name,
      items: group.permissions.map((item) => ({
        key: item.permission,
        label: item.label,
        description: item.description,
        actions: item.actions,
      })),
    }))
  }
  return demoPermissionCatalog.map((module) => ({
    key: module.id,
    name: module.name,
    items: module.actions.map((action) => ({
      key: `${module.id}.${action}`,
      label: ({ read: '查看', manage: '管理', assign: '分配', export: '导出' } as Record<string, string>)[action] ?? action,
      description: `${module.name} · ${action}`,
      actions: [] as string[],
    })),
  }))
})
const selectedPermissionGroups = computed(() => permissionGroups.value.map((group) => ({
  ...group,
  items: group.items.filter((item) => draft.value.permissions.includes(item.key)),
})).filter((group) => group.items.length > 0))
const totalPermissionCount = computed(() => permissionGroups.value.reduce((count, group) => count + group.items.length, 0))
const visiblePermissionGroups = computed(() => permissionFilter.value === 'all'
  ? permissionGroups.value
  : selectedPermissionGroups.value)

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
    permissionFilter.value = 'all'
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

function selectedInGroup(group: (typeof permissionGroups.value)[number]) {
  return group.items.filter((item) => draft.value.permissions.includes(item.key)).length
}

function groupChecked(group: (typeof permissionGroups.value)[number]) {
  return group.items.length > 0 && selectedInGroup(group) === group.items.length
}

function groupMixed(group: (typeof permissionGroups.value)[number]) {
  const selected = selectedInGroup(group)
  return selected > 0 && selected < group.items.length
}

function toggleGroup(group: (typeof permissionGroups.value)[number], event: Event) {
  if (readonly.value || group.items.length === 0) return
  const keys = new Set(group.items.map((item) => item.key))
  const checked = (event.target as HTMLInputElement).checked
  if (!checked) {
    draft.value.permissions = draft.value.permissions.filter((permission) => !keys.has(permission))
    return
  }
  draft.value.permissions = [...new Set([
    ...draft.value.permissions,
    ...group.items.map((item) => item.key),
  ])]
}

async function revalidatePermissionSelection() {
  if (!apiMode.value) return
  const catalog = await readActionCatalog()
  replaceRolePermissionCatalog(catalog.permissions ?? [])
  catalogRevision.value += 1
  catalogReady.value = true
  const available = availableRolePermissionKeys()
  const unavailable = draft.value.permissions.filter((permission) => !available.has(permission))
  if (unavailable.length === 0) return
  draft.value.permissions = draft.value.permissions.filter((permission) => available.has(permission))
  throw new Error('可配置权限已发生变化，已移除不可用项，请重新确认后保存。')
}

async function save() {
  error.value = ''
  busy.value = true
  try {
    await revalidatePermissionSelection()
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
    width="1080px"
    @close="emit('close')"
  >
    <div class="page-stack">
      <Notice v-if="readonly">
        <AppIcon name="shield" />内置角色只读。企业所有者拥有受保护的管理能力，不能通过此页面修改或禁用。
      </Notice>
      <Notice v-if="apiMode">
        <AppIcon name="help" />当前可配置权限以系统权限目录为准；角色只能选择已开放的权限项。
      </Notice>
      <Notice v-if="apiMode && catalogBusy">正在读取可配置权限…</Notice>

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
      </div>

      <section class="role-permission-workspace" aria-label="功能权限">
        <div class="permission-explorer">
          <div class="permission-workspace-heading"><div><h3>选择权限</h3><p>按业务域勾选，勾选结果会同步显示在右侧。</p></div><div class="permission-statistics"><span>总权限 {{ totalPermissionCount }}</span><strong>已分配 {{ draft.permissions.length }}</strong></div></div>
          <UiTabs :model-value="permissionFilter" label="权限筛选" class="permission-filter" @update:model-value="(value: string) => permissionFilter = value as 'all' | 'assigned'"><template #list><UiTabTrigger value="all">全部（{{ totalPermissionCount }}）</UiTabTrigger><UiTabTrigger value="assigned">已分配（{{ draft.permissions.length }}）</UiTabTrigger></template></UiTabs>
          <div class="permission-tree" data-role-permission-tree>
            <div v-if="visiblePermissionGroups.length" class="permission-groups">
              <section v-for="group in visiblePermissionGroups" :key="group.key" class="permission-group" :class="{ selected: selectedInGroup(group) > 0 }" :data-role-permission-group="group.key">
                <label class="permission-group-header">
                  <UiInput type="checkbox" :aria-label="`${group.name} 全选`" :checked="groupChecked(group)" :indeterminate="groupMixed(group)" :disabled="readonly || group.items.length === 0" :data-role-group-state="groupMixed(group) ? 'mixed' : groupChecked(group) ? 'checked' : 'unchecked'" @change="toggleGroup(group, $event)" />
                  <AppIcon name="folder" :size="18" />
                  <span><strong>{{ group.name }}</strong><small>{{ selectedInGroup(group) }} / {{ group.items.length }}</small></span>
                </label>
                <label v-for="item in group.items" :key="item.key" class="permission-item" :class="{ selected: draft.permissions.includes(item.key) }" :data-role-permission-leaf="item.key">
                  <UiInput type="checkbox" :aria-label="`${group.name} ${item.label}`" :checked="draft.permissions.includes(item.key)" :disabled="readonly" @change="toggle(item.key)" />
                  <span><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span>
                </label>
              </section>
            </div>
            <div v-else class="permission-filter-empty"><AppIcon name="shield" :size="22" /><p>暂无已分配权限</p><UiButton variant="outline" size="sm" @click="permissionFilter = 'all'">查看全部权限</UiButton></div>
          </div>
        </div>
        <aside class="selected-permission-panel" data-role-selected-permissions aria-label="已选权限">
          <header><div><h3>已选权限</h3><p>当前角色将拥有以下权限。</p></div><strong>{{ draft.permissions.length }}</strong></header>
          <div v-if="selectedPermissionGroups.length" class="selected-permission-groups">
            <section v-for="group in selectedPermissionGroups" :key="`selected-${group.key}`" class="selected-permission-group">
              <div class="selected-group-heading"><AppIcon name="folder" :size="17" /><strong>{{ group.name }}</strong><span>{{ group.items.length }}</span></div>
              <ul>
                <li v-for="item in group.items" :key="`selected-${item.key}`"><AppIcon name="check" :size="14" /><div><strong>{{ item.label }}</strong><small>{{ item.description }}</small></div></li>
              </ul>
            </section>
          </div>
          <div v-else class="selected-permission-empty"><AppIcon name="shield" :size="24" /><p>尚未选择权限</p><small>从左侧权限树选择后会显示在这里。</small></div>
        </aside>
      </section>
      <Notice v-if="apiMode && catalogReady && !permissionGroups.length">
        当前企业暂无可配置权限。
      </Notice>

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
.role-permission-workspace { display: grid; grid-template-columns: minmax(0, 1.12fr) minmax(300px, .88fr); gap: 16px; }
.permission-explorer, .selected-permission-panel { box-sizing: border-box; min-width: 0; height: 480px; max-height: 480px; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface); }
.permission-explorer { display: grid; grid-template-rows: auto auto minmax(0, 1fr); padding: 16px; }
.permission-workspace-heading, .selected-permission-panel header { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.permission-workspace-heading h3, .selected-permission-panel h3 { font-size: var(--text-base); }
.permission-workspace-heading p, .selected-permission-panel header p { margin-top: 4px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.5; }
.permission-statistics { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; color: var(--color-text-secondary); font-size: var(--text-xs); }
.permission-statistics strong { padding: 4px 7px; border-radius: 999px; background: var(--color-primary-soft); color: var(--color-primary); font-weight: 600; }
.permission-filter { flex-shrink: 0; margin-top: 10px; }
.permission-filter :deep(.tabs-root) { gap: 0; }
.permission-filter :deep(.tabs-list) { gap: 16px; }
.permission-filter :deep(.tab-trigger) { height: 34px; }
.permission-tree { min-height: 0; }
.permission-group-header .icon { color: var(--color-primary); }
.permission-filter-empty { display: grid; min-height: 0; align-content: center; justify-items: center; gap: 8px; padding: 24px; color: var(--color-text-muted); text-align: center; }
.permission-filter-empty .icon { color: var(--color-primary); }
.permission-filter-empty p { color: var(--color-text-secondary); font-size: var(--text-sm); }
.permission-groups { display: block; height: 100%; min-height: 0; overflow-y: auto; overflow-x: hidden; }
.permission-group { display: block; margin-bottom: 8px; border: 1px solid var(--color-border); border-radius: var(--radius-sm); overflow: hidden; }
.permission-group.selected { border-color: var(--color-primary); }
.permission-group-header { display: grid; grid-template-columns: 18px 18px minmax(0, 1fr); align-items: center; gap: 9px; padding: 12px; background: var(--color-surface-soft); }
.permission-group.selected .permission-group-header, .permission-item.selected { background: var(--color-primary-soft); }
.permission-group-header > [data-slot="input"], .permission-item > [data-slot="input"] { width: 18px; min-width: 18px; height: 18px; padding: 0; }
.permission-group-header > span { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 8px; }
.permission-group-header strong { overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.permission-group-header small { flex-shrink: 0; color: var(--color-text-muted); font-size: 10px; }
.permission-item { display: grid; grid-template-columns: 18px minmax(0, 1fr); align-items: flex-start; gap: 9px; padding: 10px 12px 10px 42px; border-top: 1px solid var(--color-border); }
.permission-item strong, .permission-item small { display: block; }
.permission-item strong { font-size: 12px; }
.permission-item small { margin-top: 3px; color: var(--color-text-muted); font-size: 10px; }
.selected-permission-panel { display: grid; min-height: 0; grid-template-rows: auto minmax(0, 1fr); overflow: hidden; }
.selected-permission-panel header { flex-shrink: 0; padding: 16px; border-bottom: 1px solid var(--color-border); background: var(--color-surface-soft); }
.selected-permission-panel header > strong { display: grid; min-width: 28px; height: 28px; place-items: center; border-radius: 999px; background: var(--color-primary-soft); color: var(--color-primary); font-size: var(--text-sm); }
.selected-permission-groups { display: block; min-height: 0; overflow-y: auto; overflow-x: hidden; }
.selected-permission-group { display: block; border-bottom: 1px solid var(--color-border); }
.selected-group-heading { display: flex; align-items: center; gap: 8px; padding: 12px 16px; }
.selected-group-heading .icon { color: var(--color-primary); }
.selected-group-heading strong { flex: 1; font-size: var(--text-sm); }
.selected-group-heading span { color: var(--color-text-muted); font-size: var(--text-xs); }
.selected-permission-group ul { display: flex; flex-direction: column; gap: 8px; margin: 0; padding: 0 16px 14px; list-style: none; }
.selected-permission-group li { display: flex; align-items: flex-start; gap: 8px; }
.selected-permission-group li > .icon { margin-top: 2px; color: var(--color-success); }
.selected-permission-group li strong, .selected-permission-group li small { display: block; }
.selected-permission-group li strong { font-size: var(--text-xs); }
.selected-permission-group li small { margin-top: 2px; color: var(--color-text-muted); font-size: 10px; line-height: 1.45; }
.selected-permission-empty { display: grid; min-height: 0; align-content: center; justify-items: center; gap: 6px; padding: 32px 18px; text-align: center; color: var(--color-text-muted); }
.selected-permission-empty .icon { color: var(--color-primary); }
.selected-permission-empty p { color: var(--color-text-secondary); font-size: var(--text-sm); font-weight: 600; }
.selected-permission-empty small { font-size: var(--text-xs); line-height: 1.5; }
@media (max-width: 767px) { .role-permission-workspace { grid-template-columns: 1fr; } }
</style>
