<script setup lang="ts">
import AppIcon from './AppIcon.vue'
withDefaults(
  defineProps<{
    label: string
    value: string | number
    icon: string
    tone?: string
    caption?: string
    change?: string
    unit?: string
  }>(),
  { tone: 'blue', caption: '', change: '', unit: '' },
)
</script>
<template>
  <section class="card metric-card">
    <div class="metric-icon" :class="tone"><AppIcon :name="icon" :size="27" /></div>
    <div class="metric-body">
      <p>{{ label }}</p>
      <div class="metric-value numeric">
        {{ value }}<small v-if="unit">{{ unit }}</small
        ><span v-if="change" class="change">{{ change }}</span>
      </div>
      <div v-if="caption" class="metric-caption">{{ caption }}</div>
    </div>
  </section>
</template>
<style scoped>
.metric-card {
  padding: 22px 20px;
  display: flex;
  align-items: flex-start;
  gap: 16px;
  min-width: 0;
  min-height: 110px;
}
.metric-icon {
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  flex-shrink: 0;
}
.metric-icon.green {
  background: var(--color-success-soft);
  color: var(--color-success);
}
.metric-icon.orange {
  background: var(--color-warning-soft);
  color: var(--color-warning);
}
.metric-icon.purple {
  background: var(--color-violet-soft);
  color: var(--color-violet);
}
.metric-body {
  min-width: 0;
}
.metric-body p {
  font-size: var(--text-sm);
  font-weight: 500;
  margin-bottom: 2px;
}
.metric-value {
  font-size: var(--text-metric);
  font-weight: 650;
  line-height: 1.3;
  letter-spacing: -0.4px;
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}
.metric-value small {
  font-weight: 400;
  color: var(--color-text-secondary);
  font-size: 14px;
}
.change {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-success);
  letter-spacing: 0;
}
.metric-caption {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  margin-top: 5px;
}
@media (max-width: 1200px) {
  .metric-card {
    padding: 18px 14px;
    gap: 10px;
  }
  .metric-icon {
    width: 40px;
    height: 40px;
  }
  .metric-value {
    font-size: 26px;
  }
}
@media (max-width: 767px) {
  .metric-card {
    padding: 16px 12px;
  }
  .metric-icon {
    display: none;
  }
  .metric-value {
    font-size: 28px;
  }
  .change {
    font-size: 11px;
  }
}
@media (max-height: 830px) and (min-width: 768px) {
  .metric-card {
    min-height: 96px;
    padding: 14px 16px;
    gap: 12px;
  }
  .metric-icon {
    height: 40px;
    width: 40px;
  }
  .metric-value {
    font-size: 28px;
  }
  .metric-caption {
    margin-top: 3px;
  }
}
</style>
