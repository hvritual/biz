<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { useUiStore } from '@/stores/ui'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  ui = useUiStore(),
  filter = ref('全部'),
  selected = ref('NT-01')
const notices = computed(() => store.snapshot.notifications.filter((n) => filter.value !== '未读' || !n.read))
const current = computed(() => store.snapshot.notifications.find((n) => n.id === selected.value))
function read(id: string) {
  try {
    const result = store.run({ action: 'read-notice', id, values: {}, key: crypto.randomUUID() })
    ui.toast(result.detail)
  } catch (e) {
    ui.toast((e as Error).message, 'error')
  }
}
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="通知与待决策"
      breadcrumb="客户经营"
      description="让待办可见，同时避免把“已读”误当作“工作完成”"
    />
    <nav class="customer-tabs">
      <button
        v-for="name in ['全部', '未读']"
        :key="name"
        :class="{ active: filter === name }"
        @click="filter = name"
      >
        {{ name
        }}<span v-if="name === '未读'"
          >（{{ store.snapshot.notifications.filter((n) => !n.read).length }}）</span
        >
      </button>
    </nav>
    <div class="customer-split">
      <CustomerSection title="需要关注的工作" icon="bell"
        ><div v-for="notice in notices" :key="notice.id" class="customer-record">
          <div>
            <button class="btn-link mainline" @click="selected = notice.id">{{ notice.title }}</button>
            <p>{{ notice.description }}</p>
          </div>
          <div class="customer-checklist">
            <StatusBadge
              :text="notice.read ? '已读' : '未读'"
              :tone="notice.read ? 'neutral' : 'primary'"
            /><button class="btn-link" :disabled="notice.read" @click="read(notice.id)">标记已读</button>
          </div>
        </div>
        <p v-if="!notices.length" class="customer-help">当前没有未读通知。</p></CustomerSection
      ><CustomerSection v-if="current" title="事项与决策入口" icon="checks"
        ><h3 style="font-size: 16px; margin-bottom: 12px">{{ current.title }}</h3>
        <p style="line-height: 1.9">{{ current.description }}</p>
        <dl class="customer-info-list" style="margin: 22px 0">
          <dt>类别</dt>
          <dd>{{ current.category }}</dd>
          <dt>关联事项</dt>
          <dd>{{ current.workId }}</dd>
          <dt>当前状态</dt>
          <dd>{{ store.snapshot.work.find((w) => w.id === current?.workId)?.status }}</dd>
          <dt>通知状态</dt>
          <dd>{{ current.read ? '已读' : '未读' }}</dd>
        </dl>
        <RouterLink :to="`/customers/work/${current.workId}`" class="btn btn-primary"
          >进入事项处理</RouterLink
        ></CustomerSection
      >
    </div>
    <CustomerAlert
      title="通知已读只改变通知状态"
      description="事项仍需由责任人处理、补齐证据并完成验收；消息不会自动替用户作出经营决策。"
    />
  </div>
</template>
