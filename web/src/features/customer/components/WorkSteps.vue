<script setup lang="ts">
import type { WorkItem } from '@/types/customer'
import { workflows } from '@/services/customer/seed'
import AppIcon from '@/components/ui/AppIcon.vue'
defineProps<{ work: WorkItem }>()
</script>
<template>
  <div
    class="card customer-flow"
    :class="{ 'resolved-unsuccessful': work.status === '已结束' && work.resolution !== '成功' }"
    aria-label="事项工作流"
  >
    <div
      v-for="(label, index) in workflows[work.kind]"
      :key="label"
      class="customer-flow-step"
      :class="{
        done: index < work.stage || work.status === '已结束',
        current: index === work.stage && work.status !== '已结束',
      }"
    >
      <span
        ><AppIcon v-if="index < work.stage || work.status === '已结束'" name="check" :size="15" /><template
          v-else
          >{{ index + 1 }}</template
        ></span
      >{{ label }}
    </div>
  </div>
</template>
