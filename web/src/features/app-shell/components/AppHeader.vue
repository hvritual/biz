<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUiStore } from '@/stores/ui'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useRouter, useRoute } from 'vue-router'
import { setUiLocale, type UiLocale } from '@/i18n'
import AppIcon from '@/ui/common/AppIcon.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppearanceControls from '@/ui/common/AppearanceControls.vue'
import brand from '@/assets/brand-mark.png'

const ui = useUiStore()
const store = useEnterpriseStore()
const router = useRouter()
const route = useRoute()
const { t, locale } = useI18n()

const live = computed(() => route.meta.surface === 'platform' || route.meta.surface === 'runtime')
const search = ref('')
const panel = ref('')

function changeLocale(event: Event) {
  setUiLocale((event.target as HTMLSelectElement).value as UiLocale)
}

async function changeTenant(event: Event) {
  const tenantId = (event.target as HTMLSelectElement).value
  if (!tenantId) return
  ui.closeMenu()
  search.value = ''
  panel.value = ''
  try {
    await store.switchTenant(tenantId)
    await router.replace({ path: route.path, query: {} })
    ui.toast(store.sourceKind === 'api' ? t('header.tenantSwitchedApi') : t('header.tenantSwitchedPreview'), 'info')
  } catch (error) {
    ui.toast(error instanceof Error ? error.message : t('header.tenantSwitchFailed'), 'error')
  }
}

function globalSearch() {
  if (search.value.trim()) {
    void router.push({ path: '/enterprise/members', query: { q: search.value.trim() } })
    ui.closeMenu()
  }
}

async function logout() {
  panel.value = ''
  try {
    await store.logout()
    window.location.assign(store.loginHref())
  } catch (error) {
    ui.toast(error instanceof Error ? error.message : t('common.logout'), 'error')
  }
}
</script>

<template>
  <header class="app-header">
    <a class="brand" href="#/enterprise/members" :aria-label="t('header.brandAria')">
      <img :src="brand" alt="CoffeeLink" />
      <span><strong>CoffeeLink</strong><small>{{ t('header.platformSubtitle') }}</small></span>
    </a>
    <UiButton class="icon-button mobile-toggle" :aria-label="t('header.openNav')" @click="ui.mobileOpen = !ui.mobileOpen">
      <AppIcon name="menu" />
    </UiButton>

    <div v-if="!live" class="header-company">
      <AppIcon name="company" :size="19" />
      <UiSelect :value="store.tenantId" :aria-label="t('header.switchCompany')" :disabled="store.loading" @change="changeTenant">
        <UiOption v-if="store.tenantOptions.length === 0" value="" disabled>{{ t('header.selectCompany') }}</UiOption>
        <UiOption v-for="tenant in store.tenantOptions" :key="tenant.id" :value="tenant.id">{{ tenant.name }}</UiOption>
      </UiSelect>
      <span class="edition">{{ store.sourceKind === 'api' ? t('header.liveData') : t('header.standardEdition') }}</span>
    </div>
    <div v-else class="header-company">
      {{ route.meta.surface === 'platform' ? t('header.platform') : t('header.workspace') }}
    </div>

    <form v-if="!live" class="global-search" role="search" @submit.prevent="globalSearch">
      <AppIcon name="search" :size="16" />
      <UiInput v-model="search" :aria-label="t('header.globalSearch')" :placeholder="t('header.searchPlaceholder')" />
      <span>⌘ K</span>
    </form>

    <div class="header-actions">
      <AppearanceControls />
      <UiSelect class="locale-select" :value="locale" :aria-label="t('common.locale')" @change="changeLocale">
        <UiOption value="zh-CN">{{ t('common.chinese') }}</UiOption>
        <UiOption value="en-US">{{ t('common.english') }}</UiOption>
      </UiSelect>
      <template v-if="!live">
        <UiButton class="icon-button notification" :aria-label="t('header.notifications')" @click="panel = t('header.notifications')">
          <AppIcon name="bell" :size="21" /><b>12</b>
        </UiButton>
        <UiButton class="header-link" @click="panel = t('header.help')"><AppIcon name="help" />{{ t('header.help') }}</UiButton>
        <UiButton class="header-link" @click="panel = t('header.downloads')"><AppIcon name="download" />{{ t('header.downloads') }}</UiButton>
        <UiButton class="profile" @click="panel = t('header.currentAccount')">
          <AvatarMark :name="store.session?.user_id || 'U'" :size="36" tone="solid" />
          <span>
            {{ store.session?.user_id || 'User' }}
            <small>{{ store.sourceKind === 'api' ? t('header.currentLogin') : t('header.superAdmin') }}</small>
          </span>
          <AppIcon name="down" :size="14" />
        </UiButton>
      </template>
    </div>
  </header>

  <UiDialog :open="Boolean(panel)" :title="panel" @close="panel = ''">
    <div class="page-stack">
      <div class="notice-box">
        <AppIcon name="help" />
        {{ store.sourceKind === 'api' ? 'The current enterprise surface uses server data; unavailable notification or download capabilities remain explicit.' : 'This is an isolated frontend preview and is not connected to production notification or download services.' }}
      </div>
      <template v-if="panel === t('header.help')">
        <h3>{{ t('shell.companyGuide') }}</h3>
        <p class="secondary">Use the primary navigation to open the connected module panel. Escape closes overlays and keyboard navigation remains available.</p>
      </template>
      <template v-else-if="panel === t('header.notifications')">
        <p>No notification source is connected.</p>
        <UiButton class="btn" @click="() => { router.push('/system/notifications'); panel = '' }">Open notification settings</UiButton>
      </template>
      <template v-else-if="panel === t('header.downloads')">
        <p>List exports are downloaded by the browser and are not uploaded to a remote service.</p>
      </template>
      <template v-else>
        <p v-if="store.sourceKind === 'api'">{{ store.session?.user_id || 'Unknown user' }} · {{ store.tenantId || 'No tenant selected' }}</p>
        <p v-else>Preview identity · enterprise owner</p>
        <UiButton v-if="store.sourceKind === 'api'" class="btn" @click="logout">{{ t('common.logout') }}</UiButton>
      </template>
    </div>
  </UiDialog>
