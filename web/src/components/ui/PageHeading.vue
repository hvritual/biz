<script setup lang="ts">
import coffee from '@/assets/coffee-banner.webp'
defineProps<{
  title: string
  description?: string
  breadcrumb?: string
  banner?: boolean
  compact?: boolean
}>()
</script>
<template>
  <header class="page-heading" :class="{ compact }">
    <div class="heading-main">
      <div v-if="!compact" class="breadcrumbs">
        <span>{{ breadcrumb || '企业中心' }}</span
        ><span>/</span><strong>{{ title }}</strong>
      </div>
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div v-if="banner" class="coffee-hero">
      <div>
        <h2>优秀的团队，成就更好的咖啡体验</h2>
        <p>让每一杯咖啡更智能、更高效</p>
      </div>
      <img :src="coffee" alt="白色咖啡杯与咖啡时刻" />
    </div>
    <slot />
  </header>
</template>
<style scoped>
.page-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  min-height: 115px;
  padding: 0 0 2px;
}
.breadcrumbs {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  margin-bottom: 18px;
}
.breadcrumbs strong {
  font-weight: 500;
  color: var(--color-text);
}
.heading-main {
  padding-bottom: 5px;
}
.heading-main > p {
  margin-top: 8px;
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
}
.coffee-hero {
  position: relative;
  isolation: isolate;
  display: flex;
  align-items: center;
  padding: 22px 26px;
  width: 47%;
  height: 105px;
  overflow: hidden;
  border-radius: var(--radius-md);
  background: linear-gradient(115deg, var(--color-primary-soft), var(--color-surface));
}
.coffee-hero > div {
  z-index: 1;
  max-width: calc(100% - 90px);
}
.coffee-hero h2 {
  font-size: 18px;
  line-height: 1.6;
  letter-spacing: -0.2px;
}
.coffee-hero p {
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
  margin-top: 3px;
}
.coffee-hero img {
  position: absolute;
  right: 0;
  bottom: 0;
  height: 105px;
  width: 265px;
  object-fit: cover;
  object-position: right;
  mask-image: linear-gradient(to right, transparent, black 46%);
  z-index: 0;
}
@media (max-width: 1200px) {
  .coffee-hero {
    width: 43%;
    padding: 18px;
  }
  .coffee-hero h2 {
    font-size: 15px;
  }
  .coffee-hero img {
    opacity: 0.55;
  }
  .coffee-hero > div {
    max-width: 85%;
  }
}
@media (max-width: 850px) {
  .coffee-hero {
    display: none;
  }
  .page-heading {
    min-height: 102px;
  }
  .breadcrumbs {
    margin-bottom: 12px;
  }
}
@media (max-height: 830px) and (min-width: 851px) {
  .page-heading {
    min-height: 86px;
  }
  .breadcrumbs {
    margin-bottom: 10px;
  }
  .coffee-hero {
    height: 85px;
  }
  .coffee-hero img {
    height: 85px;
    width: 205px;
  }
  .coffee-hero h2 {
    font-size: 16px;
  }
}
.page-heading.compact {
  min-height: 80px;
  align-items: center;
  gap: 18px;
}
.compact .heading-main {
  flex: 1;
  min-width: 0;
  padding-bottom: 0;
}
.compact .heading-main > p {
  line-height: 1.65;
}
.compact .coffee-hero {
  width: 43%;
  height: 80px;
  flex-shrink: 0;
  padding: 15px 20px;
}
.compact .coffee-hero img {
  height: 80px;
  width: 205px;
  opacity: 1;
}
.compact .coffee-hero h2 {
  font-size: 16px;
}
.compact .coffee-hero > div {
  max-width: calc(100% - 70px);
}
@container member-main (max-width: 1000px) {
  .compact .coffee-hero {
    width: 36%;
    padding: 12px 14px;
  }
  .compact .coffee-hero h2 {
    font-size: 12px;
  }
  .compact .coffee-hero p {
    font-size: 10px;
  }
  .compact .coffee-hero img {
    width: 135px;
  }
  .compact .coffee-hero > div {
    max-width: calc(100% - 45px);
  }
  .compact .heading-main > p {
    font-size: 12px;
  }
}
@media (max-width: 850px) {
  .page-heading.compact {
    min-height: 76px;
  }
}
</style>
