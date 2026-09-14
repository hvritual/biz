<script setup lang="ts">
import type { WorkItem, WorkStatus } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workKindNames } from '@/services/customer/seed'
import AppIcon from '@/components/ui/AppIcon.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
defineProps<{ items: WorkItem[] }>()
const store = useCustomerStore(),
  actions = useCustomerActions()
const statuses: WorkStatus[] = ['待开始', '处理中', '等待客户', '待验收', '已结束']
function drop(e: DragEvent, status: WorkStatus) {
  e.preventDefault()
  const id = e.dataTransfer?.getData('text/plain')
  if (!id) return
  actions.open(status === '已结束' ? 'accept' : 'transition', id, [], { status })
}
</script>
<template>
  <div class="board-viewport">
    <div class="work-board">
      <section
        v-for="status in statuses"
        :key="status"
        class="work-column"
        @dragover.prevent
        @drop="drop($event, status)"
      >
        <header>
          <span
            class="column-dot"
            :class="status === '已结束' ? 'green' : status === '待验收' ? 'orange' : ''"
          />
          <h2>{{ status }}</h2>
          <span class="count">{{ items.filter((w) => w.status === status).length }}</span>
        </header>
        <article
          v-for="w in items.filter((w) => w.status === status)"
          :key="w.id"
          class="work-card"
          :draggable="w.writable && w.status !== '已结束'"
          @dragstart="($event as DragEvent).dataTransfer?.setData('text/plain', w.id)"
        >
          <div class="row-between">
            <small>{{ w.id }}</small
            ><button
              class="icon-button"
              :aria-label="`流转 ${w.id}`"
              :disabled="!w.writable"
              @click="actions.open(w.status === '已结束' ? 'reopen' : 'transition', w.id)"
            >
              <AppIcon name="more" :size="17" />
            </button>
          </div>
          <RouterLink :to="`/customers/work/${w.id}`" class="work-title">{{ w.title }}</RouterLink>
          <p class="work-client">{{ store.customerName(w.customerId) }}</p>
          <div class="row wrap">
            <StatusBadge :text="workKindNames[w.kind]" tone="primary" :dot="false" /><StatusBadge
              v-if="
                w.dependencies.some((id) => store.snapshot.work.find((x) => x.id === id)?.status !== '已结束')
              "
              text="存在阻塞"
              tone="warning"
            />
          </div>
          <div class="work-card-meta">
            <span class="row"><AvatarMark :name="w.owner" :size="23" />{{ w.owner }}</span
            ><span class="row"><AppIcon name="calendar" :size="14" />{{ w.deadline.slice(5) }}</span>
          </div>
          <p class="work-next">
            {{ w.status === '已结束' ? `结果：${w.resolution}` : `下一步：${w.nextAction}` }}
          </p>
        </article>
        <p v-if="!items.some((w) => w.status === status)" class="column-empty">暂无事项</p>
      </section>
    </div>
  </div>
</template>
<style scoped>
.board-viewport {
  overflow-x: auto;
  min-width: 0;
}
.work-board {
  display: grid;
  grid-template-columns: repeat(5, minmax(205px, 1fr));
  gap: 14px;
  min-width: 1070px;
  align-items: start;
}
.work-column {
  border-radius: 10px;
  background: var(--color-surface-soft);
  border: 1px solid var(--color-border);
  padding: 12px 10px;
  min-height: 420px;
}
.work-column > header {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 0 4px 12px;
}
.work-column h2 {
  font-size: 14px;
  font-weight: 600;
}
.column-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary);
}
.column-dot.green {
  background: var(--color-success);
}
.column-dot.orange {
  background: var(--color-warning);
}
.count {
  color: var(--color-text-secondary);
  font-size: 13px;
  margin-left: auto;
}
.work-card {
  padding: 12px 13px;
  margin-bottom: 12px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  box-shadow: 0 2px 5px var(--color-shadow);
}
.work-card:hover {
  border-color: var(--color-primary);
}
.work-card small {
  font-size: 12px;
  color: var(--color-text-muted);
}
.work-card .icon-button {
  width: 22px;
  height: 22px;
}
.work-title {
  display: block;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.65;
  margin: 6px 0;
  color: var(--color-text);
}
.work-client {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-bottom: 12px;
}
.work-card-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 12px;
  margin: 14px 0 10px;
  color: var(--color-text-secondary);
}
.work-next {
  font-size: 12px;
  line-height: 1.75;
  color: var(--color-text-secondary);
  padding-top: 9px;
  border-top: 1px solid var(--color-border);
}
.column-empty {
  text-align: center;
  color: var(--color-text-muted);
  font-size: 12px;
  margin-top: 40px;
}
</style>