</template>

<style scoped>
.app-header { height: var(--header-height); position: fixed; inset: 0 0 auto; z-index: var(--z-header); display: flex; align-items: center; gap: 20px; padding: 0 24px 0 20px; backdrop-filter: blur(10px); }
.brand { display: flex; align-items: center; gap: 7px; width: 176px; flex-shrink: 0; color: var(--color-text); }
.brand img { width: 38px; height: 43px; object-fit: contain; mix-blend-mode: multiply; }
:global(:root[data-ui-mode='dark']) .brand img { mix-blend-mode: normal; }
.brand strong { display: block; font-size: 21px; line-height: 1.2; letter-spacing: -0.6px; font-weight: 750; }
.brand small { display: block; font-size: 11px; margin-top: 2px; }
.header-company { display: flex; align-items: center; gap: 9px; white-space: nowrap; }
.header-company > .icon { color: var(--color-success); }
.header-company select { border: 0; background: transparent; font-size: 12px; font-weight: 600; max-width: 190px; outline-offset: 4px; }
.edition { font-size: 12px; background: var(--color-primary-soft); color: var(--color-primary); border-radius: 7px; padding: 8px 11px; }
.global-search { display: flex; align-items: center; gap: 8px; padding: 0 12px; height: 36px; border: 1px solid var(--color-border); border-radius: 9px; background: var(--color-surface-soft); margin: 0 auto; max-width: 470px; flex: 1; min-width: 100px; color: var(--color-text-muted); }
.global-search input { border: 0; background: none; outline: 0; min-width: 0; flex: 1; font-size: 12px; }
.global-search input::placeholder { color: var(--color-text-muted); }
.global-search > span { font-size: 11px; white-space: nowrap; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.locale-select { width: 96px; min-width: 96px; font-size: 11px; }
.header-link { display: flex; align-items: center; gap: 6px; white-space: nowrap; padding: 0; font-size: 12px; }
.notification { position: relative; }
.notification b { position: absolute; right: -1px; top: 0; color: var(--color-on-primary); background: var(--color-danger); font-size: 9px; line-height: 14px; min-width: 15px; border-radius: 9px; border: 1px solid var(--color-surface); }
.profile { display: flex; align-items: center; gap: 10px; text-align: left; padding: 0 0 0 12px; border-left: 1px solid var(--color-border); font-size: 13px; }
.profile small { display: block; color: var(--color-text-muted); font-size: 11px; }
.mobile-toggle { display: none; }
@media (max-width: 1250px) { .header-link { display: none; } .header-company select { max-width: 155px; } .header-actions { gap: 6px; } .app-header { gap: 14px; } .edition { display: none; } }
@media (max-width: 900px) { .locale-select { display: none; } }
@media (max-width: 767px) { .app-header { padding: 0 14px; gap: 8px; } .brand { width: auto; flex: 1; } .brand strong { font-size: 19px; } .brand img { height: 37px; width: 31px; } .brand small { font-size: 10px; } .header-company, .global-search, .profile > span:not(.avatar-mark), .profile > .icon { display: none; } .profile { padding-left: 5px; border: 0; } .mobile-toggle { display: flex; order: -1; } .header-actions { gap: 2px; } }
</style>
