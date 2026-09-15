<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'

import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Department } from '@/types/enterprise'
import { descendantIds, departmentMoveAllowed } from '@/utils/organization'
import PageHeading from '@/ui/common/PageHeading.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import DepartmentTree from '@/features/enterprise/components/organization/DepartmentTree.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppPagination from '@/ui/common/AppPagination.vue'
import EnterpriseSourceBanner from '@/features/enterprise/components/EnterpriseSourceBanner.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  router = useRouter()
const selected = ref(''),
  page = ref(1),
  pageSize = ref(10),
  editOpen = ref(false),
  error = ref('')
const draft = ref<Department>({
  id: '',
  name: '',
  parentId: null,
  leaderId: '',
  code: '',
  description: '',
  enabled: true,
})
const department = computed(() => store.departments.find((d) => d.id === selected.value))
const members = computed(() =>
  store.members.filter(
    (m) =>
      m.status !== 'removed' &&
      (!selected.value || descendantIds(store.departments, selected.value).includes(m.departmentId)),
  ),
)
const paged = computed(() =>
  members.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)
function openEditor(edit: boolean) {
  error.value = ''
  draft.value =
    edit && department.value
      ? { ...department.value }
      : {
          id: crypto.randomUUID(),
          name: '',
          parentId: selected.value || null,
          leaderId: store.members.find((member) => member.status === 'active')?.id ?? '',
          code: '',
          description: '',
          enabled: true,
        }
  editOpen.value = true
}
async function save() {
  try {
    if (!departmentMoveAllowed(store.departments, draft.value.id, draft.value.parentId))
      throw new Error('部门不能移动到自身或其下级部门。')
    if (
      !draft.value.enabled &&
      store.members.some((m) => m.departmentId === draft.value.id && m.status !== 'removed')
    )
      throw new Error('请先转移该部门成员，再停用部门。')
    await store.saveDepartment(draft.value)
    selected.value = draft.value.id
    editOpen.value = false
    ui.toast(store.sourceKind === 'api' ? '组织调整已由服务端确认并回读。' : '组织调整已保存到当前企业预览。')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败。'
  }
}
watch(
  () => store.departments.map((department) => department.id).join(','),
  () => {
    if (selected.value && store.departments.some((department) => department.id === selected.value)) return
    selected.value = store.departments[0]?.id ?? ''
    page.value = 1
  },
  { immediate: true },
)
onMounted(() => void store.ensureDomains(['departments', 'members', 'roles']).catch(() => undefined))
</script>
<template>
  <div class="page-stack" data-enterprise-page="organization" data-ui-template="WorkbenchPage">
    <PageHeading title="组织架构" description="管理部门与汇报关系，让组织协作与数据边界保持清晰" />
    <EnterpriseSourceBanner />
    <div class="metric-grid">
      <MetricCard
        label="部门数量"
        :value="store.departments.length"
        icon="organization"
        caption="包含下级部门"
      /><MetricCard
        label="岗位类型"
        :value="new Set(store.members.map((m) => m.position)).size"
        icon="user"
        caption="按当前成员岗位去重"
      /><MetricCard
        label="启用成员"
        :value="store.members.filter((m) => m.status === 'active').length"
        icon="users"
        tone="green"
        caption="不包含待激活和已禁用成员"
      /><MetricCard
        label="部门负责人"
        :value="new Set(store.departments.map((d) => d.leaderId)).size"
        icon="crown"
        tone="purple"
        caption="按负责人身份去重"
      />
    </div>
    <div class="organization-layout">
      <DepartmentTree
        :selected="selected"
        @select="
          ($event: string) => {
            selected = $event
            page = 1
          }
        "
      />
      <section class="card panel-pad">
        <div class="row-between">
          <div>
            <h2>{{ department?.name || store.company.name }}</h2>
            <p class="department-description">{{ department?.description || '当前企业的全部组织与成员' }}</p>
          </div>
          <div class="row">
            <UiButton v-if="department" class="btn" @click="openEditor(true)">
              <AppIcon name="edit" :size="15" />编辑部门</UiButton><UiButton class="btn btn-primary" @click="openEditor(false)">
              <AppIcon name="plus" :size="16" />新建部门
            </UiButton>
          </div>
        </div>
        <div class="department-overview">
          <div>
            <span>部门负责人</span
            ><strong class="row"
              ><AvatarMark
                :name="store.members.find((m) => m.id === department?.leaderId)?.name || '张三'"
                :size="28"
              />{{ store.members.find((m) => m.id === department?.leaderId)?.name || '张三' }}</strong
            >
          </div>
          <div>
            <span>部门成员</span><strong>{{ members.length }}<small> 人 · 包含下级部门</small></strong>
          </div>
          <div>
            <span>上级部门</span
            ><strong>{{
              department?.parentId ? store.departmentName(department.parentId) : store.company.shortName
            }}</strong>
          </div>
          <div>
            <span>部门状态</span
            ><strong><StatusBadge :text="department?.enabled === false ? '已停用' : '正常'" /></strong>
          </div>
        </div>
        <div class="row-between section-heading">
          <h3>成员列表</h3>
          <UiButton
            class="btn-link"
            @click="router.push({ path: '/enterprise/members', query: { department: selected } })"
          >
            前往成员管理<AppIcon name="right" :size="14" />
          </UiButton>
        </div>
        <div class="table-scroll">
          <table class="data-table org-member-table">
            <thead>
              <tr>
                <th>姓名</th>
                <th>岗位</th>
                <th>所属部门</th>
                <th>角色</th>
                <th>加入时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in paged" :key="m.id">
                <td>
                  <div class="row">
                    <AvatarMark :name="m.name" :size="32" /><span>{{ m.name }}</span>
                  </div>
                </td>
                <td>{{ m.position }}</td>
                <td>{{ store.departmentName(m.departmentId) }}</td>
                <td>
                  <span class="pill">{{ m.roleIds.map(store.roleName).join('、') }}</span>
                </td>
                <td class="muted">{{ m.joinedAt }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <AppPagination v-model:page="page" v-model:page-size="pageSize" :total="members.length" />
      </section>
    </div>
    <UiDialog :open="editOpen" title="部门信息" width="600px" @close="editOpen = false"
      ><div class="form-grid">
        <label class="field"
          ><span class="required">部门名称</span
          ><UiInput v-model="draft.name" class="input" maxlength="40" /></label
        ><label class="field"
          ><span>部门编号</span><UiInput v-model="draft.code" class="input" maxlength="30" :readonly="store.sourceKind === 'api'" :placeholder="store.sourceKind === 'api' ? '服务端合同暂未提供部门编号' : ''" /></label
        ><label class="field"
          ><span>上级部门</span
          ><UiSelect v-model="draft.parentId" class="select">
            <UiOption :value="null">企业根组织</UiOption>
            <UiOption
              v-for="d in store.departments.filter(
                (d) => !descendantIds(store.departments, draft.id).includes(d.id),
              )"
              :key="d.id"
              :value="d.id"
            >
              {{ d.name }}
            </UiOption>
          </UiSelect></label
        ><label class="field"
          ><span>部门负责人</span
          ><UiSelect v-model="draft.leaderId" class="select">
            <UiOption v-for="m in store.members.filter((m) => m.status === 'active')" :key="m.id" :value="m.id">
              {{ m.name }}
            </UiOption>
          </UiSelect></label
        ><label class="field full-width"
          ><span>部门职责</span
          ><UiTextarea v-model="draft.description" class="textarea" maxlength="300" :readonly="store.sourceKind === 'api'" :placeholder="store.sourceKind === 'api' ? '服务端合同暂未提供部门职责字段' : ''" /></label
        ><label class="option-line"><UiInput v-model="draft.enabled" type="checkbox" />启用部门</label>
      </div>
      <div class="notice-box department-notice">
        <AppIcon name="help" />组织调整会影响“所属部门及下级”的数据范围。真实授权变更必须由服务端重新计算。
      </div>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <template #footer
        ><UiButton class="btn" @click="editOpen = false">取消</UiButton><UiButton class="btn btn-primary" @click="save">保存部门</UiButton></template
      ></UiDialog
    >
  </div>
