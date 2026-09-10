<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Department } from '@/types/enterprise'
import { descendantIds, departmentMoveAllowed } from '@/utils/organization'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import DepartmentTree from '@/components/organization/DepartmentTree.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  router = useRouter()
const selected = ref('operations'),
  page = ref(1),
  pageSize = ref(10),
  editOpen = ref(false),
  error = ref('')
const draft = ref<Department>({
  id: '',
  name: '',
  parentId: null,
  leaderId: 'member-1',
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
          leaderId: 'member-1',
          code: '',
          description: '',
          enabled: true,
        }
  editOpen.value = true
}
function save() {
  try {
    if (!departmentMoveAllowed(store.departments, draft.value.id, draft.value.parentId))
      throw new Error('部门不能移动到自身或其下级部门。')
    if (
      !draft.value.enabled &&
      store.members.some((m) => m.departmentId === draft.value.id && m.status !== 'removed')
    )
      throw new Error('请先转移该部门成员，再停用部门。')
    store.saveDepartment(draft.value)
    selected.value = draft.value.id
    editOpen.value = false
    ui.toast('组织调整已保存到当前企业预览。')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败。'
  }
}
</script>
<template>
  <div class="page-stack">
    <PageHeading title="组织架构" description="管理部门与汇报关系，让组织协作与数据边界保持清晰" />
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
            <button v-if="department" class="btn" @click="openEditor(true)">
              <AppIcon name="edit" :size="15" />编辑部门</button
            ><button class="btn btn-primary" @click="openEditor(false)">
              <AppIcon name="plus" :size="16" />新建部门
            </button>
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
          <button
            class="btn-link"
            @click="router.push({ path: '/enterprise/members', query: { department: selected } })"
          >
            前往成员管理<AppIcon name="right" :size="14" />
          </button>
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
          ><input v-model="draft.name" class="input" maxlength="40" /></label
        ><label class="field"
          ><span>部门编号</span><input v-model="draft.code" class="input" maxlength="30" /></label
        ><label class="field"
          ><span>上级部门</span
          ><select v-model="draft.parentId" class="select">
            <option :value="null">企业根组织</option>
            <option
              v-for="d in store.departments.filter(
                (d) => !descendantIds(store.departments, draft.id).includes(d.id),
              )"
              :key="d.id"
              :value="d.id"
            >
              {{ d.name }}
            </option>
          </select></label
        ><label class="field"
          ><span>部门负责人</span
          ><select v-model="draft.leaderId" class="select">
            <option v-for="m in store.members.filter((m) => m.status === 'active')" :key="m.id" :value="m.id">
              {{ m.name }}
            </option>
          </select></label
        ><label class="field full-width"
          ><span>部门职责</span
          ><textarea v-model="draft.description" class="textarea" maxlength="300" /></label
        ><label class="option-line"><input v-model="draft.enabled" type="checkbox" />启用部门</label>
      </div>
      <div class="notice-box department-notice">
        <AppIcon name="help" />组织调整会影响“所属部门及下级”的数据范围。真实授权变更必须由服务端重新计算。
      </div>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <template #footer
        ><button class="btn" @click="editOpen = false">取消</button
        ><button class="btn btn-primary" @click="save">保存部门</button></template
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
