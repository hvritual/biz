<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUiStore } from '@/stores/ui'
import { useEnterpriseStore } from '@/stores/enterprise'
import { usePersonalProfileStore } from '@/stores/personalProfile'
import { useNotificationInboxStore } from '@/stores/notificationInbox'
import { isPersonalProfileSession } from '@/services/enterprise/personalProfileRuntime'
import type { InAppNotification } from '@/services/enterprise/notificationInboxRuntime'
import { useRouter, useRoute } from 'vue-router'
import { formatRelativeTime, setUiLocale, type UiLocale } from '@/i18n'
import AppIcon from '@/ui/common/AppIcon.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppearanceControls from '@/ui/common/AppearanceControls.vue'
import Notice from '@/ui/common/Notice.vue'
import brand from '@/assets/brand-mark.png'
import {
  authorizationApiMode,
  currentAuthorizationAllows,
  currentAuthorizationState,
} from '@/services/runtime/authorization'

const ui = useUiStore()
const store = useEnterpriseStore()
const personal = usePersonalProfileStore()
const inbox = useNotificationInboxStore()
const router = useRouter()
const route = useRoute()
const { t, locale } = useI18n()

const live = computed(() => route.meta.surface === 'platform' || route.meta.surface === 'runtime')
const search = ref('')
const panel = ref('')
const headerName = computed(() =>
  personal.profile?.name || personal.profile?.username || store.session?.user_id || 'User',
)
const headerAvatarRef = computed(() => personal.profile?.avatarAssetRef ?? '')
const headerTenant = computed(() => personal.profile?.tenantName || store.tenantId || '')
const unreadCount = computed(() => inbox.snapshot?.unread_count ?? 0)
const unreadBadge = computed(() => unreadCount.value > 99 ? '99+' : String(unreadCount.value))
const notificationBellLabel = computed(() => t('notificationInbox.bellLabel', { count: unreadCount.value }))
const notificationError = computed(() => inbox.error ? t(`notificationInbox.errors.${inbox.error}`) : '')
const showPanelNotice = computed(() => panel.value === t('header.help') || panel.value === t('header.downloads'))

const notificationTypeKeys: Record<string, string> = {
  'device.fault': 'deviceFault',
  'device.offline': 'deviceOffline',
  'service.maintenance_due': 'maintenanceDue',
  'device.connection_recovered': 'connectionRecovered',
  'system.announcement': 'announcement',
}

function changeLocale(event: Event) {
  setUiLocale((event.target as HTMLSelectElement).value as UiLocale)
}

async function refreshPersonalProfile() {
  if (
    store.sourceKind !== 'api' ||
    !isPersonalProfileSession(store.session) ||
    (authorizationApiMode() && (
      currentAuthorizationState.status !== 'ready' ||
      currentAuthorizationState.snapshot?.tenant_id !== store.session.active_tenant_id ||
      !currentAuthorizationAllows('tenant.member.personal_profile.get')
    ))
  ) {
    personal.clear()
    return
  }
  await personal.refresh(store.session)
}

async function refreshNotificationInbox() {
  if (live.value || store.sourceKind !== 'api' || !isPersonalProfileSession(store.session)) {
    inbox.clear()
    return
  }
  await inbox.refresh(store.session)
}

function openNotificationPanel() {
  panel.value = t('header.notifications')
  void refreshNotificationInbox()
}

function notificationTitle(message: InAppNotification) {
  const key = notificationTypeKeys[message.type_code]
  return key ? t(`notificationInbox.types.${key}`) : t('notificationInbox.types.unknown', { code: message.type_code })
}

function notificationReference(message: InAppNotification) {
  return t('notificationInbox.reference', { kind: message.reference_kind, id: message.reference_id })
}

function notificationTime(message: InAppNotification) {
  return formatRelativeTime(message.created_at)
}

