<script setup lang="ts">
import { ref } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import AppIcon from '@/components/ui/AppIcon.vue'
import brand from '@/assets/brand-mark.png'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  draft = ref({ ...store.settings })
function save() {
  if (!String(draft.value.platformName).trim()) {
    ui.toast('请填写平台显示名称。', 'error')
    return
  }
  store.saveSettings(draft.value)
  ui.toast('基础设置已保存到本地预览。')
}
</script>
<template>
  <form @submit.prevent="save">
    <div class="row-between block-title">
      <div>
        <h2>平台基础信息</h2>
        <p class="settings-caption">设置当前企业的界面名称、语言与时区偏好。</p>
      </div>
      <img class="settings-brand" :src="brand" alt="CoffeeLink" />
    </div>
    <div class="form-grid">
      <label class="field full-width"
        ><span class="required">平台显示名称</span
        ><input v-model="draft.platformName" class="input" maxlength="60" required /></label
      ><label class="field"
        ><span>默认语言</span
        ><select v-model="draft.language" class="select">
          <option>简体中文</option></select
        ><small>当前交付仅包含简体中文界面。</small></label
      ><label class="field"
        ><span>默认时区</span
        ><select v-model="draft.timezone" class="select">
          <option value="Asia/Shanghai">中国标准时间 · UTC+08:00</option>
          <option value="UTC">协调世界时 · UTC</option>
          <option value="Europe/Berlin">欧洲柏林时间</option>
        </select></label
      >
    </div>
    <section class="form-section">
      <h3>视觉与布局</h3>
      <div class="settings-description">
        <div>
          <strong>品牌主题</strong>
          <p>沿用 CoffeeLink 蓝白主题与统一设计变量。</p>
        </div>
        <span class="theme-dot" />
      </div>
      <div class="settings-description">
        <div>
          <strong>悬浮模块导航</strong>
          <p>480px 模块抽屉，子菜单与快捷操作左右并排，不挤压业务区。</p>
        </div>
        <AppIcon name="check" />
      </div>
      <div class="settings-description">
        <div>
          <strong>列表密度</strong>
          <p>统一的表格行高与间距，不单独缩小文字。</p>
        </div>
        <span class="pill">标准</span>
      </div>
    </section>
    <div class="form-footer">
      <button class="btn" type="button" @click="draft = { ...store.settings }">取消修改</button
      ><button class="btn btn-primary" type="submit">保存设置</button>
    </div>
  </form>
</template>
<style scoped>
.settings-caption {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 7px;
}
.settings-brand {
  width: 48px;
  height: 52px;
  object-fit: contain;
}
.settings-description {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 0;
  border-bottom: 1px solid var(--color-border);
}
.settings-description strong {
  font-size: 13px;
  font-weight: 500;
}
.settings-description p {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 5px;
}
.settings-description .icon {
  color: var(--color-success);
}
.theme-dot {
  height: 26px;
  width: 26px;
  border: 4px solid var(--color-primary-soft);
  border-radius: 50%;
  background: var(--color-primary);
  flex-shrink: 0;
}
</style>