</template>
<style scoped>
.organization-layout {
  display: grid;
  grid-template-columns: 244px minmax(0, 1fr);
  gap: 16px;
}
.organization-layout > * {
  min-width: 0;
}
.department-description {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 7px;
  max-width: 540px;
}
.department-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  padding: 24px 20px;
  background: var(--color-surface-soft);
  border-radius: 10px;
  margin: 24px 0;
}
.department-overview > div > span {
  display: block;
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-bottom: 13px;
}
.department-overview strong {
  font-size: 14px;
  min-height: 28px;
  display: flex;
  align-items: center;
  gap: 5px;
  flex-wrap: wrap;
}
.department-overview strong small {
  font-size: 10px;
  color: var(--color-text-muted);
  font-weight: 400;
}
.section-heading {
  margin-bottom: 16px;
}
.org-member-table {
  min-width: 670px;
}
.org-member-table td {
  height: 56px;
}
.department-notice {
  margin-top: 22px;
}
@media (max-width: 1100px) {
  .organization-layout {
    grid-template-columns: 215px minmax(0, 1fr);
  }
  .department-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .organization-layout .row-between {
    flex-wrap: wrap;
  }
}
@media (max-width: 767px) {
  .organization-layout {
    grid-template-columns: 1fr;
  }
  .organization-layout :deep(.department-tree) {
    max-height: 255px;
    overflow: auto;
  }
  .department-overview {
    padding: 18px;
  }
}
</style>
