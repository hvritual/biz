<script setup lang="ts">
import { ref } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import { changeOwnPassword, PasswordChangeError } from '@/services/runtime/accountSecurity'
import { UiButton, UiInput } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'

const store = useEnterpriseStore()
const ui = useUiStore()
const draft = ref({ ...store.settings })
const error = ref('')

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const showCurrent = ref(false)
const showNew = ref(false)
const showConfirm = ref(false)
const changingPassword = ref(false)
const passwordError = ref('')
const passwordChanged = ref(false)

function save() {
  error.value = ''
  const minutes = Number(draft.value.sessionMinutes)
  const attempts = Number(draft.value.attempts)
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

function validNewPassword(value: string) {
  return value.length >= 8 && value.length <= 16 && /[A-Z]/.test(value) && /\d/.test(value)
}

async function submitPasswordChange() {
  passwordError.value = ''
  passwordChanged.value = false
  if (!validNewPassword(newPassword.value)) {
    passwordError.value = '新密码需为 8–16 位，并至少包含一个大写字母和一个数字。'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = '两次输入的新密码不一致。'
    return
  }
  changingPassword.value = true
  try {
    await changeOwnPassword({
      currentPassword: currentPassword.value,
      newPassword: newPassword.value,
      confirmPassword: confirmPassword.value,
    })
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    passwordChanged.value = true
    ui.toast('密码已修改，所有旧会话已失效，请重新登录。', 'success')
  } catch (cause) {
    if (cause instanceof PasswordChangeError) {
      switch (cause.code) {
        case 'CURRENT_PASSWORD_INVALID':
          passwordError.value = '当前密码错误。'
          break
        case 'WEAK_PASSWORD':
          passwordError.value = '新密码不符合安全规则。'
          break
        case 'PASSWORD_MISMATCH':
          passwordError.value = '两次输入的新密码不一致。'
          break
        case 'UNAUTHENTICATED':
          passwordError.value = '当前会话已失效，请重新登录。'
          break
        default:
          passwordError.value = '密码修改服务暂不可用。'
      }
    } else {
      passwordError.value = '密码修改服务暂不可用。'
    }
  } finally {
    changingPassword.value = false
  }
}
</script>

<template>
  <div class="page-stack">
    <h2>登录与认证策略</h2>
    <div class="notice-box">
      <AppIcon name="shield" />此处企业策略仍是预览；账号密码修改属于本人全局 Account 安全动作，由后端身份服务执行。
    </div>

    <section class="security-setting">
      <h3>修改我的密码</h3>
      <p>验证当前密码后更新全局账号凭据。成功后包括当前标签页在内的全部旧会话立即失效。</p>
      <div v-if="!passwordChanged" class="password-form">
        <label class="field">
          <span>当前密码</span>
          <div class="password-row">
            <UiInput v-model="currentPassword" aria-label="当前密码" :type="showCurrent ? 'text' : 'password'" autocomplete="current-password" />
            <UiButton class="btn" type="button" @click="showCurrent = !showCurrent">{{ showCurrent ? '隐藏' : '显示' }}</UiButton>
          </div>
        </label>
        <label class="field">
          <span>新密码</span>
          <div class="password-row">
            <UiInput v-model="newPassword" aria-label="新密码" :type="showNew ? 'text' : 'password'" autocomplete="new-password" maxlength="16" />
            <UiButton class="btn" type="button" @click="showNew = !showNew">{{ showNew ? '隐藏' : '显示' }}</UiButton>
          </div>
        </label>
        <label class="field">
          <span>确认新密码</span>
          <div class="password-row">
            <UiInput v-model="confirmPassword" aria-label="确认新密码" :type="showConfirm ? 'text' : 'password'" autocomplete="new-password" maxlength="16" />
            <UiButton class="btn" type="button" @click="showConfirm = !showConfirm">{{ showConfirm ? '隐藏' : '显示' }}</UiButton>
          </div>
        </label>
        <small>8–16 位，至少包含一个大写字母和一个数字。</small>
        <p v-if="passwordError" class="form-error" role="alert">{{ passwordError }}</p>
        <UiButton class="btn btn-primary security-link" :disabled="changingPassword" @click="submitPasswordChange">
          {{ changingPassword ? '修改中…' : '修改密码' }}
        </UiButton>
      </div>
      <div v-else class="notice-box success-box" role="status">
        密码已修改，旧会话已全部撤销。
        <a class="btn-link security-link" href="/auth/login?return_to=/">重新登录</a>
      </div>
    </section>

    <section class="security-setting">
      <div class="row-between">
        <div>
          <h3>要求多因素认证</h3>
          <p>敏感角色登录时增加独立验证因素。</p>
        </div>
        <UiButton class="switch" role="switch" aria-label="要求多因素认证" :aria-checked="Boolean(draft.mfa)" @click="draft.mfa = !draft.mfa" />
      </div>
    </section>

    <section class="security-setting">
      <div class="row-between">
        <div>
          <h3>登录失败保护</h3>
          <p>连续认证失败时触发限速或临时锁定。</p>
        </div>
        <UiButton class="switch" role="switch" aria-label="登录失败保护" :aria-checked="Boolean(draft.loginLock)" @click="draft.loginLock = !draft.loginLock" />
      </div>
      <label class="field inline-field">
        <span>连续失败次数</span>
        <UiInput v-model.number="draft.attempts" class="input" type="number" min="3" max="10" :disabled="!draft.loginLock" /><small>次</small>
      </label>
    </section>

    <section class="security-setting">
      <h3>会话管理</h3>
      <p>会话超时后重新认证；权限收回不应等待会话自然过期。</p>
      <label class="field inline-field">
        <span>空闲会话时长</span>
        <UiInput v-model.number="draft.sessionMinutes" aria-label="空闲会话时长" class="input" type="number" min="5" max="480" /><small>分钟</small>
      </label>
    </section>

    <section class="security-setting">
      <h3>管理员密码恢复</h3>
      <p>租户管理员不能直接修改跨企业共享 Account 的全局密码。Q-007 未批准前，只允许 Account 本人通过登录页“忘记密码”完成身份自证。</p>
      <RouterLink class="btn-link security-link" to="/enterprise/members">前往成员账户安全<AppIcon name="right" :size="15" /></RouterLink>
    </section>

    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <div class="form-footer">
      <UiButton class="btn" @click="draft = { ...store.settings }">取消修改</UiButton>
      <UiButton class="btn btn-primary" @click="save">保存策略草稿</UiButton>
    </div>
  </div>
</template>

<style scoped>
.security-setting { padding: 20px 0; border-bottom: 1px solid var(--color-border); }
.security-setting p { color: var(--color-text-muted); font-size: 12px; margin-top: 7px; max-width: 640px; }
.inline-field { display: flex; flex-direction: row; align-items: center; margin-top: 18px; gap: 14px; }
.inline-field .input { width: 100px; }
.security-link { margin-top: 15px; }
.form-footer { margin-top: 0; padding-top: 8px; border: 0; }
.password-form { max-width: 560px; margin-top: 16px; }
.password-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: center; }
.password-row .btn { min-width: 68px; }
.success-box { margin-top: 16px; }
@media (max-width: 767px) {
  .password-row { grid-template-columns: 1fr; }
}
</style>
