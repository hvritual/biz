<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workKindNames } from '@/services/customer/seed'
import PageHeading from '@/components/ui/PageHeading.vue'
import SearchField from '@/components/ui/SearchField.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import WorkTable from '@/components/customer/WorkTable.vue'
import WorkBoard from '@/components/customer/WorkBoard.vue'
import WorkCalendar from '@/components/customer/WorkCalendar.vue'
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
  applied.value = { query: query.value.trim(), kind: kind.value, owner: owner.value, status: status.value }
  page.value = 1
  selected.value = []
}
function reset() {
  query.value = ''
  kind.value = ''
  owner.value = ''
  status.value = ''
  savedView.value = ''
  apply()
}
function useView() {
  const v = view.value
  if (v) {
    query.value = v.search
    kind.value = v.kind
    owner.value = v.owner
    apply()
  }
}
watch(
  () => route.query.kind,
  (k) => {
    kind.value = String(k || '')
    apply()
  },
  { immediate: true },
)
watch([page, pageSize], () => (selected.value = []))
</script>
<template>
  <div class="page-stack">
    <PageHeading
      :title="
        mode === 'board' ? '客户事项 · 看板' : mode === 'calendar' ? '客户事项 · 行动日历' : '客户事项工作台'
      "
      breadcrumb="客户经营"
      description="以事项推动工作；以真实业务依据验证结果"
      ><div class="customer-heading-actions">
        <button
          class="btn btn-primary"
          @click="actions.open('create-work', String(route.query.customer || 'CUS-0186'))"
        >
          <AppIcon name="plus" :size="16" />新建事项
        </button>
      </div></PageHeading
    >
    <div class="card data-panel">
      <nav class="customer-tabs" aria-label="事项工作台视图">
        <button
          v-for="[key, label] in [
            ['list', '列表'],
            ['board', '看板'],
            ['calendar', '日历'],
          ]"
          :key="key"
          :class="{ active: mode === key }"
          @click="router.push({ path: '/customers/work', query: { ...route.query, view: key } })"
        >
          {{ label }}</button
        ><button
          @click="
            () => {
              owner = '张敏'
              apply()
            }
          "
        >
          我的待办</button
        ><button
          @click="
            () => {
              status = '待验收'
              apply()
            }
          "
        >
          待验收</button
        ><button @click="reset()">全部事项</button>
      </nav>
      <form class="query-bar" @submit.prevent="apply">
        <SearchField v-model="query" label="搜索客户事项" placeholder="搜索事项标题、编号、客户…" /><select
          v-model="kind"
          class="select"
          aria-label="事项类型"
        >
          <option value="">全部类型</option>
          <option v-for="(label, k) in workKindNames" :key="k" :value="k">{{ label }}</option></select
        ><select v-model="owner" class="select" aria-label="事项负责人">
          <option value="">全部负责人</option>
          <option>张敏</option>
          <option>李川</option>
          <option>陈晓</option>
          <option>王宁</option></select
        ><select v-model="status" class="select" aria-label="事项状态">
          <option value="">全部状态</option>
          <option>待开始</option>
          <option>处理中</option>
          <option>等待客户</option>
          <option>待验收</option>
          <option>已结束</option></select
        ><button class="btn btn-primary" type="submit">查询</button
        ><button class="btn" type="button" @click="reset">重置</button>
      </form>
      <div class="customer-view-toolbar">
        <div class="row wrap">
          <span class="customer-help">{{
            selected.length ? '当前页已选 ' + selected.length + ' 项' : '共 ' + filtered.length + ' 项'
          }}</span
          ><button
            v-if="mode === 'list'"
            class="btn"
            :disabled="!selected.length"
            @click="actions.open('assign', 'work', selected)"
          >
            批量分配</button
          ><span v-if="route.query.customer" class="pill">{{
            store.customerName(String(route.query.customer))
          }}</span>
        </div>
        <div class="row wrap">
          <select
            v-if="store.snapshot.views.length"
            v-model="savedView"
            class="select"
            aria-label="已保存视图"
            @change="useView"
          >
            <option value="">系统视图</option>
            <option v-for="v in store.snapshot.views" :key="v.name">{{ v.name }}</option></select
          ><button
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
          </button>
        </div>
      </div>
      <template v-if="mode === 'list'"
        ><WorkTable
          v-if="paged.length"
          v-model:selected="selected"
          :items="paged"
          selectable
          :view="view" /><EmptyState v-else><button class="btn" @click="reset">清空筛选</button></EmptyState
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
