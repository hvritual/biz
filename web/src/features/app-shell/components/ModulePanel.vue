<script setup lang="ts">
import { UiButton } from '@/ui/base'
import { computed, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/stores/ui'
import { enterpriseNavigation, platformCommercialNavigation, platformCommercialQuickActions, systemNavigation, systemQuickActions, quickActions, primaryNavigation, type NavigationItem } from '@/router/navigation'
import AppIcon from '@/ui/common/AppIcon.vue'
import coffee from '@/assets/coffee-menu.webp'
import { customerDomains } from '@/router/customerNavigation'
import {
  authorizationApiMode,
  currentAuthorizationAllows,
  currentAuthorizationAllowsAny,
  currentAuthorizationState,
} from '@/services/runtime/authorization'
const ui=useUiStore(),route=useRoute(),router=useRouter(); const {t}=useI18n(); const closeButton=ref<HTMLButtonElement|null>(null)
onMounted(()=>{void nextTick(()=>closeButton.value?.focus())})
const primaryTitle=computed(()=>{const item=primaryNavigation.find(p=>p.id===ui.module);return item?t(`navigation.primary.${item.id}`):t('navigation.primary.enterprise')})
const links=computed(()=>ui.module==='enterprise'?enterpriseNavigation:ui.module==='platform-commercial'?platformCommercialNavigation:ui.module==='system'?systemNavigation:(customerDomains[ui.module??'']?.links??[]))
const visibleLinks=computed(()=>links.value.filter((item)=>{
  if(!authorizationApiMode())return true
  if(ui.module==='platform-commercial')return true
  if(!item.authorizationActions?.length)return false
  return currentAuthorizationState.status==='ready'&&currentAuthorizationAllowsAny(item.authorizationActions)
}))
const linkLabel=(item:NavigationItem)=>ui.module==='enterprise'?t(`navigation.enterprise.${item.id}`):ui.module==='platform-commercial'?t(`navigation.platform.${item.id}`):ui.module==='system'?t(`navigation.system.${item.id}`):item.label
const translatedPlatformGroups=new Set(['overview','lifecycle','product','entitlement','governance'])
const groupLabel=(id:string,raw:string)=>translatedPlatformGroups.has(id)?t(`navigation.groups.${id}`):raw
const linkGroups=computed(()=>{const groups:Array<{label:string;raw:string;items:NavigationItem[]}>=[];for(const item of visibleLinks.value){const id=item.groupId??item.group??'';let group=groups.find(g=>g.raw===id);if(!group){group={raw:id,label:groupLabel(id,item.group??''),items:[]};groups.push(group)}group.items.push(item)}return groups})
const actions=computed(()=>ui.module==='enterprise'?quickActions:ui.module==='platform-commercial'?platformCommercialQuickActions:ui.module==='system'?systemQuickActions:(customerDomains[ui.module??'']?.actions??[]))
const visibleActions=computed(()=>actions.value.filter((action)=>{
  const required='authorizationActions' in action&&Array.isArray(action.authorizationActions)?action.authorizationActions:[]
  if(!authorizationApiMode())return true
  if(ui.module==='platform-commercial')return true
  if(!required.length)return false
  if(currentAuthorizationState.status!=='ready')return false
  return 'authorizationMode' in action&&action.authorizationMode==='all'
    ? required.every(currentAuthorizationAllows)
    : currentAuthorizationAllowsAny(required)
}))
const quickKey=(path:string)=>({
  '/enterprise/members?action=create':'addMember','/enterprise/members?action=invite':'inviteMember','/enterprise/roles?action=create':'createRole','/enterprise/plan?action=upgrade':'adjustPlan','/enterprise/company':'editCompany','/enterprise/logs':'viewLogs','/platform/tenants':'openTenants','/platform/commercial/plans':'publishPlan','/platform/commercial/tenant-entitlements':'adjustEntitlements','/platform/commercial/usage-billing':'viewUsage','/system/general':'openGeneral','/system/security':'openSecurity','/system/notifications':'openNotifications','/system/integrations':'openIntegrations'
} as Record<string,string>)[path]
const actionLabel=(action:{label:string;path:string})=>{const key=quickKey(action.path);return key?t(`navigation.quick.${key}`):action.label}
const description=computed(()=>ui.module==='enterprise'?t('shell.enterpriseDescription'):ui.module==='platform-commercial'?t('shell.platformDescription'):ui.module==='system'?t('shell.systemDescription'):(customerDomains[ui.module??'']?.description??t('shell.fallbackDescription')))
function navigate(path?:string){if(path){ui.closeMenu();void router.push(path)}}
function isLinkActive(item:NavigationItem){return Boolean(item.path&&route.path===item.path.split('?')[0])}
</script>
<template>
  <section id="module-drawer" class="module-panel" role="dialog" aria-modal="true" :aria-label="t('shell.moduleNavigation',{title:primaryTitle})">
    <header class="module-heading"><div><h2>{{ primaryTitle }}</h2><p>{{ description }}</p></div><UiButton ref="closeButton" class="icon-button" :aria-label="t('shell.closeModule')" @click="ui.closeMenu()"><AppIcon name="close" :size="18"/></UiButton></header>
    <div v-if="visibleLinks.length" :class="['module-columns',{single:!visibleActions.length}]">
      <nav class="sub-navigation" :aria-label="t('shell.functionMenu')"><h3>{{ t('shell.functionMenu') }}</h3><section v-for="group in linkGroups" :key="group.raw||'default'" class="sub-group" :data-menu-group="group.raw||undefined"><h4 v-if="group.raw">{{ group.label }}</h4><UiButton v-for="item in group.items" :key="item.id" :class="['sub-link',{active:isLinkActive(item),unavailable:!item.path}]" :aria-current="isLinkActive(item)?'page':undefined" :disabled="!item.path" :title="item.path?linkLabel(item):`${linkLabel(item)} · ${t('common.unavailable')}`" @click="navigate(item.path)"><AppIcon :name="item.icon" :size="20"/><span>{{ linkLabel(item) }}</span><small v-if="!item.path">{{ t('common.unavailable') }}</small></UiButton></section></nav>
      <nav v-if="visibleActions.length" class="quick-navigation" :aria-label="t('shell.quickActions')"><h3>{{ t('shell.quickActions') }}</h3><UiButton v-for="action in visibleActions" :key="action.path" class="quick-link" @click="navigate(action.path)"><span :class="['quick-icon',{orange:action.icon==='crown'}]"><AppIcon :name="action.icon" :size="19"/></span><span>{{ actionLabel(action) }}</span></UiButton></nav>
    </div>
    <div v-else class="module-unavailable"><AppIcon name="lock" :size="32"/><h3>{{ t('shell.unavailableTitle') }}</h3><p>{{ t('shell.unavailableDescription') }}</p><UiButton class="btn" @click="ui.module='enterprise'">{{ t('shell.enterEnterprise') }}</UiButton></div>
    <footer class="menu-art"><div><h3>{{ t('shell.artTitle') }}</h3><p>{{ t('shell.artDescription') }}</p></div><img :src="coffee" alt="CoffeeLink"/></footer>
  </section>
</template>
<style scoped>
.module-panel{width:var(--module-width);min-width:var(--module-width);max-height:100%;display:flex;flex-direction:column;padding:20px;background:var(--color-surface);border-radius:0 var(--radius-xl) var(--radius-xl) 0;box-shadow:var(--shadow-menu);overflow-y:auto}.module-heading{position:relative;display:flex;gap:12px;justify-content:space-between;border-bottom:1px solid var(--color-border);padding-bottom:19px;flex-shrink:0}.module-heading h2{font-size:23px;font-weight:700;letter-spacing:-.5px}.module-heading p{font-size:var(--text-sm);color:var(--color-text-secondary);margin-top:6px}.module-heading>.icon-button{margin-top:-5px;margin-right:-8px;flex-shrink:0}.module-columns{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:12px;flex:1 0 auto;padding-top:20px;min-height:420px}.module-columns.single{grid-template-columns:minmax(0,1fr)}.module-columns h3{font-size:var(--text-sm);font-weight:650;margin-bottom:17px}.quick-navigation{border-left:1px solid var(--color-border);padding-left:20px}.sub-group+.sub-group{margin-top:14px}.sub-group h4{margin:0 0 5px;color:var(--color-text-muted);font-size:11px;font-weight:650;letter-spacing:.02em}.sub-link{display:flex;align-items:center;gap:12px;padding:9px 12px;width:fit-content;max-width:100%;min-height:40px;margin:3px 0 5px -6px;border-radius:8px;font-size:14px;white-space:nowrap}.sub-link>.icon{color:var(--color-text-secondary)}.sub-link.active{background:var(--color-primary-soft);color:var(--color-primary)}.sub-link.active>.icon{color:var(--color-primary)}.sub-link:hover:not(:disabled){background:var(--color-surface-soft)}.sub-link.active:hover:not(:disabled){background:var(--color-primary-soft)}.sub-link.unavailable{cursor:not-allowed;color:var(--color-text-muted)}.sub-link.unavailable>.icon{color:var(--color-text-muted)}.sub-link small{padding:2px 6px;border-radius:999px;background:var(--color-surface-soft);color:var(--color-text-muted);font-size:10px;font-weight:500}.quick-link{width:100%;display:flex;align-items:center;gap:10px;min-height:48px;border:1px solid var(--color-border);background:linear-gradient(115deg,var(--color-surface),var(--color-surface-soft));border-radius:8px;padding:8px 10px;margin-bottom:10px;text-align:left;font-size:12px;white-space:normal;overflow-wrap:anywhere}.quick-link:hover{border-color:var(--color-primary);background:var(--color-primary-soft)}.quick-icon{display:flex;color:var(--color-primary)}.quick-icon.orange{color:var(--color-warning)}.menu-art{position:relative;display:flex;align-items:center;min-height:125px;border-radius:10px;background:linear-gradient(115deg,var(--color-primary-soft),var(--color-canvas));isolation:isolate;overflow:hidden;flex-shrink:0;margin-top:24px;padding:22px 20px}.menu-art>div{z-index:1;max-width:280px}.menu-art h3{font-size:16px;letter-spacing:-.3px}.menu-art p{font-size:11px;margin-top:8px;color:var(--color-text-secondary)}.menu-art img{position:absolute;right:0;bottom:0;height:126px;width:140px;object-fit:cover;mask-image:linear-gradient(to right,transparent,black 35%);z-index:0}.module-unavailable{flex:1;display:flex;flex-direction:column;gap:16px;align-items:flex-start;justify-content:center;color:var(--color-text-secondary);font-size:var(--text-sm);padding-bottom:20px}.module-unavailable>.icon{color:var(--color-primary)}
@media(max-height:800px){.module-panel{padding:20px}.module-columns{min-height:360px;padding-top:15px}.sub-group+.sub-group{margin-top:10px}.sub-link{min-height:37px;margin-bottom:4px}.quick-link{min-height:43px;margin-bottom:8px}.module-columns h3{margin-bottom:12px}.menu-art{min-height:96px;margin-top:15px}.menu-art img{height:103px}.menu-art p{max-width:245px}}
@media(max-width:767px){.module-panel{width:calc(100vw - var(--rail-collapsed-width));min-width:0;padding:20px 16px}.module-columns{gap:12px;grid-template-columns:1fr 1fr}.module-columns.single{grid-template-columns:1fr}.quick-navigation{padding-left:12px}.sub-group+.sub-group{margin-top:10px}.sub-group h4{font-size:10px}.sub-link{font-size:12px;gap:7px;padding-left:6px;padding-right:7px;min-height:36px;white-space:normal}.quick-link{font-size:11px;padding:7px;gap:6px}.quick-icon>.icon{width:16px}.menu-art{padding:15px}.menu-art h3{font-size:14px}.menu-art p{max-width:145px}.module-heading h2{font-size:20px}.module-heading p{font-size:11px}}
</style>