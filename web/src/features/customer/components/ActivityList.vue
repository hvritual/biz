<script setup lang="ts">
import type { Activity } from '@/types/customer'
import StatusBadge from '@/components/ui/StatusBadge.vue'
defineProps<{ items: Activity[] }>()
</script>
<template>
  <div v-if="items.length">
    <div v-for="item in items" :key="item.id" class="customer-activity">
      <span class="activity-dot" />
      <div>
        <small>{{ item.time }}　{{ item.actor }}</small>
        <p>
          <strong>{{ item.action }}</strong>
          <StatusBadge
            :text="item.visibility === 'shared' ? '客户可见' : '内部'"
            :tone="item.visibility === 'shared' ? 'primary' : 'neutral'"
            :dot="false"
          />
        </p>
        <p>{{ item.detail }}</p>
      </div>
    </div>
  </div>
  <p v-else class="customer-help">尚无相关活动记录。完成操作后会在此保留时间、责任人与变更依据。</p>
</template>
