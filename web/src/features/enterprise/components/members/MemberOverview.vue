<script setup lang="ts">
import { computed } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import AppIcon from '@/components/ui/AppIcon.vue'
const store = useEnterpriseStore()
const inUse = computed(() => store.members.filter((m) => m.status !== 'removed').length)
// Matches the preview plan's member allocation; this is not a production entitlement.
const quota = 500
const percentage = computed(() => Math.round((inUse.value / quota) * 100))
const metrics = computed(() => [
  { label: '成员总数', value: inUse.value, icon: 'users', caption: '当前企业成员', detail: '含待激活成员' },
  {
    label: '部门数',
    value: store.departments.filter((d) => d.enabled).length,
    icon: 'organization',
    caption: '组织架构',
    detail: '已启用部门',
  },
  {
    label: '角色数',
    value: store.roles.filter((r) => r.enabled).length,
    icon: 'shield',
    caption: '角色配置',
    detail: '内置及自定义',
  },
])
</script>
<template>
  <section class="member-overview" aria-label="成员管理统计">
    <article v-for="item in metrics" :key="item.label" class="card overview-card">
      <div class="overview-icon"><AppIcon :name="item.icon" :size="28" :stroke="2.3" /></div>
      <div class="overview-copy">
        <p>{{ item.label }}</p>
        <strong class="metric-value numeric">{{ item.value }}</strong>
        <small
          ><span>{{ item.caption }}</span
          ><span class="overview-note">{{ item.detail }}</span></small
        >
      </div>
    </article>
    <article class="card overview-card quota-card">
      <div class="quota-ring" :aria-label="`成员额度使用率 ${percentage}%`">
        <svg viewBox="0 0 72 72" aria-hidden="true">
          <circle class="ring-track" cx="36" cy="36" r="30" />
          <circle
            class="ring-progress"
            cx="36"
            cy="36"
            r="30"
            pathLength="100"
            :stroke-dasharray="`${Math.min(percentage, 100)} 100`"
          />
        </svg>
        <strong>{{ percentage }}%</strong>
      </div>
      <div class="overview-copy">
        <p>套餐使用率</p>
        <strong class="quota-value numeric"
          >{{ inUse }} <span>/ {{ quota }}</span></strong
        >
        <small>已使用 / 成员总额度</small>
      </div>
    </article>
  </section>
</template>
<style scoped>
.member-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.overview-card {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 114px;
  padding: 18px;
  border-radius: var(--radius-md);
}
.overview-icon {
  display: grid;
  place-items: center;
  width: 50px;
  height: 54px;
  flex-shrink: 0;
  color: var(--color-primary);
  background: var(--color-primary-soft);
  border-radius: 11px;
}
.overview-copy {
  min-width: 0;
}
.overview-copy p {
  font-size: var(--text-sm);
  font-weight: 500;
}
.overview-copy .metric-value {
  display: block;
  font-size: 28px;
  line-height: 1.45;
  letter-spacing: -0.5px;
  font-weight: 650;
}
.overview-copy small {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 8px;
  color: var(--color-text-muted);
  font-size: 11px;
}
.overview-note {
  color: var(--color-success);
}
.quota-card {
  gap: 16px;
}
.quota-ring {
  position: relative;
  width: 70px;
  height: 70px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
}
.quota-ring svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.quota-ring circle {
  fill: none;
  stroke-width: 7;
}
.ring-track {
  stroke: var(--color-success-soft);
}
.ring-progress {
  stroke: var(--color-success);
  stroke-linecap: round;
}
.quota-ring strong {
  color: var(--color-success);
  font-size: 14px;
}
.quota-value {
  display: block;
  margin: 9px 0 5px;
  font-size: 20px;
  white-space: nowrap;
}
.quota-value span {
  font-size: 16px;
  font-weight: 500;
}
@container member-main (max-width: 1020px) {
  .overview-card {
    gap: 12px;
    padding: 14px;
  }
  .overview-icon {
    width: 42px;
    height: 48px;
  }
  .quota-card {
    gap: 10px;
  }
  .quota-ring {
    width: 62px;
    height: 62px;
  }
  .overview-note {
    display: none;
  }
}
@container member-main (max-width: 790px) {
  .overview-card {
    gap: 8px;
    padding: 12px;
    min-height: 104px;
  }
  .overview-icon {
    width: 32px;
    height: 38px;
  }
  .overview-icon .icon {
    width: 22px;
    height: 22px;
  }
  .quota-ring {
    width: 48px;
    height: 48px;
  }
  .quota-value {
    font-size: 16px;
  }
  .quota-value span {
    font-size: 13px;
  }
}
@media (max-width: 767px) {
  .member-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .overview-card {
    min-height: 106px;
  }
}
</style>
