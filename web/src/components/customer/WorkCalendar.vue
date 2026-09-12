<script setup lang="ts">
import { computed, ref } from 'vue'
import type { WorkItem } from '@/types/customer'
import { useCustomerActions } from '@/composables/customerActions'
import AppIcon from '@/components/ui/AppIcon.vue'
const props = defineProps<{ items: WorkItem[] }>()
const actions = useCustomerActions(),
  offset = ref(0)
const days = computed(() =>
  Array.from({ length: 7 }, (_, i) => {
    const d = new Date(Date.UTC(2026, 8, 7 + offset.value * 7 + i))
    return {
      key: d.toISOString().slice(0, 10),
      label: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'][i],
      day: d.getUTCDate(),
    }
  }),
)
const scheduled = computed(() => props.items.filter((w) => w.status !== '已结束'))
const hours = ['09:00', '10:00', '11:00', '14:00', '16:00']
function inCell(day: string, hour: string) {
  return scheduled.value.filter(
    (w) =>
      w.nextAt.slice(0, 10) === day &&
      (w.nextAt.slice(11, 13) === hour.slice(0, 2) ||
        (!hours.some((h) => h.slice(0, 2) === w.nextAt.slice(11, 13)) && hour === '16:00')),
  )
}
</script>
<template>
  <section class="card">
    <div class="customer-section-header">
      <h2>{{ days[0]?.key }} — {{ days[6]?.key }}</h2>
      <div class="row">
        <button class="btn" aria-label="上一周" @click="offset--"><AppIcon name="left" :size="15" /></button
        ><button class="btn" @click="offset = 0">本周</button
        ><button class="btn" aria-label="下一周" @click="offset++">
          <AppIcon name="right" :size="15" />
        </button>
      </div>
    </div>
    <div class="calendar-scroll">
      <div class="calendar-grid">
        <div class="calendar-heading">时间</div>
        <div v-for="day in days" :key="day.key" class="calendar-heading">{{ day.label }} {{ day.day }}</div>
        <template v-for="hour in hours" :key="hour"
          ><div class="calendar-time">{{ hour }}</div>
          <div v-for="day in days" :key="`${day.key}${hour}`" class="calendar-cell">
            <button
              v-for="work in inCell(day.key, hour)"
              :key="work.id"
              class="calendar-event"
              @click="actions.open('reschedule', work.id)"
            >
              <strong>{{ work.id }} · {{ work.owner }}</strong
              ><span>{{ work.nextAction }}</span>
            </button>
          </div></template
        >
      </div>
    </div>
    <p class="customer-help" style="padding: 16px 20px">
      按下一次行动时间落格；点击日程可以改期。事项截止时间与服务时钟不会被自动改写。
    </p>
  </section>
</template>
<style scoped>
.calendar-scroll {
  overflow: auto;
}
.calendar-grid {
  display: grid;
  grid-template-columns: 70px repeat(7, minmax(130px, 1fr));
  min-width: 1040px;
}
.calendar-heading {
  padding: 15px 12px;
  font-size: 12px;
  text-align: center;
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  border: 1px solid var(--color-border);
  border-left: 0;
}
.calendar-time {
  font-size: 12px;
  padding: 14px 12px;
  color: var(--color-text-muted);
  border-right: 1px solid var(--color-border);
  border-bottom: 1px solid var(--color-border);
}
.calendar-cell {
  padding: 10px;
  min-height: 100px;
  border-right: 1px solid var(--color-border);
  border-bottom: 1px solid var(--color-border);
}
.calendar-event {
  display: block;
  padding: 9px 10px;
  border-left: 3px solid var(--color-primary);
  border-radius: 5px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: 12px;
  line-height: 1.7;
  text-align: left;
  width: 100%;
  margin-bottom: 7px;
}
.calendar-event span {
  display: block;
  color: var(--color-text-secondary);
}
</style>
