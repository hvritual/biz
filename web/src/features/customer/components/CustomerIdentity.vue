<script setup lang="ts">
import type { Customer } from '@/types/customer'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
defineProps<{ customer: Customer }>()
</script>
<template>
  <section class="card customer-identity">
    <div class="customer-identity-mark"><AppIcon name="company" :size="32" /></div>
    <div class="flex-1">
      <div class="customer-identity-title">
        <h2>{{ customer.name }}</h2>
        <StatusBadge
          :text="customer.lifecycle"
          :tone="customer.lifecycle === '合作中' ? 'success' : 'neutral'"
        /><StatusBadge v-if="customer.archived" text="已归档" tone="neutral" />
      </div>
      <div class="customer-identity-details">
        <span>{{ customer.id }} · {{ customer.category }}</span
        ><span>{{ customer.area }}</span
        ><span class="row"
          ><AvatarMark :name="customer.owner" :size="22" /> <b>{{ customer.owner }}</b> · 客户负责人</span
        >
      </div>
    </div>
    <div class="identity-summary">
      <p>下次行动</p>
      <strong>{{ customer.nextContact }}</strong>
      <p style="margin-top: 7px">有效点位 / 关联设备　{{ customer.sites }} / {{ customer.devices }}</p>
    </div>
  </section>
</template>
