<script setup lang="ts">
import { ref } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  webhook = ref(String(store.settings.webhook ?? '')),
  error = ref('')
function save() {
  error.value = ''
  if (webhook.value) {
    try {
      const url = new URL(webhook.value)
      if (url.protocol !== 'https:') throw new Error()
    } catch {
      error.value = '请输入有效的 HTTPS 回调地址。'
      return
    }
  }
  store.saveSettings({ webhook: webhook.value })
  ui.toast('回调地址草稿已保存，未向该地址发送请求。', 'info')
}
</script>
<template>
  <div class="page-stack">
    <div class="row-between">
      <h2>开放接口与系统集成</h2>
      <StatusBadge text="未接入" tone="neutral" />
    </div>
    <div class="notice-box warning">
      <AppIcon name="lock" />当前仅完成前端复刻。没有生成 API 密钥，没有开放真实接口，也没有执行 Webhook
      投递。
    </div>
    <section class="integration-card">
      <span><AppIcon name="key" :size="25" /></span>
      <div>
        <h3>API 凭据</h3>
        <p>凭据由服务端创建并限定租户、权限与有效期。管理端只展示脱敏信息。</p>
        <small class="muted">待接入凭据管理服务</small>
      </div>
    </section>
    <div class="form-section">
      <h3>Webhook 配置草稿</h3>
      <label class="field"
        ><span>回调地址</span
        ><input v-model="webhook" class="input" placeholder="https://example.com/webhook" type="url" /><small
          >保存草稿不会向该地址发送请求。正式接入需服务端完成地址验证与签名投递。</small
        ></label
      >
    </div>
    <div class="form-section">
      <h3>集成状态</h3>
      <div class="integration-status">
        <span>企业身份提供方（SSO）</span><StatusBadge text="未连接" tone="neutral" />
      </div>
      <div class="integration-status">
        <span>企业消息服务</span><StatusBadge text="未连接" tone="neutral" />
      </div>
      <div class="integration-status">
        <span>文件与下载服务</span><StatusBadge text="未连接" tone="neutral" />
      </div>
    </div>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <div class="form-footer"><button class="btn btn-primary" @click="save">保存回调草稿</button></div>
  </div>
</template>
<style scoped>
.integration-card {
  display: flex;
  gap: 18px;
  padding: 24px;
  background: var(--color-surface-soft);
  border-radius: 10px;
}
.integration-card > span {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  flex-shrink: 0;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  border-radius: 14px;
}
.integration-card p {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin: 8px 0 12px;
}
.integration-status {
  display: flex;
  justify-content: space-between;
  padding: 17px 0;
  border-bottom: 1px solid var(--color-border);
  font-size: 13px;
}
.form-footer {
  border: 0;
  margin-top: 0;
}
</style>
