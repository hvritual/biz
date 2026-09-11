<script setup lang="ts">
import type { Preferences } from "../model/preferences";
const model = defineModel<Preferences>({ required: true });
</script>
<template>
  <section class="settings-section">
    <h3>登录与认证策略</h3>
    <p>下列选项为安全策略配置界面，不代表真实身份服务已经启用这些能力。</p>
    <label class="toggle-setting"
      ><span
        ><strong>多因素认证</strong><small>企业成员登录时增加一次身份校验</small></span
      ><input
        v-model="model.mfa"
        type="checkbox"
        role="switch"
        aria-label="多因素认证" /></label
    ><label class="toggle-setting"
      ><span
        ><strong>登录失败锁定</strong
        ><small>连续失败触发保护，具体阈值由身份服务统一管理</small></span
      ><input
        v-model="model.loginLock"
        type="checkbox"
        role="switch"
        aria-label="登录失败锁定" /></label
    ><label class="toggle-setting"
      ><span
        ><strong>企业单点登录</strong
        ><small>接入企业身份提供方后，由服务端验证登录声明</small></span
      ><input v-model="model.sso" type="checkbox" role="switch" aria-label="企业单点登录"
    /></label>
  </section>
  <section class="settings-section">
    <h3>会话与凭据</h3>
    <p>账号禁用、密码重置和权限回收，应通过服务端使相关授权及时失效。</p>
    <label class="field" style="max-width: 320px"
      ><span>空闲会话超时</span
      ><select v-model="model.sessionMinutes">
        <option value="15">15 分钟</option>
        <option value="30">30 分钟</option>
        <option value="60">60 分钟</option>
      </select></label
    >
    <div class="info-banner">
      本地演示不会修改真实密码、签发会话令牌或强制用户退出。接入身份服务后需另行进行安全验证。
    </div>
  </section>
</template>
