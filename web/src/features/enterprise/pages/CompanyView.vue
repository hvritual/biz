<script setup lang="ts">
import { computed, ref } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import brand from '@/assets/brand-mark.png'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  draft = ref({ ...store.company }),
  error = ref(''),
  logo = ref(brand)
const changed = computed(() => JSON.stringify(draft.value) !== JSON.stringify(store.company))
function save() {
  error.value = ''
  if (!draft.value.name.trim()) {
    error.value = '请填写企业名称。'
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(draft.value.email)) {
    error.value = '请填写有效的企业邮箱。'
    return
  }
  try {
    store.saveCompany(draft.value)
    ui.toast('企业资料已保存到本地预览。')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败。'
  }
}
function upload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type) || file.size > 2 * 1024 * 1024) {
    ui.toast('请选择不超过 2 MB 的 PNG、JPG 或 WebP 图片。', 'error')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    logo.value = String(reader.result)
    ui.toast('已在当前页面预览 Logo，尚未上传文件服务。', 'info')
  }
  reader.readAsDataURL(file)
}
</script>
<template>
  <div class="page-stack">
    <PageHeading title="企业信息" description="维护企业基本资料与联系信息，统一团队的身份与展示" />
    <div class="split-layout">
      <form class="card panel-pad company-form" @submit.prevent="save">
        <div class="row-between block-title">
          <h2>基本信息</h2>
          <StatusBadge text="企业正常" />
        </div>
        <div class="form-grid">
          <label class="field"
            ><span class="required">企业名称</span
            ><input v-model="draft.name" class="input" required maxlength="100" /></label
          ><label class="field"
            ><span class="required">企业简称</span
            ><input v-model="draft.shortName" class="input" required maxlength="30" /></label
          ><label class="field"
            ><span>所属行业</span><input v-model="draft.industry" class="input" maxlength="80" /></label
          ><label class="field"
            ><span>企业规模</span
            ><select v-model="draft.size" class="select">
              <option>1–10 人</option>
              <option>10–50 人</option>
              <option>50–200 人</option>
              <option>200 人以上</option>
            </select></label
          ><label class="field"
            ><span>默认时区</span
            ><select v-model="draft.timezone" class="select">
              <option value="Asia/Shanghai">中国标准时间 · UTC+08:00</option>
              <option value="UTC">协调世界时 · UTC</option>
              <option value="Europe/Berlin">欧洲柏林 · 按当地夏令时规则</option>
            </select></label
          >
          <div class="field">
            <span>企业 Logo</span>
            <div class="logo-control">
              <img :src="logo" alt="企业 Logo 预览" /><label class="btn"
                ><AppIcon name="upload" :size="15" />选择图片<input
                  class="sr-only"
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  aria-label="选择企业 Logo"
                  @change="upload"
              /></label>
            </div>
            <small>PNG / JPG / WebP，最大 2 MB；当前仅预览。</small>
          </div>
          <label class="field full-width"
            ><span>企业简介</span
            ><textarea v-model="draft.description" class="textarea" rows="3" maxlength="500" />
          </label>
        </div>
        <section class="form-section">
          <h3>联系信息</h3>
          <div class="form-grid">
            <label class="field"
              ><span>联系人</span><input v-model="draft.contact" class="input" maxlength="40" /></label
            ><label class="field"
              ><span>联系电话</span><input v-model="draft.phone" class="input" maxlength="30" /></label
            ><label class="field"
              ><span class="required">企业邮箱</span
              ><input v-model="draft.email" class="input" type="email" required maxlength="150" /></label
            ><label class="field"
              ><span>联系地址</span><input v-model="draft.address" class="input" maxlength="200"
            /></label>
          </div>
        </section>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
        <div class="form-footer">
          <span v-if="changed" class="muted flex-1">有尚未保存的修改</span
          ><button
            class="btn"
            type="button"
            @click="
              () => {
                draft = { ...store.company }
                logo = brand
              }
            "
          >
            取消修改</button
          ><button class="btn btn-primary" type="submit"><AppIcon name="check" :size="15" />保存修改</button>
        </div>
      </form>
      <aside class="side-summary">
        <section class="card panel-pad">
          <h2>企业资料概览</h2>
          <div class="company-identity">
            <span><AppIcon name="company" :size="28" /></span>
            <h3>{{ store.company.name }}</h3>
            <p>{{ store.company.industry }}</p>
          </div>
          <dl class="detail-list">
            <dt>租户标识</dt>
            <dd class="mono">{{ store.tenantId }}</dd>
            <dt>当前套餐</dt>
            <dd>标准版（示例）</dd>
            <dt>企业成员</dt>
            <dd>{{ store.members.filter((m) => m.status !== 'removed').length }} 人</dd>
          </dl>
          <div class="divider" />
          <div class="notice-box">
            <AppIcon name="help" :size="16" />本轮不采集营业执照、法人证件或支付资料，不虚构企业认证结果。
          </div>
        </section>
        <section class="card panel-pad">
          <div class="row-between block-title">
            <h2>最近更新记录</h2>
            <RouterLink class="btn-link" to="/enterprise/logs">更多</RouterLink>
          </div>
          <div class="timeline">
            <div
              v-for="log in store.logs.filter((l) => l.module === '企业信息').slice(0, 4)"
              :key="log.id"
              class="timeline-item"
            >
              <strong>{{ log.action }}</strong
              ><small>{{ log.time }} · {{ log.actor }}</small>
            </div>
          </div>
        </section>
      </aside>
    </div>
  </div>
</template>
<style scoped>
.company-form {
  padding: 28px;
}
.logo-control {
  display: flex;
  align-items: center;
  gap: 20px;
}
.logo-control img {
  width: 42px;
  height: 45px;
  object-fit: contain;
}
.company-identity {
  text-align: center;
  padding: 24px 5px;
}
.company-identity > span {
  width: 64px;
  height: 64px;
  display: grid;
  place-items: center;
  background: var(--color-primary-soft);
  border-radius: 20px;
  color: var(--color-primary);
  margin: 0 auto 18px;
}
.company-identity h3 {
  font-size: 15px;
}
.company-identity p {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 8px;
}
.side-summary .detail-list {
  grid-template-columns: 85px 1fr;
}
.side-summary .notice-box {
  font-size: 11px;
}
@media (max-width: 767px) {
  .company-form {
    padding: 20px;
  }
}
</style>
