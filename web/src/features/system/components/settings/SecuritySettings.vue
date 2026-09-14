<script setup lang="ts">
import { ref } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import AppIcon from '@/components/ui/AppIcon.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  draft = ref({ ...store.settings }),
  error = ref('')
function save() {
  error.value = ''
  const minutes = Number(draft.value.sessionMinutes),
    attempts = Number(draft.value.attempts)
  if (!Number.isInteger(minutes) || minutes < 5 || minutes > 480) {
    error.value = '会话时长必须为 5–480 分钟的整数。'
    return
  }
  if (!Number.isInteger(attempts) || attempts < 3 || attempts > 10) {
    error.value = '失败次数必须为 3–10 次的整数。'
    return
  }
  store.saveSettings(draft.value)
  ui.toast('安全策略草稿已保存；尚未应用到身份服务。', 'info')
}
</script>
<template>
  <div class="page-stack">
    <h2>登录与认证策略</h2>
    <div class="notice-box">
      <AppIcon
        name="shield"
      />此处编辑企业安全策略预览，不代表策略已在真实登录链路生效。身份认证、会话撤销和二次验证必须由后端执行。
    </div>
    <section class="security-setting">
      <div class="row-between">
        <div>
          <h3>要求多因素认证</h3>
          <p>敏感角色登录时增加独立验证因素。</p>
        </div>
        <button
          class="switch"
          role="switch"
          aria-label="要求多因素认证"
          :aria-checked="Boolean(draft.mfa)"
          @click="draft.mfa = !draft.mfa"
        />
      </div>
    </section>
    <section class="security-setting">
      <div class="row-between">
        <div>
          <h3>登录失败保护</h3>
          <p>连续认证失败时触发限速或临时锁定。</p>
        </div>
        <button
          class="switch"
          role="switch"
          aria-label="登录失败保护"
          :aria-checked="Boolean(draft.loginLock)"
          @click="draft.loginLock = !draft.loginLock"
        />
      </div>
      <label class="field inline-field"
        ><span>连续失败次数</span
        ><input
          v-model.number="draft.attempts"
          class="input"
          type="number"
          min="3"
          max="10"
          :disabled="!draft.loginLock"
        /><small>次</small></label
      >
    </section>
    <section class="security-setting">
      <h3>会话管理</h3>
      <p>会话超时后重新认证；权限收回不应等待会话自然过期。</p>
      <label class="field inline-field"
        ><span>空闲会话时长</span
        ><input v-model.number="draft.sessionMinutes" aria-label="空闲会话时长" class="input" type="number" min="5" max="480" /><small
          >分钟</small
        ></label
      >
    </section>
    <section class="security-setting">
      <h3>密码重置</h3>
      <p>仅发送单次、限时的重置链接；不向管理员展示明文密码，不将密码写入日志。</p>
      <RouterLink class="btn-link security-link" to="/enterprise/members"
        >前往成员账户安全<AppIcon name="right" :size="15"
      /></RouterLink>
    </section>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <div class="form-footer">
      <button class="btn" @click="draft = { ...store.settings }">取消修改</button
      ><button class="btn btn-primary" @click="save">保存策略草稿</button>
    </div>
  </div>
</template>
<style scoped>
.security-setting {
  padding: 20px 0;
  border-bottom: 1px solid var(--color-border);
}
.security-setting p {
  color: var(--color-text-muted);
  font-size: 12px;
  margin-top: 7px;
  max-width: 590px;
}
.inline-field {
  display: flex;
  flex-direction: row;
  align-items: center;
  margin-top: 18px;
  gap: 14px;
}
.inline-field .input {
  width: 100px;
}
.security-link {
  margin-top: 15px;
}
.form-footer {
  margin-top: 0;
  padding-top: 8px;
  border: 0;
}
</style>
