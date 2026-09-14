<script setup lang="ts">
import { computed } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { quickActions } from '@/router/navigation'
const store = useEnterpriseStore()
const groups = computed(() => [
  { name: '启用', count: store.members.filter((m) => m.status === 'active').length, tone: 'green' },
  { name: '待激活', count: store.members.filter((m) => m.status === 'invited').length, tone: 'orange' },
  { name: '已禁用', count: store.members.filter((m) => m.status === 'suspended').length, tone: 'red' },
  { name: '已移除', count: store.members.filter((m) => m.status === 'removed').length, tone: 'gray' },
])
const donutBackground = computed(() => {
  const colors = [
    'var(--color-success)',
    'var(--color-warning)',
    'var(--color-danger)',
    'var(--color-text-muted)',
  ]
  let offset = 0
  return `conic-gradient(${groups.value
    .map((group, index) => {
      const start = offset
      offset += store.members.length ? (group.count / store.members.length) * 100 : 0
      return `${colors[index]} ${start}% ${offset}%`
    })
    .join(',')})`
})
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="你好，张三"
      breadcrumb="工作台"
      description="从成员、权限与组织协作开始，管理你的企业工作空间"
      banner
    />
    <div class="metric-grid">
      <MetricCard
        label="企业成员"
        :value="store.members.length"
        icon="users"
        caption="当前企业成员关系总览"
      /><MetricCard
        label="启用成员"
        :value="groups[0]!.count"
        icon="success"
        tone="green"
        caption="具备有效企业访问状态"
      /><MetricCard
        label="待激活成员"
        :value="groups[1]!.count"
        icon="clock"
        tone="orange"
        caption="待完成邀请激活"
      /><MetricCard
        label="角色数量"
        :value="store.roles.length"
        icon="shield"
        tone="purple"
        caption="按岗位授予必要权限"
      />
    </div>
    <div class="workbench-grid">
      <section class="card panel-pad">
        <div class="row-between block-title">
          <h2>成员生命周期概览</h2>
          <RouterLink class="btn-link" to="/enterprise/members"
            >查看成员<AppIcon name="right" :size="14"
          /></RouterLink>
        </div>
        <div class="lifecycle-summary">
          <div class="donut" :style="{ background: donutBackground }">
            <div>
              <strong>{{ store.members.length }}</strong
              ><small>成员总数</small>
            </div>
          </div>
          <div class="lifecycle-legend">
            <div v-for="g in groups" :key="g.name" class="legend-row">
              <i :class="g.tone" /><span>{{ g.name }}</span
              ><strong>{{ g.count }}</strong
              ><small
                >{{
                  store.members.length ? ((g.count / store.members.length) * 100).toFixed(1) : '0.0'
                }}%</small
              >
            </div>
          </div>
        </div>
      </section>
      <section class="card panel-pad">
        <h2 class="block-title">快捷开始</h2>
        <div class="workbench-actions">
          <RouterLink v-for="a in quickActions" :key="a.label" :to="a.path"
            ><span><AppIcon :name="a.icon" :size="23" /></span><strong>{{ a.label }}</strong></RouterLink
          >
        </div>
      </section>
      <section class="card panel-pad">
        <div class="row-between block-title">
          <h2>最近操作</h2>
          <RouterLink class="btn-link" to="/enterprise/logs">查看全部</RouterLink>
        </div>
        <div v-for="log in store.logs.slice(0, 6)" :key="log.id" class="recent-action">
          <span class="recent-icon"><AppIcon name="file" :size="18" /></span>
          <div class="flex-1">
            <strong>{{ log.action }}</strong>
            <p>{{ log.actor }} · {{ log.target }}</p>
          </div>
          <time>{{ log.time.slice(11, 16) }}</time>
        </div>
      </section>
      <section class="card panel-pad">
        <h2 class="block-title">待处理事项</h2>
        <RouterLink class="todo-row" to="/enterprise/members"
          ><span><AppIcon name="invite" />成员邀请待激活</span
          ><StatusBadge :text="String(groups[1]!.count)" tone="warning" :dot="false" /></RouterLink
        ><RouterLink class="todo-row" to="/enterprise/roles"
          ><span><AppIcon name="shield" />复核角色与数据范围</span
          ><AppIcon name="right" :size="15" /></RouterLink
        ><RouterLink class="todo-row" to="/system/security"
          ><span><AppIcon name="key" />检查企业安全策略</span><AppIcon name="right" :size="15"
        /></RouterLink>
        <div class="notice-box workbench-notice">
          <AppIcon name="help" />设备、订单与出杯数据尚未接入，此工作台不展示虚构的实时设备指标。
        </div>
      </section>
    </div>
  </div>
