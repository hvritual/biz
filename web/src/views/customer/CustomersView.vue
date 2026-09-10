<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { downloadCsv } from '@/utils/format'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import SearchField from '@/components/ui/SearchField.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
const store = useCustomerStore(),
  actions = useCustomerActions()
const search = ref(''),
  lifecycle = ref(''),
  owner = ref(''),
  applied = ref({ search: '', lifecycle: '', owner: '' }),
  tab = ref('全部客户'),
  page = ref(1),
  pageSize = ref(10),
  more = ref('')
const filtered = computed(() =>
  store.snapshot.customers.filter(
    (c) =>
      (tab.value === '已归档' ? c.archived : !c.archived) &&
      (tab.value !== '我的客户' || c.owner === '张敏') &&
      (tab.value !== '存在风险' || ['服务风险', '回款风险', '到期风险'].includes(c.risk)) &&
      (!applied.value.search || `${c.name} ${c.id}`.includes(applied.value.search)) &&
      (!applied.value.lifecycle || c.lifecycle === applied.value.lifecycle) &&
      (!applied.value.owner || c.owner === applied.value.owner),
  ),
)
const paged = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)
const target = computed(() => store.snapshot.customers.find((c) => c.id === more.value))
function apply() {
  applied.value = { search: search.value.trim(), lifecycle: lifecycle.value, owner: owner.value }
  page.value = 1
}
function clear() {
  search.value = ''
  lifecycle.value = ''
  owner.value = ''
  apply()
}
function exportList() {
  downloadCsv('客户列表-预览.csv', [
    ['客户', '生命周期', '负责人', '有效点位', '关联设备'],
    ...filtered.value.map((c) => [c.name, c.lifecycle, c.owner, c.sites, c.devices]),
  ])
}
watch(tab, () => (page.value = 1))
</script>
<template>
  <div class="page-stack">
    <PageHeading title="客户总览" breadcrumb="客户经营" description="以客户为中心管理关系、风险与下一步行动"
      ><div class="customer-heading-actions">
        <RouterLink class="btn" to="/customers/import"
          ><AppIcon name="upload" :size="16" />导入客户</RouterLink
        ><button class="btn btn-primary" @click="actions.open('create-customer')">
          <AppIcon name="plus" :size="16" />新建客户
        </button>
      </div></PageHeading
    >
    <div class="metric-grid">
      <MetricCard
        label="合作客户"
        :value="store.snapshot.customers.filter((c) => c.lifecycle === '合作中' && !c.archived).length"
        icon="customer"
        tone="green"
        caption="按当前租户有效客户统计"
      /><MetricCard
        label="风险客户"
        :value="
          store.snapshot.customers.filter((c) => ['服务风险', '回款风险', '到期风险'].includes(c.risk)).length
        "
        icon="warning"
        tone="orange"
        caption="服务、回款或到期风险"
      /><MetricCard
        label="待推进续约"
        :value="store.openWork.filter((w) => w.kind === 'renewal').length"
        icon="file"
        caption="按未结束续约事项统计"
      /><MetricCard
        label="未结束事项"
        :value="store.openWork.length"
        icon="checks"
        tone="purple"
        caption="下一步行动需要持续跟进"
      />
    </div>
    <section class="card data-panel">
      <nav class="customer-tabs" aria-label="客户视图">
        <button
          v-for="name in ['全部客户', '我的客户', '存在风险', '已归档']"
          :key="name"
          :class="{ active: tab === name }"
          @click="tab = name"
        >
          {{ name }}
        </button>
      </nav>
      <form class="query-bar" @submit.prevent="apply">
        <SearchField v-model="search" label="搜索客户" placeholder="搜索客户名称、客户编号…" /><select
          v-model="lifecycle"
          class="select"
          aria-label="客户生命周期"
        >
          <option value="">全部生命周期</option>
          <option>潜在客户</option>
          <option>试用中</option>
          <option>合作中</option>
          <option>合作终止</option></select
        ><select v-model="owner" class="select" aria-label="客户负责人">
          <option value="">全部负责人</option>
          <option>张敏</option>
          <option>李川</option>
          <option>陈晓</option>
          <option>王宁</option></select
        ><button class="btn btn-primary" type="submit">查询</button
        ><button class="btn" type="button" @click="clear">重置</button>
      </form>
      <div class="customer-view-toolbar">
        <div class="customer-filter-chips" style="padding: 0">
          <span class="customer-help">已生效条件：</span
          ><span class="pill">{{ applied.lifecycle || '全部生命周期' }}</span
          ><span class="pill">{{ applied.owner || '全部负责人' }}</span
          ><span v-if="applied.search" class="pill">{{ applied.search }}</span>
        </div>
        <button class="btn-link" @click="exportList">
          <AppIcon name="download" :size="16" />导出当前结果
        </button>
      </div>
      <div v-if="paged.length" class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>客户名称</th>
              <th>类型 / 生命周期</th>
              <th>风险原因</th>
              <th>客户负责人</th>
              <th>点位 / 设备</th>
              <th>未结束事项</th>
              <th>下一次行动</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in paged" :key="c.id">
              <td>
                <RouterLink class="btn-link mainline" :to="`/customers/accounts/${c.id}`">{{
                  c.name
                }}</RouterLink>
                <p class="subline">{{ c.id }} · {{ c.area }}</p>
              </td>
              <td>
                <p>{{ c.category }}</p>
                <StatusBadge :text="c.lifecycle" :tone="c.lifecycle === '合作中' ? 'success' : 'neutral'" />
              </td>
              <td>
                <StatusBadge
                  :text="c.risk"
                  :tone="['服务风险', '回款风险', '到期风险'].includes(c.risk) ? 'warning' : 'neutral'"
                />
              </td>
              <td>
                <span class="row"><AvatarMark :name="c.owner" :size="25" />{{ c.owner }}</span>
              </td>
              <td>{{ c.sites }} / {{ c.devices }}</td>
              <td>{{ store.openWork.filter((w) => w.customerId === c.id).length }}</td>
              <td>{{ c.nextContact }}</td>
              <td>
                <div class="table-actions">
                  <RouterLink class="btn-link" :to="`/customers/accounts/${c.id}`">工作区</RouterLink
                  ><button class="btn-link" :aria-label="`管理 ${c.name}`" @click="more = c.id">更多</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-else
        ><button
          class="btn"
          @click="
            () => {
              clear()
              tab = '全部客户'
            }
          "
        >
          清空筛选
        </button></EmptyState
      ><AppPagination v-model:page="page" v-model:page-size="pageSize" :total="filtered.length" />
    </section>
    <UiDialog
      :open="Boolean(more)"
      :title="`${target?.name || ''} · 客户操作`"
      width="440px"
      @close="more = ''"
      ><div class="customer-checklist">
        <button
          class="btn"
          @click="
            () => {
              actions.open('edit-customer', more)
              more = ''
            }
          "
        >
          编辑客户资料</button
        ><button
          class="btn"
          @click="
            () => {
              actions.open('handover', more)
              more = ''
            }
          "
        >
          客户与事项移交</button
        ><button
          class="btn"
          @click="
            () => {
              actions.open(target?.archived ? 'restore' : 'archive', more)
              more = ''
            }
          "
        >
          {{ target?.archived ? '恢复客户' : '归档前检查' }}
        </button>
      </div></UiDialog
    >
  </div>
</template>
