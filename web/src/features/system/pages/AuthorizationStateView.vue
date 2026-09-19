<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { UiButton } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import { redirectToTrustedLogin } from '@/services/runtime/authorization'

const route = useRoute()
const reason = computed(() => String(route.query.reason ?? 'forbidden'))
const unavailable = computed(() => reason.value === 'unavailable')
</script>

<template>
  <section class="authorization-state card panel-pad" data-authorization-state>
    <AppIcon :name="unavailable ? 'error' : 'lock'" :size="34" />
    <h1>{{ unavailable ? '授权信息暂时不可用' : '没有访问权限' }}</h1>
    <p v-if="unavailable">当前授权事实未能从服务端安全读取。系统不会回退到缓存权限或演示数据。</p>
    <p v-else>当前账号没有进入此页面的有效授权。菜单隐藏不能替代服务端 API 授权。</p>
    <div class="row">
      <UiButton v-if="reason === 'unauthenticated'" class="btn btn-primary" @click="redirectToTrustedLogin">重新登录</UiButton>
      <RouterLink v-else class="btn" to="/dashboard">返回工作台</RouterLink>
    </div>
  </section>
</template>

<style scoped>
.authorization-state{max-width:640px;margin:64px auto;display:flex;flex-direction:column;gap:16px;align-items:flex-start}
.authorization-state>.icon{color:var(--color-primary)}
.authorization-state h1{font-size:24px}
.authorization-state p{color:var(--color-text-secondary);line-height:1.7}
</style>