</template>
<style scoped>
.workbench-grid {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  gap: 16px;
}
.lifecycle-summary {
  display: flex;
  align-items: center;
  gap: 36px;
  padding: 18px 10px;
}
.donut {
  width: 176px;
  height: 176px;
  flex-shrink: 0;
  border-radius: 50%;
  padding: 19px;
  transform: rotate(-90deg);
}
.donut > div {
  height: 100%;
  border-radius: 50%;
  background: var(--color-surface);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  transform: rotate(90deg);
}
.donut strong {
  font-size: 28px;
  font-weight: 650;
}
.donut small {
  color: var(--color-text-muted);
  font-size: 12px;
}
.lifecycle-legend {
  flex: 1;
}
.legend-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 0;
  border-bottom: 1px solid var(--color-border);
  font-size: 13px;
}
.legend-row i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-success);
}
.legend-row i.orange {
  background: var(--color-warning);
}
.legend-row i.red {
  background: var(--color-danger);
}
.legend-row i.gray {
  background: var(--color-text-muted);
}
.legend-row > span {
  flex: 1;
}
.legend-row > small {
  width: 43px;
  color: var(--color-text-muted);
  text-align: right;
}
.workbench-actions {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px 12px;
  padding: 10px 0;
}
.workbench-actions > a {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 9px;
  font-size: 12px;
  color: var(--color-text);
}
.workbench-actions > a > span {
  height: 47px;
  width: 47px;
  border: 1px solid var(--color-border);
  background: linear-gradient(145deg, var(--color-primary-soft), var(--color-surface));
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: var(--color-primary);
}
.workbench-actions strong {
  font-size: 12px;
  font-weight: 500;
}
.workbench-actions > a:hover {
  color: var(--color-primary);
}
.recent-action {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 0;
  border-bottom: 1px solid var(--color-border);
}
.recent-icon {
  height: 34px;
  width: 34px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.recent-action strong {
  font-size: 12px;
  font-weight: 500;
}
.recent-action p,
.recent-action time {
  font-size: 11px;
  color: var(--color-text-muted);
  margin-top: 3px;
}
.todo-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 0;
  border-bottom: 1px solid var(--color-border);
  font-size: 13px;
  color: var(--color-text);
}
.todo-row > span {
  display: flex;
  align-items: center;
  gap: 10px;
}
.todo-row .icon {
  color: var(--color-primary);
}
.workbench-notice {
  margin-top: 28px;
  font-size: 12px;
}
@media (max-width: 1100px) {
  .workbench-grid {
    grid-template-columns: 1fr;
  }
  .lifecycle-summary {
    max-width: 550px;
    margin: auto;
  }
}
@media (max-width: 767px) {
  .lifecycle-summary {
    gap: 20px;
    padding: 10px 0;
  }
  .donut {
    width: 135px;
    height: 135px;
    padding: 15px;
  }
  .legend-row {
    gap: 7px;
    font-size: 12px;
  }
  .legend-row > small {
    font-size: 10px;
  }
  .workbench-actions > a > strong {
    font-size: 11px;
  }
}
</style>
