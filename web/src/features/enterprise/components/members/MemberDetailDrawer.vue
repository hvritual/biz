<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Member, MemberAction } from '@/types/enterprise'
import { scopeLabels, statusLabels } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import MemberRoleTags from './MemberRoleTags.vue'
import MemberDetailOverview from './MemberDetailOverview.vue'
const props = defineProps<{ member: Member | null }>()
const emit = defineEmits<{
  close: []
  action: [action: MemberAction, member: Member]
  more: [member: Member]
}>()
const store = useEnterpriseStore()
const tabs = ['基本信息', '角色权限', '数据权限', '操作日志'] as const
const tab = ref<(typeof tabs)[number]>('基本信息')
const panel = ref<HTMLElement>()
const mobile = ref(false)
const memberLogs = computed(() => store.logs.filter((log) => log.target === props.member?.name))
let trigger: HTMLElement | null = null
let rootWasInert = false
let inertOwned = false
function setInert() {
  const root = document.getElementById('app')
  if (!root) return
  if (props.member && mobile.value) {
    if (!inertOwned) rootWasInert = root.inert
    root.inert = true
    inertOwned = true
  } else if (inertOwned) {
    root.inert = rootWasInert
    inertOwned = false
  }
}
function resize() {
  mobile.value = window.innerWidth < 1280
  setInert()
}
function keydown(event: KeyboardEvent) {
  if (!props.member) return
  if (event.key === 'Escape') {
    event.stopPropagation()
    emit('close')
  }
  if (event.key === 'Tab' && mobile.value) {
    const items = panel.value?.querySelectorAll<HTMLElement>('button:not(:disabled),a[href],[tabindex="0"]')
    if (!items?.length) return
    const first = items[0]!,
      last = items[items.length - 1]!
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }
}
function moveTab(event: KeyboardEvent, index: number) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? tabs.length - 1
        : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
  tab.value = tabs[next]!
  void nextTick(() => panel.value?.querySelectorAll<HTMLElement>('[role="tab"]')[next]?.focus())
}
watch(
  () => props.member?.id,
  async (id, old) => {
    if (id) {
      if (!old) trigger = document.activeElement as HTMLElement
      tab.value = '基本信息'
      resize()
      await nextTick()
      panel.value?.querySelector<HTMLElement>('[data-close-detail]')?.focus()
    } else {
      setInert()
      await nextTick()
      if (trigger?.isConnected) trigger.focus()
    }
  },
)
onMounted(() => {
  resize()
  window.addEventListener('resize', resize)
  document.addEventListener('keydown', keydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  document.removeEventListener('keydown', keydown)
  const root = document.getElementById('app')
  if (root && inertOwned) root.inert = rootWasInert
})
</script>
<template>
  <Teleport to="body">
    <Transition name="detail-backdrop"
      ><button
        v-if="member && mobile"
        class="member-detail-backdrop"
        tabindex="-1"
        aria-label="关闭成员详情"
        @click="emit('close')"
    /></Transition>
    <Transition name="member-detail">
      <aside
        v-if="member"
        id="member-detail-panel"
        ref="panel"
        class="member-detail-panel"
        role="dialog"
        :aria-modal="mobile"
        aria-labelledby="member-detail-title"
      >
        <header class="detail-header">
          <h2 id="member-detail-title">成员详情</h2>
          <button class="icon-button" data-close-detail aria-label="关闭成员详情" @click="emit('close')">
            <AppIcon name="close" :size="18" />
          </button>
        </header>
        <div class="detail-scroll">
          <div class="detail-profile">
            <AvatarMark :name="member.name" :size="46" tone="solid" />
            <div class="detail-profile-copy">
              <div class="profile-title">
                <h2>{{ member.name }}</h2>
                <StatusBadge
                  :text="statusLabels[member.status]"
                  :tone="
                    member.status === 'active'
                      ? 'success'
                      : member.status === 'suspended'
                        ? 'danger'
                        : 'neutral'
                  "
                />
              </div>
              <MemberRoleTags :ids="member.roleIds" />
            </div>
            <div class="profile-actions">
              <button class="btn" aria-label="编辑成员资料" @click="emit('action', 'edit', member)">
                <AppIcon name="edit" :size="13" />编辑</button
              ><button class="btn more-button" aria-label="成员详情更多操作" @click="emit('more', member)">
                <AppIcon name="more" :size="15" />
              </button>
            </div>
          </div>
          <div class="detail-tabs" role="tablist" aria-label="成员详情栏目">
            <button
              v-for="(item, index) in tabs"
              :id="`detail-tab-${index}`"
              :key="item"
              :class="{ active: tab === item }"
              role="tab"
              :aria-selected="tab === item"
              :tabindex="tab === item ? 0 : -1"
              aria-controls="member-tab-content"
              @click="tab = item"
              @keydown="moveTab($event, index)"
            >
              {{ item }}
            </button>
          </div>
          <div
            id="member-tab-content"
            role="tabpanel"
            :aria-labelledby="`detail-tab-${tabs.indexOf(tab)}`"
            tabindex="0"
          >
            <MemberDetailOverview v-if="tab === '基本信息'" :member="member" @logs="tab = '操作日志'" />
            <div v-else-if="tab === '角色权限'" class="tab-body">
              <div class="row-between">
                <h3>已分配角色</h3>
                <button class="btn-link" @click="emit('action', 'role', member)">调整角色</button>
              </div>
              <article
                v-for="role in store.roles.filter((r) => member!.roleIds.includes(r.id))"
                :key="role.id"
                class="permission-section"
              >
                <MemberRoleTags :ids="[role.id]" />
                <p>{{ role.description }}</p>
                <ul>
                  <li v-for="permission in role.permissions" :key="permission">
                    <AppIcon name="check" :size="13" />{{ permission }}
                  </li>
                </ul>
              </article>
            </div>
            <div v-else-if="tab === '数据权限'" class="tab-body">
              <div class="row-between">
                <h3>当前数据范围</h3>
                <button class="btn-link" @click="emit('action', 'role', member)">调整范围</button>
              </div>
              <div class="scope-summary">
                <AppIcon name="database" :size="24" />
                <div>
                  <strong>{{ scopeLabels[member.scope] }}</strong>
                  <p>{{ store.departmentName(member.departmentId) }}</p>
                </div>
              </div>
              <p class="secondary">
                功能权限决定可以执行的操作，数据范围决定可以访问的对象。此处展示当前成员的范围配置。
              </p>
              <div class="notice-box">示例权限仅用于界面预览；实际授权以服务端校验为准。</div>
            </div>
            <div v-else class="tab-body">
              <h3>成员操作记录</h3>
              <ol v-if="memberLogs.length" class="audit-list">
                <li v-for="log in memberLogs" :key="log.id">
                  <strong>{{ log.action }}</strong
                  ><small>{{ log.time }} · {{ log.actor }}</small>
                  <p>{{ log.reason || '界面预览操作' }}</p>
                </li>
              </ol>
              <div v-else class="detail-empty">
                <AppIcon name="file" :size="32" /><strong>暂无操作记录</strong>
                <p>该成员后续的资料与权限变更将在这里展示。</p>
              </div>
            </div>
          </div>
        </div>
        <footer class="detail-footer">
          <AppIcon name="shield" :size="13" />仅展示当前企业授权范围内的成员信息
        </footer>
      </aside>
    </Transition>
  </Teleport>
</template>
<style scoped>
.member-detail-panel {
  position: fixed;
  top: var(--header-height);
  bottom: 0;
  right: 0;
  width: var(--member-detail-width);
  max-width: 100vw;
  display: flex;
  flex-direction: column;
  background: var(--color-surface);
  border-left: 1px solid var(--color-border);
  box-shadow: -6px 0 24px rgb(49 81 121 / 4%);
  z-index: var(--z-member-detail);
}
.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 13px 18px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}
.detail-header h2 {
  font-size: 16px;
}
.detail-scroll {
  padding: 0 18px;
  overflow-y: auto;
  overscroll-behavior: contain;
  flex: 1;
  min-height: 0;
}
.detail-profile {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 22px 0;
  flex-wrap: wrap;
}
.detail-profile-copy {
  flex: 1;
  min-width: 118px;
}
.profile-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.profile-title h2 {
  font-size: 17px;
}
.profile-title :deep(.status-badge) {
  font-size: 10px;
  padding: 1px 5px;
}
.profile-actions {
  display: flex;
  gap: 6px;
  margin-left: auto;
}
.profile-actions .btn {
  height: 30px;
  padding: 0 7px;
  font-size: 11px;
  color: var(--color-primary);
  border-color: var(--color-border);
}
.profile-actions .more-button {
  padding: 0 5px;
}
.detail-tabs {
  display: flex;
  justify-content: space-between;
  border-bottom: 1px solid var(--color-border);
  gap: 8px;
}
.detail-tabs button {
  white-space: nowrap;
  font-size: 12px;
  padding: 8px 0 11px;
  border-bottom: 2px solid transparent;
  color: var(--color-text-secondary);
}
.detail-tabs button.active {
  color: var(--color-primary);
  border-bottom-color: var(--color-primary);
  font-weight: 600;
}
.tab-body {
  padding: 20px 0;
  font-size: 12px;
}
.tab-body h3 {
  font-size: 13px;
}
.tab-body .btn-link {
  font-size: 11px;
}
.permission-section {
  padding: 18px 0;
  border-bottom: 1px solid var(--color-border);
}
.permission-section p {
  color: var(--color-text-muted);
  margin: 10px 0;
}
.permission-section ul {
  list-style: none;
  margin: 12px 0 0;
  padding: 0;
}
.permission-section li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  font-size: 11px;
}
.permission-section .icon {
  color: var(--color-success);
}
.scope-summary {
  padding: 20px 16px;
  margin: 20px 0;
  background: var(--color-success-soft);
  color: var(--color-success);
  border-radius: var(--radius-md);
  display: flex;
  gap: 14px;
  align-items: center;
}
.scope-summary p {
  font-size: 11px;
  margin-top: 4px;
}
.tab-body .notice-box {
  margin-top: 20px;
  font-size: 12px;
}
.detail-empty {
  display: flex;
  align-items: center;
  flex-direction: column;
  gap: 12px;
  text-align: center;
  padding: 42px 12px;
  color: var(--color-text-muted);
}
.audit-list {
  list-style: none;
  padding: 0;
}
.audit-list li {
  padding: 15px 0;
  border-bottom: 1px solid var(--color-border);
}
.audit-list small {
  display: block;
  margin: 5px 0;
  color: var(--color-text-muted);
  font-size: 11px;
}
.audit-list p {
  color: var(--color-text-secondary);
}
.detail-footer {
  padding: 10px 18px;
  flex-shrink: 0;
  border-top: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-text-muted);
  font-size: 10px;
}
.member-detail-enter-active,
.member-detail-leave-active {
  transition:
    transform 0.24s cubic-bezier(0.2, 0.8, 0.2, 1),
    opacity 0.24s;
}
.member-detail-enter-from,
.member-detail-leave-to {
  transform: translateX(100%);
  opacity: 0;
}
.member-detail-backdrop {
  position: fixed;
  inset: var(--header-height) 0 0;
  z-index: calc(var(--z-member-detail) - 1);
  background: var(--color-overlay);
}
.detail-backdrop-enter-active,
.detail-backdrop-leave-active {
  transition: opacity 0.2s;
}
.detail-backdrop-enter-from,
.detail-backdrop-leave-to {
  opacity: 0;
}
@media (max-width: 767px) {
  .member-detail-panel {
    top: 0;
    width: 100%;
    z-index: var(--z-dialog);
  }
  .member-detail-backdrop {
    inset: 0;
  }
  .detail-scroll {
    padding: 0 22px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .member-detail-enter-active,
  .member-detail-leave-active,
  .detail-backdrop-enter-active,
  .detail-backdrop-leave-active {
    transition: none;
  }
}
</style>
