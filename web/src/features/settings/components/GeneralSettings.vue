<script setup lang="ts">
import type { Preferences } from "../model/preferences";
import BrandLogo from "@/shared/ui/BrandLogo.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
const model = defineModel<Preferences>({ required: true });
</script>
<template>
  <section class="settings-section">
    <h3>平台基础信息</h3>
    <p>设置系统展示名称、简介与默认偏好，统一企业使用体验。</p>
    <div class="form-grid">
      <label class="field"
        ><span>平台名称 <em>*</em></span
        ><input v-model="model.name" maxlength="60" required
      /></label>
      <div class="field">
        <span>平台标识</span>
        <div class="company-logo-box"><BrandLogo /></div>
        <small>当前采用 CoffeeLink 设计标识</small>
      </div>
      <label class="field"
        ><span>平台简称</span><input v-model="model.shortName" maxlength="20" /><small
          >用于导航和通知场景，最多 20 个字符</small
        ></label
      ><label class="field"
        ><span>系统主题色</span>
        <div class="theme-swatches">
          <button class="theme-swatch" type="button" disabled aria-label="标准蓝色主题">
            <AppIcon name="Check" :size="17" />
          </button>
        </div>
        <small>沿用本项目统一蓝白设计语言</small></label
      ><label class="field full-width"
        ><span>平台描述</span><textarea v-model="model.description" maxlength="200" />
      </label>
    </div>
  </section>
  <section class="settings-section">
    <h3>语言与时间</h3>
    <p>日期与时间使用企业统一时区，避免统计与操作日志口径不一致。</p>
    <div class="form-grid">
      <label class="field"
        ><span>默认语言</span
        ><select v-model="model.language">
          <option>简体中文</option>
          <option>English</option></select
        ><small>语言选项仅保存偏好；本轮界面提供简体中文</small></label
      ><label class="field"
        ><span>企业时区</span
        ><select v-model="model.timezone">
          <option value="Asia/Shanghai">(UTC+08:00) 北京、上海</option>
          <option value="UTC">(UTC+00:00) 协调世界时</option>
          <option value="Europe/Berlin">欧洲 / 柏林</option></select
        ><small>演示数据的时间标注保持 Asia/Shanghai</small></label
      >
    </div>
  </section>
</template>
