<script setup lang="ts">
import { ref } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import AppIcon from '@/components/ui/AppIcon.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  draft = ref({ ...store.settings })
const items = [
  { key: 'alert', title: '设备告警', description: '关注离线、故障与重要设备事件', icon: 'warning' },
  { key: 'workorder', title: '工单通知', description: '工单分配、处理进度与完成结果', icon: 'ticket' },
  { key: 'member', title: '成员与权限变更', description: '邀请、禁用以及角色授权调整', icon: 'users' },
  { key: 'digest', title: '经营数据摘要', description: '接收周期性运营数据摘要', icon: 'chart' },
]
function save() {
  store.saveSettings(draft.value)
  ui.toast('通知偏好已保存在本地，未发送外部消息。')
}
</script>
<template>
  <div>
    <h2>通知渠道</h2>
    <p class="muted intro">按业务优先级选择通知类型，避免重复打扰。</p>
    <div class="channel-grid">
      <label class="channel-card"
        ><AppIcon name="bell" :size="26" />
        <div>
          <strong>站内通知</strong>
          <p>在通知中心接收消息</p>
        </div>
        <input v-model="draft.notificationInApp" type="checkbox" /></label
      ><label class="channel-card"
        ><AppIcon name="mail" :size="26" />
        <div>
          <strong>邮件通知</strong>
          <p>通过已验证邮箱接收</p>
        </div>
        <input v-model="draft.notificationEmail" type="checkbox"
      /></label>
    </div>
    <div class="form-section">
      <h3>通知内容</h3>
      <div v-for="item in items" :key="item.key" class="notification-row">
        <span class="notification-icon"><AppIcon :name="item.icon" :size="20" /></span>
        <div class="flex-1">
          <strong>{{ item.title }}</strong>
          <p>{{ item.description }}</p>
        </div>
        <button
          class="switch"
          role="switch"
          :aria-label="item.title"
          :aria-checked="Boolean(draft[item.key])"
          @click="draft[item.key] = !draft[item.key]"
        />
      </div>
    </div>
    <div class="notice-box">
      <AppIcon name="help" />通知订阅为界面预览，消息投递状态必须以实际服务端回执为准。
    </div>
    <div class="form-footer"><button class="btn btn-primary" @click="save">保存通知设置</button></div>
  </div>
</template>
<style scoped>
.intro {
  font-size: 12px;
  margin: 8px 0 24px;
}
.channel-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 26px;
}
.channel-card {
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 13px;
}
.channel-card > .icon {
  color: var(--color-primary);
}
.channel-card > div {
  flex: 1;
}
.channel-card strong {
  font-size: 13px;
}
.channel-card p,
.notification-row p {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 5px;
}
.notification-row {
  display: flex;
  gap: 14px;
  align-items: center;
  padding: 20px 0;
  border-bottom: 1px solid var(--color-border);
}
.notification-row:last-child {
  border-bottom: 0;
  margin-bottom: 12px;
}
.notification-row strong {
  font-size: 13px;
  font-weight: 500;
}
.notification-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  display: grid;
  place-items: center;
}
@media (max-width: 767px) {
  .channel-grid {
    grid-template-columns: 1fr;
  }
}
</style>
