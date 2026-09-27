<script setup lang="ts">
defineOptions({ name: 'UiTimeline' })
export type TimelineItem = {
  title: string
  description?: string
  time?: string
  tone?: 'default' | 'primary' | 'success' | 'warning' | 'danger'
}

withDefaults(
  defineProps<{
    items: TimelineItem[]
  }>(),
  {},
)
</script>
<template>
  <ol class="timeline" data-ui-pattern="Timeline">
    <li
      v-for="(item, index) in items"
      :key="index"
      class="timeline-item"
      :data-tone="item.tone || 'default'"
    >
      <span class="timeline-dot" aria-hidden="true" />
      <div class="timeline-content">
        <div class="timeline-header">
          <strong class="timeline-title">{{ item.title }}</strong>
          <small v-if="item.time" class="timeline-time">{{ item.time }}</small>
        </div>
        <p v-if="item.description" class="timeline-desc">{{ item.description }}</p>
      </div>
    </li>
  </ol>
</template>
<style scoped>
.timeline {
  list-style: none;
  margin: 0;
  padding: 0;
  border-left: 1px solid var(--color-border);
  padding-left: var(--space-5);
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}
.timeline-item {
  position: relative;
}
.timeline-dot {
  position: absolute;
  left: calc(-1 * var(--space-5) - 5px);
  top: 4px;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--color-primary);
  border: 2px solid var(--color-surface);
}
.timeline-item[data-tone='default'] .timeline-dot { background: var(--color-text-muted); }
.timeline-item[data-tone='primary'] .timeline-dot { background: var(--color-primary); }
.timeline-item[data-tone='success'] .timeline-dot { background: var(--color-success); }
.timeline-item[data-tone='warning'] .timeline-dot { background: var(--color-warning); }
.timeline-item[data-tone='danger'] .timeline-dot { background: var(--color-danger); }
.timeline-content {
  min-width: 0;
}
.timeline-header {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  flex-wrap: wrap;
}
.timeline-title {
  font-size: var(--text-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}
.timeline-time {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}
.timeline-desc {
  margin-top: var(--space-1);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: var(--leading-relaxed);
}
</style>
