<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import coffee from '@/assets/coffee-banner.webp'

const props = defineProps<{
  title: string
  description?: string
  breadcrumb?: string
  banner?: boolean
  bannerTitle?: string
  bannerDescription?: string
}>()
const route = useRoute()
const isPlatform = computed(() => route.meta.surface === 'platform')
const effectiveBreadcrumb = computed(() => props.breadcrumb || (isPlatform.value ? '平台管理' : '企业中心'))
const showBanner = computed(() => isPlatform.value || props.banner)
const effectiveBannerTitle = computed(() =>
  props.bannerTitle || (isPlatform.value ? '让租户能力配置更清晰、更可控' : '优秀的团队，成就更好的咖啡体验'),
)
const effectiveBannerDescription = computed(() =>
  props.bannerDescription || (isPlatform.value ? '模块 · 套餐 · 权益 · 额度 · 计量' : '让每一杯咖啡更智能、更高效'),
)
</script>

<template>
  <header class="page-heading" data-ui-region="page-heading">
    <div class="heading-main">
      <div class="breadcrumbs">
        <span>{{ effectiveBreadcrumb }}</span><span>/</span><strong>{{ title }}</strong>
      </div>
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div class="heading-side">
      <div v-if="showBanner" class="coffee-hero" :class="{ platform: isPlatform }">
        <div>
          <h2>{{ effectiveBannerTitle }}</h2>
          <p>{{ effectiveBannerDescription }}</p>
        </div>
        <img :src="coffee" alt="白色咖啡杯与咖啡时刻" />
      </div>
      <slot />
    </div>
  </header>
</template>

<style scoped>
.page-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  min-height: 108px;
  padding: 0 0 2px;
}
.heading-main {
  min-width: 0;
  flex: 1;
  padding-bottom: 5px;
}
.breadcrumbs {
  display: flex;
  gap: 9px;
  align-items: center;
  margin-bottom: 14px;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}
.breadcrumbs strong {
  font-weight: 500;
  color: var(--color-text);
}
.heading-main > p {
  max-width: 720px;
  margin-top: 7px;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  line-height: 1.65;
}
.heading-side {
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
  gap: 12px;
  flex: 0 0 auto;
}
.coffee-hero {
  position: relative;
  isolation: isolate;
  display: flex;
  align-items: center;
  width: min(430px, 35vw);
  height: 96px;
  overflow: hidden;
  padding: 18px 22px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: linear-gradient(115deg, var(--color-primary-soft), var(--color-surface));
}
.coffee-hero.platform {
  background: linear-gradient(115deg, var(--color-workbench), var(--color-surface));
}
.coffee-hero > div {
  z-index: 1;
  max-width: calc(100% - 92px);
}
.coffee-hero h2 {
  font-size: 17px;
  line-height: 1.5;
  letter-spacing: -0.2px;
}
.coffee-hero p {
  margin-top: 4px;
  color: var(--color-text-secondary);
  font-size: var(--text-xs);
}
.coffee-hero img {
  position: absolute;
  right: 0;
  bottom: 0;
  z-index: 0;
  width: 238px;
  height: 96px;
  object-fit: cover;
  object-position: right;
  mask-image: linear-gradient(to right, transparent, black 48%);
}
@media (max-width: 1200px) {
  .page-heading { gap: 18px; }
  .coffee-hero { width: min(360px, 34vw); padding: 16px 18px; }
  .coffee-hero h2 { font-size: 15px; }
  .coffee-hero img { opacity: 0.58; }
  .coffee-hero > div { max-width: 82%; }
}
@media (max-width: 850px) {
  .page-heading { min-height: 96px; }
  .heading-side { display: block; }
  .coffee-hero { display: none; }
  .breadcrumbs { margin-bottom: 11px; }
}
@media (max-height: 830px) and (min-width: 851px) {
  .page-heading { min-height: 82px; }
  .breadcrumbs { margin-bottom: 9px; }
  .coffee-hero { height: 82px; }
  .coffee-hero img { width: 198px; height: 82px; }
  .coffee-hero h2 { font-size: 15px; }
}
</style>
