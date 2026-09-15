<script setup lang="ts">
withDefaults(defineProps<{ id?: string; active: string }>(), { id: 'CUS-0186' })
</script>
<template>
  <nav class="customer-tabs" aria-label="客户工作区标签">
    <RouterLink :class="{ active: active === 'overview' }" :to="`/customers/accounts/${id}`">概览</RouterLink
    ><RouterLink :class="{ active: active === 'work' }" :to="`/customers/work?customer=${id}`"
      >事项</RouterLink
    ><RouterLink :class="{ active: active === 'plans' }" :to="`/customers/plans?customer=${id}`"
      >经营计划</RouterLink
    ><RouterLink :class="{ active: active === 'contracts' }" :to="`/customers/contracts?customer=${id}`"
      >合同与续约</RouterLink
    ><RouterLink :class="{ active: active === 'sites' }" :to="`/customers/accounts/${id}?tab=sites`"
      >点位与投放</RouterLink
    ><RouterLink :class="{ active: active === 'contacts' }" :to="`/customers/contacts?customer=${id}`"
      >联系人与协作</RouterLink
    ><RouterLink :class="{ active: active === 'activity' }" :to="`/customers/accounts/${id}?tab=activity`"
      >活动记录</RouterLink
    >
  </nav>
</template>
