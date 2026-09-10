<script setup lang="ts">
import EmptyState from '@/components/ui/EmptyState.vue'
</script>
<template>
  <div class="card panel-pad">
    <EmptyState title="页面不存在" description="请从导航进入企业中心，或检查访问地址。"
      ><RouterLink class="btn btn-primary" to="/enterprise/members">返回成员管理</RouterLink></EmptyState
    >
  </div>
</template>
