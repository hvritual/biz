<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { usePlanStore } from "../model/planStore";
import PageHeader from "@/shared/ui/PageHeader.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import BrandLogo from "@/shared/ui/BrandLogo.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import { useCompanyStore } from "../model/companyStore";
import { useAuditStore } from "@/services/audit";
import { useToast } from "@/shared/ui/useToast";
const planStore = usePlanStore();
const company = useCompanyStore();
const audit = useAuditStore();
const toast = useToast();
const form = reactive({ ...company.profile });
const error = ref("");
const saved = ref("尚未修改");
watch(
  () => company.profile,
  (p) => {
    Object.assign(form, p);
    saved.value = "尚未修改";
    error.value = "";
  },
);
function save() {
  try {
    company.save({ ...form });
    saved.value = "刚刚保存";
    toast.show("企业信息已保存（本地演示）。");
  } catch (e) {
    error.value = (e as Error).message;
  }
}
function scrollSection(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
}
</script>
<template>
  <PageHeader
    title="企业信息管理"
    description="维护企业基本资料与联系信息，让企业身份和协作关系清晰一致"
  />
  <div class="company-layout">
    <section class="panel">
      <div class="tabs">
        <button type="button" @click="scrollSection('company-basic')" class="active">
          基本信息</button
        ><button type="button" @click="scrollSection('company-contact')">联系信息</button>
      </div>
      <form id="company-form" class="company-form" @submit.prevent="save">
        <h3 id="company-basic" class="section-title">基本信息</h3>
        <div class="form-grid">
          <label class="field"
            ><span>企业名称 <em>*</em></span
            ><input v-model="form.name" required maxlength="80"
          /></label>
          <div class="field">
            <span>企业 Logo</span>
            <div class="company-logo-box"><BrandLogo /></div>
            <small>复用已确认的 CoffeeLink 品牌素材</small>
          </div>
          <label class="field"
            ><span>企业简称 <em>*</em></span
            ><input v-model="form.shortName" required maxlength="30" /></label
          ><label class="field"
            ><span>所属行业</span
            ><select v-model="form.industry">
              <option>咖啡设备与运营服务</option>
              <option>连锁零售</option>
              <option>设备租赁与服务</option>
            </select></label
          ><label class="field"
            ><span>企业规模</span
            ><select v-model="form.size">
              <option>1–50 人</option>
              <option>50–200 人</option>
              <option>200–500 人</option>
              <option>500 人以上</option>
            </select></label
          ><label class="field"
            ><span>默认时区</span
            ><select v-model="form.timezone">
              <option value="Asia/Shanghai">(UTC+08:00) 北京、上海</option>
              <option value="Europe/Berlin">Europe/Berlin</option>
              <option value="UTC">UTC</option>
            </select></label
          ><label class="field full-width"
            ><span>企业简介</span
            ><textarea v-model="form.description" maxlength="500" /><small
              >{{ form.description.length }} / 500</small
            ></label
          >
        </div>
        <h3 id="company-contact" class="section-title separated">联系信息</h3>
        <div class="form-grid">
          <label class="field"
            ><span>联系人 <em>*</em></span
            ><input v-model="form.contact" required /></label
          ><label class="field"
            ><span>联系邮箱 <em>*</em></span
            ><input v-model="form.email" type="email" required /></label
          ><label class="field"><span>联系电话</span><input v-model="form.phone" /></label
          ><label class="field"><span>所在城市</span><input v-model="form.city" /></label>
        </div>
        <p v-if="error" role="alert" class="form-error">{{ error }}</p>
      </form>
      <footer class="sticky-save">
        <BaseButton variant="primary" icon="Check" type="submit" form="company-form"
          >保存修改</BaseButton
        ><small>{{ saved }} · 当前企业演示资料</small>
      </footer>
    </section>
    <aside class="company-sidebar">
      <section class="panel">
        <h3 class="panel-heading">企业状态</h3>
        <div class="panel-content">
          <div class="safety-title">
            <span class="metric-icon tone-green"
              ><AppIcon name="Building2" :size="26"
            /></span>
            <div>
              <h3>演示企业</h3>
              <p>资料仅用于界面预览</p>
            </div>
          </div>
          <ul class="cert-list">
            <li><AppIcon name="CheckCircle2" />基础资料 <span>已填写</span></li>
            <li><AppIcon name="CheckCircle2" />联系信息 <span>已填写</span></li>
            <li><AppIcon name="Circle" />企业认证 <span>未接入</span></li>
          </ul>
          <div class="info-banner">
            不展示未经核验的“已认证”标识；企业资质认证待真实流程接入。
          </div>
        </div>
      </section>
      <section class="panel">
        <h3 class="panel-heading">套餐情况</h3>
        <div class="panel-content">
          <div class="plan-title">
            <span class="metric-icon tone-blue"><AppIcon name="Crown" :size="25" /></span>
            <div>
              <h3>{{ planStore.name }} <StatusBadge tone="green">演示</StatusBadge></h3>
              <p>有效期至 2027-09-08</p>
            </div>
          </div>
          <RouterLink to="/enterprise/plan" class="text-button"
            >查看套餐权益 <AppIcon name="ArrowRight" :size="14"
          /></RouterLink>
        </div>
      </section>
      <section class="panel">
        <h3 class="panel-heading">最近更新记录</h3>
        <div class="panel-content">
          <div
            v-for="record in audit.records
              .filter((r) => r.action.includes('企业'))
              .slice(0, 4)"
            :key="record.id"
            class="timeline-item"
          >
            <span />
            <div>
              <strong>{{ record.action }}</strong>
              <p>{{ record.time }}</p>
              <small>{{ record.actor }}</small>
            </div>
          </div>
        </div>
      </section>
    </aside>
  </div>
</template>
