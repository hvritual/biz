<script setup lang="ts">
import type { Preferences } from "../model/preferences";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
const model = defineModel<Preferences>({ required: true });
</script>
<template>
  <section class="settings-section">
    <h3>开放接口 <StatusBadge tone="gray">未接入</StatusBadge></h3>
    <p>凭据由服务端签发和保管，界面不展示虚构可用密钥，也不存储明文密码。</p>
    <div class="field">
      <span>API 访问凭据</span>
      <div class="read-only-value">尚未配置真实 API 服务</div>
      <small>联调完成前，成员操作仅作用于内存中的演示数据</small>
    </div>
  </section>
  <section class="settings-section">
    <h3>Webhook 回调</h3>
    <p>设备状态、工单进展等事件的回调配置入口。</p>
    <label class="toggle-setting"
      ><span
        ><strong>启用事件回调</strong
        ><small>当前仅校验和保存演示地址，不发出网络请求</small></span
      ><input
        v-model="model.webhookEnabled"
        type="checkbox"
        role="switch"
        aria-label="启用事件回调" /></label
    ><label class="field"
      ><span>回调地址</span
      ><input
        v-model="model.webhook"
        type="url"
        placeholder="https://example.com/webhook"
        :disabled="!model.webhookEnabled"
      /><small>正式环境需要配置 HTTPS、签名校验、重试和执行回执</small></label
    >
  </section>
</template>
