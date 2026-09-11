<script setup lang="ts">
import type { Preferences } from "../model/preferences";
const model = defineModel<Preferences>({ required: true });
const events: {
  key: "workOrder" | "deviceAlert" | "memberChange" | "newsletter";
  title: string;
  description: string;
}[] = [
  {
    key: "workOrder",
    title: "工单通知",
    description: "工单分配、处理进展和服务完成提醒",
  },
  {
    key: "deviceAlert",
    title: "设备告警",
    description: "设备异常、长时间离线和关键运行状态提醒",
  },
  {
    key: "memberChange",
    title: "成员与权限变更",
    description: "成员邀请、角色调整与账号状态变更提醒",
  },
  {
    key: "newsletter",
    title: "产品更新",
    description: "平台功能更新与使用说明",
  },
];
</script>
<template>
  <section class="settings-section">
    <h3>业务通知</h3>
    <p>按业务类型设置企业默认通知偏好。</p>
    <label v-for="event in events" :key="event.key" class="toggle-setting"
      ><span
        ><strong>{{ event.title }}</strong
        ><small>{{ event.description }}</small></span
      ><input
        v-model="model[event.key]"
        type="checkbox"
        role="switch"
        :aria-label="event.title"
    /></label>
  </section>
  <section class="settings-section">
    <h3>通知渠道</h3>
    <p>正式通知依赖对应服务通道配置；本轮不会发送任何外部消息。</p>
    <label class="toggle-setting"
      ><span><strong>站内通知</strong><small>在系统通知中心集中查看业务动态</small></span
      ><input
        v-model="model.siteMessage"
        type="checkbox"
        role="switch"
        aria-label="站内通知" /></label
    ><label class="toggle-setting"
      ><span
        ><strong>邮件通知</strong><small>邮件通道尚未接入，仅保存演示配置</small></span
      ><input v-model="model.email" type="checkbox" role="switch" aria-label="邮件通知"
    /></label>
  </section>
</template>
