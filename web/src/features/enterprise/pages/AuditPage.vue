<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import PageHeader from "@/shared/ui/PageHeader.vue";
import MetricCard from "@/shared/ui/MetricCard.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import EmptyState from "@/shared/ui/EmptyState.vue";
import TablePagination from "@/shared/ui/TablePagination.vue";
import AuditDetailDialog from "../components/AuditDetailDialog.vue";
import { useAuditStore, type AuditRecord } from "@/services/audit";
import { useTenantStore } from "@/services/tenant";
import { downloadText, csvCell } from "@/shared/lib/download";
const store = useAuditStore();
const tenant = useTenantStore();
const query = reactive({ keyword: "", risk: "", result: "" });
const page = ref(1);
const pageSize = ref(8);
const detail = ref<AuditRecord>();
const filtered = computed(() =>
  store.records
    .filter(
      (r) =>
        (!query.keyword ||
          [r.actor, r.action, r.target, r.requestId].some((v) =>
            v.includes(query.keyword),
          )) &&
        (!query.risk || r.risk === query.risk) &&
        (!query.result || r.result === query.result),
    )
    .sort((a, b) => b.time.localeCompare(a.time)),
);
const rows = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
);
watch(query, () => (page.value = 1));
watch(pageSize, () => (page.value = 1));
watch(
  () => tenant.tenantId,
  () => {
    detail.value = undefined;
    page.value = 1;
    Object.assign(query, { keyword: "", risk: "", result: "" });
  },
);
function exportLogs() {
  downloadText(
    "coffeelink-audit-preview.csv",
    "\uFEFF" +
      [
        ["时间", "操作人", "动作", "对象", "结果", "请求ID"],
        ...filtered.value.map((r) => [
          r.time,
          r.actor,
          r.action,
          r.target,
          r.result,
          r.requestId,
        ]),
      ]
        .map((r) => r.map(csvCell).join(","))
        .join("\r\n"),
    "text/csv;charset=utf-8",
  );
}
</script>
<template>
  <PageHeader
    title="操作日志"
    description="追踪成员操作、权限变更与企业设置，建立清晰可回溯的操作记录"
    ><BaseButton icon="Download" @click="exportLogs">导出日志</BaseButton></PageHeader
  >
  <div class="metrics-grid">
    <MetricCard
      title="操作记录"
      :value="store.records.length"
      icon="FileText"
      note="当前企业演示会话"
    /><MetricCard
      title="高风险操作"
      :value="store.records.filter((r) => r.risk === '高风险').length"
      icon="ShieldAlert"
      tone="red"
      note="按记录风险等级统计"
    /><MetricCard
      title="权限变更"
      :value="store.records.filter((r) => /权限|角色/.test(r.action)).length"
      icon="KeyRound"
      tone="purple"
      note="成员角色及数据范围调整"
    /><MetricCard
      title="导出次数"
      :value="store.records.filter((r) => r.action.includes('导出')).length"
      icon="Download"
      note="当前演示数据导出"
    />
  </div>
  <section class="panel">
    <div class="query-bar">
      <div class="search-field">
        <AppIcon name="Search" :size="17" /><input
          v-model="query.keyword"
          aria-label="搜索操作日志"
          placeholder="搜索操作人、操作内容、对象名称、请求 ID…"
        />
      </div>
      <select v-model="query.risk" aria-label="风险等级">
        <option value="">全部风险等级</option>
        <option>低风险</option>
        <option>中风险</option>
        <option>高风险</option></select
      ><select v-model="query.result" aria-label="操作结果">
        <option value="">全部结果</option>
        <option>成功</option>
        <option>失败</option></select
      ><BaseButton
        icon="RefreshCw"
        variant="quiet"
        @click="Object.assign(query, { keyword: '', risk: '', result: '' })"
        >重置</BaseButton
      >
    </div>
    <div class="table-scroll">
      <table v-if="rows.length">
        <thead>
          <tr>
            <th>时间</th>
            <th>操作人</th>
            <th>模块</th>
            <th>操作内容</th>
            <th>操作对象</th>
            <th>结果</th>
            <th>风险等级</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="record in rows" :key="record.id">
            <td class="muted numeric">{{ record.time }}</td>
            <td>{{ record.actor }}</td>
            <td class="muted">{{ record.module }}</td>
            <td>{{ record.action }}</td>
            <td class="muted">{{ record.target }}</td>
            <td>
              <StatusBadge :tone="record.result === '成功' ? 'green' : 'red'" dot>{{
                record.result
              }}</StatusBadge>
            </td>
            <td>
              <StatusBadge
                :tone="
                  record.risk === '高风险'
                    ? 'red'
                    : record.risk === '中风险'
                      ? 'orange'
                      : 'blue'
                "
                >{{ record.risk }}</StatusBadge
              >
            </td>
            <td>
              <button
                class="text-button"
                :aria-label="`查看日志 ${record.id}`"
                @click="detail = record"
              >
                查看
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState v-else />
    </div>
    <TablePagination
      v-model:page="page"
      v-model:page-size="pageSize"
      :total="filtered.length"
    />
  </section>
  <div class="info-banner">
    当前显示本地演示日志。正式审计应使用服务端生成、不可篡改的记录，并关联请求回执与执行结果。
  </div>
  <AuditDetailDialog :record="detail" @close="detail = undefined" />
</template>
