import { computed, onBeforeUnmount, ref } from 'vue'
import type { TrustedSession } from '@/services/runtime/api'
import type { TenantPersonalProfile } from '@/services/enterprise/personalProfileRuntime'
import { commercialRequestId } from '@/services/commercial/platformCommercial'
import {
  completeContactChange, completeTenantDeletion, confirmContactReadback, requestSecurityCode,
  personalSecurityErrorKey, type ContactChangeReceipt, type SecurityChallenge, type SecurityChannel, type SecurityMode,
} from '@/services/enterprise/personalSecurityRuntime'

type Options = {
  currentSession: () => TrustedSession | null
  currentProfile: () => TenantPersonalProfile | null
  applyProfile: (profile: TenantPersonalProfile) => void
  deleted: () => void
}

export function usePersonalSecurity(options: Options) {
  const open = ref(false), busy = ref(false)
  const mode = ref<SecurityMode>('contact'), channel = ref<SecurityChannel>('email')
  const password = ref(''), destination = ref(''), code = ref(''), confirmed = ref(false)
  const error = ref(''), success = ref(false), uncertain = ref(false)
  const challenge = ref<SecurityChallenge | null>(null)
  const receipt = ref<ContactChangeReceipt | null>(null)
  const clock = ref(Date.now()), resendAt = ref(0)
  const seconds = computed(() => Math.max(0, Math.ceil((resendAt.value - clock.value) / 1000)))
  const expired = computed(() => Boolean(challenge.value && Date.parse(challenge.value.expires_at) <= clock.value))
  const confirmationPending = computed(() => receipt.value !== null && !success.value)
  let epoch = 0, session: TrustedSession | null = null, timer: ReturnType<typeof setInterval> | undefined
  let requestKey = '', requestTarget = '', completeKey = ''

  function clearInputs() { password.value = ''; destination.value = ''; code.value = ''; confirmed.value = false }
  function close() {
    epoch++; open.value = false; busy.value = false; challenge.value = null; receipt.value = null
    error.value = ''; success.value = false; uncertain.value = false; session = null
    requestKey = ''; requestTarget = ''; completeKey = ''; resendAt.value = 0; clearInputs()
    if (timer) clearInterval(timer)
    timer = undefined
  }
  function begin(nextMode: SecurityMode, nextChannel: SecurityChannel) {
    close()
    const current = options.currentSession(), profile = options.currentProfile()
    if (!current?.authenticated || current.actor_kind !== 'tenant' || !profile ||
      profile.tenantId !== current.active_tenant_id || profile.userId !== current.user_id) return
    if (nextMode === 'deletion' && !(nextChannel === 'email' ? profile.email : profile.phone)) return
    session = { ...current }; mode.value = nextMode; channel.value = nextChannel; open.value = true
    clock.value = Date.now(); timer = setInterval(() => { clock.value = Date.now() }, 250)
  }
  function active(token: number) { return token === epoch && open.value }
  async function send() {
    if (!session || busy.value || seconds.value > 0 || receipt.value || uncertain.value) return
    const token = epoch, captured = session, target = destination.value.trim()
    if (mode.value === 'contact' && (!password.value || !target || target.includes('*'))) { error.value = 'contactInput'; return }
    const binding = `${mode.value}:${channel.value}:${target}`
    if (!requestKey || requestTarget !== binding) { requestKey = commercialRequestId('personal-security-request'); requestTarget = binding }
    busy.value = true; error.value = ''
    try {
      const result = await requestSecurityCode(captured, mode.value, channel.value, password.value, target, requestKey)
      if (!active(token)) return
      challenge.value = result; destination.value = target; code.value = ''; confirmed.value = false
      resendAt.value = Date.now() + result.resend_after_seconds * 1000
      clock.value = Date.now(); requestKey = ''; completeKey = commercialRequestId('personal-security-complete')
      if (result.delivery_state === 'FAILED') error.value = 'deliveryFailed'
    } catch (caught) {
      if (active(token)) {
        error.value = personalSecurityErrorKey(caught)
        if (error.value === 'rateLimited') resendAt.value = Date.now() + 60000
        if (error.value === 'sessionChanged' || error.value === 'signIn') clearInputs()
      }
    } finally { if (active(token)) { password.value = ''; busy.value = false } }
  }
  async function recover() {
    if (!session || !receipt.value || busy.value) return
    const token = epoch, captured = session, accepted = receipt.value
    busy.value = true; error.value = ''
    try {
      const profile = await confirmContactReadback(captured, accepted)
      if (!active(token)) return
      options.applyProfile(profile); success.value = true
    } catch { if (active(token)) error.value = 'confirmRead' }
    finally { if (active(token)) busy.value = false }
  }
  async function complete() {
    if (!session || !challenge.value || busy.value || receipt.value || uncertain.value) return
    if (expired.value) { error.value = 'expired'; return }
    if (!/^\d{4,10}$/.test(code.value)) { error.value = 'verification'; return }
    if (mode.value === 'deletion' && !confirmed.value) { error.value = 'confirmation'; return }
    const token = epoch, captured = session, proof = challenge.value
    busy.value = true; error.value = ''
    try {
      if (mode.value === 'deletion') {
        await completeTenantDeletion(captured, channel.value, proof, code.value, confirmed.value, completeKey)
        if (!active(token)) return
        close(); options.deleted(); return
      }
      const accepted = await completeContactChange(captured, channel.value, proof, destination.value, code.value, completeKey)
      if (!active(token)) return
      receipt.value = accepted; clearInputs(); requestTarget = ''; requestKey = ''; completeKey = ''
    } catch (caught) {
      if (!active(token)) return
      error.value = personalSecurityErrorKey(caught)
      if (['unavailable', 'invalidResponse', 'consumed'].includes(error.value)) { uncertain.value = true; clearInputs() }
    } finally { if (active(token)) { code.value = ''; password.value = ''; busy.value = false } }
    if (active(token) && receipt.value) await recover()
  }
  onBeforeUnmount(close)
  return { open, busy, mode, channel, password, destination, code, confirmed, error, success, uncertain,
    challenge, receipt, seconds, expired, confirmationPending, begin, close, send, complete, recover }
}
