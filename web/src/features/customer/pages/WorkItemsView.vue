<script setup lang="ts">
import { UiButton, UiOption, UiSelect } from '@/ui/base'

import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workKindNames } from '@/services/customer/seed'
import PageHeading from '@/ui/common/PageHeading.vue'
import SearchField from '@/ui/common/SearchField.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import AppPagination from '@/ui/common/AppPagination.vue'
import EmptyState from '@/ui/common/EmptyState.vue'
import WorkTable from '@/features/customer/components/WorkTable.vue'
import WorkBoard from '@/features/customer/components/WorkBoard.vue'
import WorkCalendar from '@/features/customer/components/WorkCalendar.vue'

const store = useCustomerStore(),
  actions = useCustomerActions(),
  route = useRoute(),
  router = useRouter()
const query = ref(''),
  kind = ref(''),
  owner = ref(''),
  status = ref(''),
  selected = ref<string[]>([]),
  page = ref(1),
  pageSize = ref(10),
  savedView = ref('')
const applied = ref({ query: '', kind: '', owner: '', status: '' })
const mode = computed(() => String(route.query.view || 'list'))
const validKind = (value: unknown) => {
  const candidate = String(value || '')
  return Object.prototype.hasOwnProperty.call(workKindNames, candidate) ? candidate : ''
}
const fixedKind = computed(() => validKind(route.meta.workKind))
const queryKind = computed(() => validKind(route.query.kind))
const scopeKind = computed(() => fixedKind.value || queryKind.value)
const scopeLabel = computed(() => {
  if (!scopeKind.value) return ''
  if (fixedKind.value && route.meta.title) return String(route.meta.title)
  return workKindNames[scopeKind.value as keyof typeof workKindNames]
})
const isRentalScope = computed(() => Boolean(fixedKind.value && route.meta.module === 'rental'))
const pageTitle = computed(() => {
  if (scopeLabel.value) return `${scopeLabel.value}工作台`
  if (mode.value === 'board') return '客户事项 · 看板'
  if (mode.value === 'calendar') return '客户事项 · 行动日历'
  return '客户事项工作台'
})
const pageDescription = computed(() => {
  const descriptions: Record<string, string> = {
    delivery: '聚合投放排期、现场准备、安装试运行与客户验收事项',
    service: '聚合服务处理、恢复验证与客户确认事项',
    payment: '聚合应收关联、回款沟通、核销与结果核验事项',
    return: '聚合回收排期、退租结算与投放终止事项',
  }
  return descriptions[scopeKind.value] || '以事项推动工作；以真实业务依据验证结果'
})
const view = computed(() => store.snapshot.views.find((v) => v.name === savedView.value))
const filtered = computed(() =>
  store.snapshot.work.filter(
    (w) =>
      (!route.query.customer || w.customerId === route.query.customer) &&
      (!applied.value.query ||
        `${w.id} ${w.title} ${store.customerName(w.customerId)}`.includes(applied.value.query)) &&
      (!applied.value.kind || w.kind === applied.value.kind) &&
      (!applied.value.owner || w.owner === applied.value.owner) &&
      (!applied.value.status || w.status === applied.value.status),
  ),
)
const paged = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)
function apply() {
  applied.value = {
    query: query.value.trim(),
    kind: scopeKind.value || kind.value,
    owner: owner.value,
    status: status.value,
  }
  page.value = 1
  selected.value = []
}
function reset() {
  query.value = ''
  kind.value = scopeKind.value
  owner.value = ''
  status.value = ''
  savedView.value = ''
  apply()
}
function useView() {
  const v = view.value
  if (v) {
    query.value = v.search
    kind.value = scopeKind.value || v.kind
    owner.value = v.owner
    apply()
  }
}
function createWork() {
  actions.open(
    'create-work',
    String(route.query.customer || 'CUS-0186'),
    [],
    scopeKind.value ? { kind: scopeKind.value } : {},
  )
}
watch(
  () => [route.meta.workKind, route.query.kind],
  () => {
    kind.value = scopeKind.value
    apply()
  },
  { immediate: true },
)
watch([page, pageSize], () => (selected.value = []))
</script>
<template>
  <div class="page-stack" :data-work-scope="scopeKind || undefined">
    <PageHeading
      :title="pageTitle"
      :breadcrumb="isRentalScope ? '租赁运营' : '客户经营'"
      :description="pageDescription"
      ><div class="customer-heading-actions">
        <UiButton class="btn btn-primary" @click="createWork">
          <AppIcon name="plus" :size="16" />新建事项
        </UiButton>
      </div></PageHeading
    >
    <div class="card data-panel">
      <nav class="customer-tabs" aria-label="事项工作台视图">
        <UiButton
          v-for="[key, label] in [
            ['list', '列表'],
            ['board', '看板'],
            ['calendar', '日历'],
          ]"
          :key="key"
          :class="{ active: mode === key }"
          @click="router.push({ path: route.path, query: { ...route.query, view: key } })"
        >
          {{ label }}</UiButton><UiButton
          @click="
            () => {
              owner = '张敏'
              apply()
            }
          "
        >
          我的待办</UiButton><UiButton
          @click="
            () => {
              status = '待验收'
              apply()
            }
          "
        >
          待验收</UiButton><UiButton @click="reset()">{{ scopeKind ? '全部本类事项' : '全部事项' }}</UiButton>
      </nav>
      <form class="query-bar" @submit.prevent="apply">
        <SearchField v-model="query" label="搜索客户事项" placeholder="搜索事项标题、编号、客户…" /><UiSelect
          v-if="!scopeKind"
          v-model="kind"
          class="select"
          aria-label="事项类型"
        >
          <UiOption value="">全部类型</UiOption>
          <UiOption v-for="(label, k) in workKindNames" :key="k" :value="k">{{ label }}</UiOption></UiSelect><UiSelect v-model="owner" class="select" aria-label="事项负责人">
          <UiOption value="">全部负责人</UiOption>
          <UiOption>张敏</UiOption>
          <UiOption>李川</UiOption>
          <UiOption>陈晓</UiOption>
          <UiOption>王宁</UiOption></UiSelect><UiSelect v-model="status" class="select" aria-label="事项状态">
          <UiOption value="">全部状态</UiOption>
          <UiOption>待开始</UiOption>
          <UiOption>处理中</UiOption>
          <UiOption>等待客户</UiOption>
          <UiOption>待验收</UiOption>
          <UiOption>已结束</UiOption></UiSelect><UiButton class="btn btn-primary" type="submit">查询</UiButton><UiButton class="btn" type="button" @click="reset">重置</UiButton>
      </form>
      <div class="customer-view-toolbar">
        <div class="row wrap">
          <span class="customer-help">{{
            selected.length ? '当前页已选 ' + selected.length + ' 项' : '共 ' + filtered.length + ' 项'
          }}</span
          ><UiButton
            v-if="mode === 'list'"
            class="btn"
            :disabled="!selected.length"
            @click="actions.open('assign', 'work', selected)"
          >
            批量分配</UiButton><span v-if="scopeLabel" class="pill">{{ scopeLabel }}</span
          ><span v-if="route.query.customer" class="pill">{{
            store.customerName(String(route.query.customer))
          }}</span>
        </div>
        <div class="row wrap">
          <UiSelect
            v-if="store.snapshot.views.length"
            v-model="savedView"
            class="select"
            aria-label="已保存视图"
            @change="useView"
          >
            <UiOption value="">系统视图</UiOption>
            <UiOption v-for="v in store.snapshot.views" :key="v.name">{{ v.name }}</UiOption></UiSelect><UiButton
            class="btn-link"
            @click="
              () => {
                actions.open('save-view', 'work', [], {
                  search: applied.query,
                  kind: applied.kind,
                  owner: applied.owner,
                })
              }
            "
          >
            <AppIcon name="eye" :size="16" />保存视图与显示设置
          </UiButton>
        </div>
      </div>
      <template v-if="mode === 'list'"
        ><WorkTable
          v-if="paged.length"
          v-model:selected="selected"
          :items="paged"
          selectable
          :view="view" /><EmptyState v-else><UiButton class="btn" @click="reset">清空筛选</UiButton></EmptyState
        ><AppPagination v-model:page="page" v-model:page-size="pageSize" :total="filtered.length"
      /></template>
    </div>
    <WorkBoard v-if="mode === 'board'" :items="filtered" /><WorkCalendar
      v-if="mode === 'calendar'"
      :items="filtered"
    />
    <p v-if="mode === 'board'" class="customer-help">
      拖拽或卡片菜单可发起流转，实际变更仍需补齐字段并通过校验。将事项拖入已结束列必须经过验收。
    </p>
  </div>
</template>
