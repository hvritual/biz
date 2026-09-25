import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { TrustedSession } from '@/services/runtime/api'
import {
  getMyPersonalProfile,
  isPersonalProfileSession,
  listMyPersonalAvatarOptions,
  personalProfileRequestId,
  personalProfileRuntimeError,
  readPersonalProfileSession,
  samePersonalProfileSession,
  updateMyPersonalAvatar,
  type PersonalAvatarOption,
  type TenantPersonalProfile,
} from '@/services/enterprise/personalProfileRuntime'

export const usePersonalProfileStore = defineStore('personal-profile', () => {
  const profile = ref<TenantPersonalProfile | null>(null)
  const avatarOptions = ref<readonly PersonalAvatarOption[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const ready = ref(false)
  const error = ref('')

  let generation = 0
  let mutation: { signature: string; key: string } | null = null
  let activeRead: { signature: string; promise: Promise<boolean> } | null = null

  function sessionSignature(session: TrustedSession) {
    return JSON.stringify({
      authenticated: session.authenticated,
      actorKind: session.actor_kind ?? '',
      userId: session.user_id ?? '',
      tenantId: session.active_tenant_id ?? '',
      contextVersion: session.context_version ?? 0,
    })
  }

  function clear() {
    generation += 1
    profile.value = null
    avatarOptions.value = []
    loading.value = false
    saving.value = false
    ready.value = false
    error.value = ''
    mutation = null
    activeRead = null
  }

  function accepts(session: TrustedSession, value: TenantPersonalProfile) {
    return (
      isPersonalProfileSession(session) &&
      value.userId === session.user_id &&
      value.tenantId === session.active_tenant_id
    )
  }

  async function load(session: TrustedSession): Promise<boolean> {
    const signature = sessionSignature(session)
    if (activeRead?.signature === signature) return activeRead.promise

    const token = ++generation
    loading.value = true
    error.value = ''
    const promise = (async () => {
      try {
        const [nextProfile, nextOptions] = await Promise.all([
          getMyPersonalProfile(session),
          listMyPersonalAvatarOptions(session),
        ])
        if (token !== generation) return false

        const current = await readPersonalProfileSession()
        if (token !== generation || !samePersonalProfileSession(session, current)) return false
        if (!accepts(session, nextProfile)) return false

        profile.value = nextProfile
        avatarOptions.value = nextOptions
        ready.value = true
        return true
      } catch (caught) {
        if (token === generation) {
          profile.value = null
          avatarOptions.value = []
          ready.value = true
          error.value = personalProfileRuntimeError(caught)
        }
        return false
      } finally {
        if (token === generation) loading.value = false
        if (activeRead?.signature === signature) activeRead = null
      }
    })()
    activeRead = { signature, promise }
    return promise
  }

  async function refresh(session?: TrustedSession | null) {
    const trusted = session ?? await readPersonalProfileSession()
    if (!isPersonalProfileSession(trusted)) {
      clear()
      ready.value = true
      error.value = personalProfileRuntimeError(new Error('请先登录业务账号。'))
      return false
    }
    return load(trusted)
  }

  async function saveAvatar(session: TrustedSession, avatarAssetRef: string) {
    const currentProfile = profile.value
    if (!currentProfile) throw new Error('个人资料尚未从服务端加载。')
    if (!avatarOptions.value.some((option) => option.assetRef === avatarAssetRef)) {
      throw new Error('请选择服务端允许的头像。')
    }

    const stable = await readPersonalProfileSession()
    if (!samePersonalProfileSession(session, stable)) {
      clear()
      throw new Error('会话或当前租户已变化，请重新读取个人资料。')
    }

    const signature = JSON.stringify({
      tenantId: currentProfile.tenantId,
      userId: currentProfile.userId,
      version: currentProfile.version,
      avatarAssetRef,
    })
    if (!mutation || mutation.signature !== signature) {
      mutation = {
        signature,
        key: personalProfileRequestId(currentProfile.tenantId),
      }
    }

    const token = generation
    saving.value = true
    error.value = ''
    try {
      const updated = await updateMyPersonalAvatar(session, currentProfile, avatarAssetRef, mutation.key)
      if (token !== generation) return false
      const after = await readPersonalProfileSession()
      if (token !== generation || !samePersonalProfileSession(session, after) || !accepts(session, updated)) {
        return false
      }
      profile.value = updated
      mutation = null
      return true
    } catch (caught) {
      if (token === generation) error.value = personalProfileRuntimeError(caught)
      throw new Error(personalProfileRuntimeError(caught))
    } finally {
      if (token === generation) saving.value = false
    }
  }

  return {
    profile,
    avatarOptions,
    loading,
    saving,
    ready,
    error,
    clear,
    refresh,
    saveAvatar,
  }
})