async function markAllNotificationInboxRead() {
  const session = store.session
  if (!isPersonalProfileSession(session)) return
  const result = await inbox.markAllRead(session)
  if (!result) return
  if (result.snapshot.unread_count > 0) {
    ui.toast(t('notificationInbox.markedWithNew', {
      count: result.receipt.marked_count,
      remaining: result.snapshot.unread_count,
    }), 'info')
  } else {
    ui.toast(t('notificationInbox.marked', { count: result.receipt.marked_count }), 'success')
  }
}

async function changeTenant(event: Event) {
  const tenantId = (event.target as HTMLSelectElement).value
  if (!tenantId) return
  const recoveryTarget =
    route.path === '/authorization-state' && typeof route.query.from === 'string' && route.query.from.startsWith('/')
      ? route.query.from
      : route.fullPath
  ui.closeMenu()
  search.value = ''
  panel.value = ''
  personal.clear()
  inbox.clear()
  try {
    await store.switchTenant(tenantId)
    await refreshPersonalProfile()
    await refreshNotificationInbox()
    await router.replace(recoveryTarget)
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

function openPersonalProfile() {
  panel.value = ''
  ui.closeMenu()
  void router.push('/enterprise/personal-profile')
}

async function logout() {
  panel.value = ''
  personal.clear()
  inbox.clear()
  try {
    await store.logout()
    window.location.assign(store.loginHref())
  } catch (error) {
    ui.toast(error instanceof Error ? error.message : t('common.logout'), 'error')
  }
}

watch(
  () => [
    store.sourceKind,
    store.session?.authenticated,
    store.session?.actor_kind,
    store.session?.user_id,
    store.session?.active_tenant_id,
    store.session?.context_version,
    currentAuthorizationState.status,
    currentAuthorizationState.snapshot?.tenant_id,
    currentAuthorizationState.snapshot?.button_codes.join('|'),
    route.meta.surface,
  ],
  () => {
    void refreshPersonalProfile()
    void refreshNotificationInbox()
  },
  { immediate: true },
)

function refreshVisibleInbox() {
  if (document.visibilityState === 'visible') void refreshNotificationInbox()
}

let inboxTimer: number | undefined
onMounted(() => {
  inboxTimer = window.setInterval(refreshVisibleInbox, 30_000)
  document.addEventListener('visibilitychange', refreshVisibleInbox)
})
onBeforeUnmount(() => {
  if (inboxTimer !== undefined) window.clearInterval(inboxTimer)
  document.removeEventListener('visibilitychange', refreshVisibleInbox)
  inbox.clear()
})
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
    </div>
    <div v-else class="header-company">
      {{ route.meta.surface === 'platform' ? t('header.platform') : t('header.workspace') }}
    </div>

    <form v-if="!live" class="global-search" role="search" @submit.prevent="globalSearch">
      <UiInput v-model="search" :aria-label="t('header.globalSearch')" :placeholder="t('header.searchPlaceholder')">
        <template #prefix><AppIcon name="search" :size="16" /></template>
        <template #suffix><kbd class="global-search-shortcut">⌘ K</kbd></template>
      </UiInput>
    </form>

    <div class="header-actions">
      <AppearanceControls />
      <UiSelect class="locale-select" :value="locale" :aria-label="t('common.locale')" @change="changeLocale">
        <UiOption value="zh-CN">{{ t('common.chinese') }}</UiOption>
        <UiOption value="en-US">{{ t('common.english') }}</UiOption>
      </UiSelect>
      <template v-if="!live">
        <UiButton
          class="icon-button notification"
          :aria-label="notificationBellLabel"
          :title="notificationBellLabel"
          @click="openNotificationPanel"
        >
          <AppIcon name="bell" :size="21" /><b v-if="unreadCount > 0">{{ unreadBadge }}</b>
        </UiButton>
        <UiButton class="header-link" @click="panel = t('header.help')"><AppIcon name="help" />{{ t('header.help') }}</UiButton>
        <UiButton class="header-link" @click="panel = t('header.downloads')"><AppIcon name="download" />{{ t('header.downloads') }}</UiButton>
        <UiButton class="profile" :aria-label="t('header.currentAccount')" @click="panel = t('header.currentAccount')">
          <AvatarMark
            :name="headerName"
            :asset-ref="headerAvatarRef"
            :size="36"
            :tone="headerAvatarRef ? 'blue' : 'solid'"
          />
          <span>
            {{ headerName }}
            <small>{{ headerTenant || (store.sourceKind === 'api' ? t('header.currentLogin') : t('header.superAdmin')) }}</small>
          </span>
          <AppIcon name="down" :size="14" />
        </UiButton>
      </template>
    </div>
  </header>

  <UiDialog :open="Boolean(panel)" :title="panel" @close="panel = ''">
    <div class="page-stack">
      <Notice v-if="showPanelNotice">
        <AppIcon name="help" />
        {{ t('header.panelNotice') }}
      </Notice>
      <template v-if="panel === t('header.help')">
        <h3>{{ t('shell.companyGuide') }}</h3>
        <p class="secondary">{{ t('header.helpDescription') }}</p>
      </template>
      <template v-else-if="panel === t('header.notifications')">
        <section class="notification-inbox" data-notification-inbox aria-live="polite">
          <div class="notification-inbox-head">
            <div>
              <strong>{{ t('notificationInbox.scope') }}</strong>
              <small>{{ t('notificationInbox.refreshHint') }}</small>
            </div>
            <UiButton
              variant="outline"
              size="sm"
              :disabled="store.sourceKind !== 'api' || unreadCount === 0 || inbox.saving"
              @click="markAllNotificationInboxRead"
            >
              {{ inbox.saving ? t('notificationInbox.marking') : t('notificationInbox.markAll') }}
            </UiButton>
          </div>

          <Notice v-if="store.sourceKind !== 'api'" tone="warning">
            {{ t('header.notificationsUnavailable') }}
          </Notice>
          <p v-else-if="inbox.loading && !inbox.ready" class="secondary" role="status">
            {{ t('notificationInbox.loading') }}
          </p>
          <Notice v-if="notificationError" tone="danger" class="notification-inbox-error" role="alert">
            <span>{{ notificationError }}</span>
            <UiButton variant="outline" size="sm" @click="refreshNotificationInbox">{{ t('notificationInbox.retry') }}</UiButton>
          </Notice>

          <div
            v-if="store.sourceKind === 'api' && inbox.ready && inbox.snapshot && inbox.snapshot.messages.length === 0 && !notificationError"
            class="notification-empty"
            role="status"
          >
            <AppIcon name="bell" :size="26" />
            <strong>{{ t('notificationInbox.empty') }}</strong>
            <p>{{ t('notificationInbox.emptyHint') }}</p>
          </div>
          <div v-else-if="inbox.snapshot?.messages.length" class="notification-list">
            <article
              v-for="message in inbox.snapshot.messages"
              :key="message.message_id"
              class="notification-item"
              :data-level="message.level"
            >
              <div class="notification-item-meta">
                <span>{{ t(`notificationInbox.levels.${message.level}`) }}</span>
                <time :datetime="message.created_at">{{ notificationTime(message) }}</time>
              </div>
              <strong>{{ notificationTitle(message) }}</strong>
              <p>{{ notificationReference(message) }}</p>
            </article>
          </div>
        </section>
      </template>
      <template v-else-if="panel === t('header.downloads')">
        <p>{{ t('header.downloadDescription') }}</p>
      </template>
      <template v-else>
        <div v-if="store.sourceKind === 'api'" class="account-summary">
          <AvatarMark
            :name="headerName"
            :asset-ref="headerAvatarRef"
            :size="48"
            :tone="headerAvatarRef ? 'blue' : 'solid'"
          />
          <div>
            <strong>{{ headerName }}</strong>
            <small>{{ headerTenant || store.tenantId || t('common.unknown') }}</small>
          </div>
        </div>
        <p v-else>{{ t('header.defaultRole') }}</p>
        <div v-if="store.sourceKind === 'api'" class="account-actions">
          <UiButton class="btn btn-primary" @click="openPersonalProfile">{{ t('personalProfile.navigation') }}</UiButton>
          <UiButton class="btn" @click="logout">{{ t('common.logout') }}</UiButton>
        </div>
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
.global-search { margin: 0 auto; max-width: 470px; flex: 1; min-width: 100px; }
.global-search :deep([data-slot='input-wrapper']) { height: 36px; gap: 8px; border-color: var(--color-border); border-radius: 9px; background: var(--color-surface-soft); color: var(--color-text-muted); }
.global-search :deep([data-slot='input']) { min-width: 0; font-size: 12px; }
.global-search :deep([data-slot='input'])::placeholder { color: var(--color-text-muted); }
.global-search-shortcut { color: inherit; font: inherit; font-size: 11px; white-space: nowrap; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.locale-select { width: 96px; min-width: 96px; font-size: 11px; }
.header-link { display: flex; align-items: center; gap: 6px; white-space: nowrap; padding: 0; font-size: 12px; }
.notification { position: relative; }
.notification b { position: absolute; right: -1px; top: 0; color: var(--color-on-primary); background: var(--color-danger); font-size: 9px; line-height: 14px; min-width: 15px; padding: 0 3px; border-radius: 9px; border: 1px solid var(--color-surface); }
.notification-inbox { display: flex; min-width: min(520px, 78vw); max-height: min(620px, 72vh); flex-direction: column; gap: 12px; }
.notification-inbox-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.notification-inbox-head strong, .notification-inbox-head small { display: block; }
.notification-inbox-head small { margin-top: 4px; color: var(--color-text-muted); font-size: var(--text-xs); line-height: 1.5; }
.notification-inbox-error { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.notification-list { display: flex; min-height: 0; flex-direction: column; gap: 8px; overflow-y: auto; }
.notification-item { padding: 12px 14px; border: 1px solid var(--color-border); border-radius: 10px; background: var(--color-surface-soft); }
.notification-item[data-level='urgent'] { border-inline-start: 3px solid var(--color-danger); }
.notification-item[data-level='important'] { border-inline-start: 3px solid var(--color-warning); }
.notification-item[data-level='general'] { border-inline-start: 3px solid var(--color-primary); }
.notification-item-meta { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 6px; color: var(--color-text-muted); font-size: var(--text-xs); }
.notification-item > strong { display: block; font-size: var(--text-sm); }
.notification-item > p { margin: 5px 0 0; overflow-wrap: anywhere; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.5; }
.notification-empty { display: grid; justify-items: center; gap: 7px; padding: 28px 16px; text-align: center; color: var(--color-text-secondary); }
.notification-empty p { max-width: 360px; margin: 0; font-size: var(--text-xs); line-height: 1.6; }
.profile { display: flex; align-items: center; gap: 10px; text-align: left; padding: 0 0 0 12px; border-left: 1px solid var(--color-border); font-size: 13px; }
.profile small { display: block; max-width: 180px; overflow: hidden; color: var(--color-text-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.account-summary { display: flex; align-items: center; gap: 12px; }
.account-summary strong, .account-summary small { display: block; }
.account-summary small { margin-top: 4px; color: var(--color-text-muted); font-size: var(--text-xs); }
.account-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.mobile-toggle { display: none; }
@media (max-width: 1250px) { .header-link { display: none; } .header-company select { max-width: 155px; } .header-actions { gap: 6px; } .app-header { gap: 14px; } }
@media (max-width: 900px) { .locale-select { display: none; } }
@media (max-width: 767px) { .notification-inbox { min-width: 0; max-height: 66vh; } .notification-inbox-head { align-items: stretch; flex-direction: column; } .notification-inbox-head .inline-flex { width: 100%; } .app-header { padding: 0 14px; gap: 8px; } .brand { width: auto; flex: 1; } .brand strong { font-size: 19px; } .brand img { height: 37px; width: 31px; } .brand small { font-size: 10px; } .header-company, .global-search, .profile > span:not(.avatar-mark), .profile > .icon { display: none; } .profile { padding-left: 5px; border: 0; } .mobile-toggle { display: flex; order: -1; } .header-actions { gap: 2px; } .account-actions { flex-direction: column; } .account-actions .btn { width: 100%; } }
</style>
