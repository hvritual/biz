import { expect, type BrowserContext, type Response, type TestInfo } from '@playwright/test'
import { createHash, randomUUID } from 'node:crypto'
import { existsSync, lstatSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs'
import { resolve, sep } from 'node:path'
import { execFileSync } from 'node:child_process'
import type { TrustedSession } from '../../../src/services/runtime/api'
import type { MessageConfiguration, MessageConfigurationReceipt } from '../../../src/services/enterprise/notificationConfigurationRuntime'

export interface NotificationFixture {
  base_url: string; ui_base_url: string
  notification: { email: string; password: string; reader_email: string; reader_password: string; tenant_a: string; tenant_b: string; site_id: string; site_b_id: string; owner_id: string; reader_id: string; contact_id: string }
}
type Identity = { run: string; attempt: string; test: string; tenant: string; site: string }
type Journal = { identity: Identity; creation: MessageConfigurationReceipt | null }
const endpoint = '/v1/tenant/notification/configurations'
const knownNotes = ['browser initial', 'response recovered', 'readback recovered']
function requireThat(value: unknown, reason: string): asserts value { if (!value) throw new Error(`N184_ATTEMPT_${reason}`) }
const equal = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

// Validate the complete set BEFORE any delete. A familiar name/site alone never
// establishes ownership: IDs must come from this test's real create receipt,
// persisted outside Playwright's attempt-specific folder. As in the production
// readback contract, MySQL timestamp rounding is not an identity/version change.
export function ownedConfigurations(data: NotificationFixture, current: readonly MessageConfiguration[], creation: MessageConfigurationReceipt | null): readonly MessageConfiguration[] {
  const n = data.notification
  requireThat(current.length <= 2 && new Set(current.map(row => row.id)).size === current.length, 'UNEXPECTED_ROWS')
  if (!creation) { requireThat(current.length === 0, 'MISSING_CREATE_RECEIPT'); return current }
  requireThat(creation.tenantId === n.tenant_a && creation.receiptId && creation.configurations.length === 2, 'INVALID_CREATE_RECEIPT')
  requireThat(equal(creation.configurations.map(row => row.level).sort(), ['general', 'urgent']), 'INVALID_LEVELS')
  requireThat(new Set(creation.configurations.map(row => row.id)).size === 2, 'DUPLICATE_IDS')
  for (const born of creation.configurations) {
    requireThat(born.id && born.tenantId === n.tenant_a && born.groupId === n.site_id && born.version === '1' && !born.deleted && born.notes === knownNotes[0] && Number.isFinite(Date.parse(born.createdAt)), 'INVALID_BIRTH')
  }
  for (const row of [...creation.configurations, ...current]) {
    requireThat(row.tenantId === n.tenant_a && row.groupId === n.site_id && !row.deleted, 'WRONG_SCOPE')
    requireThat(equal(row.channels, ['in_app']) && row.primaryUserId === n.owner_id && row.secondaryUserId === n.reader_id && equal(row.additionalUserIds, [n.contact_id]), 'FOREIGN_CONFIGURATION')
  }
  for (const row of current) {
    const born = creation.configurations.find(item => item.id === row.id)
    requireThat(born && born.level === row.level && Number.isFinite(Date.parse(row.createdAt)), 'UNOWNED_ID')
    const permittedVersions = row.level === 'urgent' ? ['1', '2', '3'] : ['1']
    requireThat(permittedVersions.includes(row.version) && row.notes === knownNotes[Number(row.version) - 1], 'UNRECOGNIZED_CHANGE')
  }
  return current
}

export function notificationAttempt(data: NotificationFixture, info: TestInfo) {
  const run = process.env.GITHUB_RUN_ID ?? '', attempt = process.env.GITHUB_RUN_ATTEMPT ?? ''
  const n = data.notification, container = `ce12-browser-${run}-${attempt}`
  requireThat(process.env.GITHUB_ACTIONS === 'true' && /^\d+$/.test(run) && /^\d+$/.test(attempt) && process.env.CE12_MYSQL_CONTAINER === container, 'ISOLATED_CI_REQUIRED')
  for (const url of [data.base_url, data.ui_base_url]) {
    const parsed = new URL(url)
    requireThat(parsed.protocol === 'http:' && parsed.hostname === '127.0.0.1' && !parsed.username && !parsed.password && parsed.pathname === '/', 'LOOPBACK_REQUIRED')
  }
  requireThat(n.site_id === 'n184-site-103' && n.site_b_id === 'n184-site-b' && n.owner_id === 'n184-browser-owner' && n.reader_id === 'n184-browser-reader' && n.contact_id === 'n184-browser-contact', 'WRONG_FIXTURE')
  requireThat(n.tenant_a !== n.tenant_b && [n.tenant_a, n.tenant_b].every(id => /^[\w:-]+$/.test(id)), 'INVALID_TENANT')
  const identity: Identity = { run, attempt, test: `${info.testId}:${info.repeatEachIndex}`, tenant: n.tenant_a, site: n.site_id }
  const output = resolve(info.project.outputDir)
  requireThat(output.startsWith(resolve('test-results') + sep), 'OUTPUT_OUTSIDE_TEST_RESULTS')
  mkdirSync(output, { recursive: true })
  const path = resolve(output, `n184-ownership-${createHash('sha256').update(JSON.stringify(identity)).digest('hex')}.json`)
  function sql(query: string) {
    return execFileSync('docker', ['exec', container, 'mysql', '-uroot', '-proot', '--batch', '--skip-column-names', 'biz_ce12_browser', '-e', query], { encoding: 'utf8', timeout: 10000, stdio: ['ignore', 'pipe', 'pipe'] }).trim()
  }
  function verifyDatabase() {
    const observed = sql(`SELECT DATABASE(); SELECT COUNT(*) FROM biz_deviceops_site WHERE (tenant_id='${n.tenant_a}' AND id='${n.site_id}') OR (tenant_id='${n.tenant_b}' AND id='${n.site_b_id}'); SELECT COUNT(*) FROM biz_member_sites WHERE tenant_id='${n.tenant_a}' AND user_id='${n.owner_id}' AND site_id='${n.site_id}';`)
    requireThat(observed === 'biz_ce12_browser\n2\n1', 'DATABASE_FIXTURE_MISMATCH')
  }
  function readJournal(): Journal {
    if (!existsSync(path)) return { identity, creation: null }
    requireThat(!lstatSync(path).isSymbolicLink(), 'SYMLINK_JOURNAL')
    const value = JSON.parse(readFileSync(path, 'utf8')) as Journal
    requireThat(equal(value.identity, identity), 'JOURNAL_IDENTITY_CHANGED')
    return value
  }
  function save(creation: MessageConfigurationReceipt | null) {
    const temporary = `${path}.${randomUUID()}.tmp`
    writeFileSync(temporary, JSON.stringify({ identity, creation }) + '\n', { mode: 0o600, flag: 'wx' })
    renameSync(temporary, path)
  }
  async function headers(context: BrowserContext) {
    const response = await context.request.get(data.base_url + '/auth/session')
    expect(response.status()).toBe(200)
    const s = await response.json() as TrustedSession
    requireThat(s.authenticated && (s.actor_kind === 'user' || s.actor_kind === 'tenant') && s.user_id === n.owner_id && s.active_tenant_id === n.tenant_a && s.csrf_token && Number.isSafeInteger(s.context_version), 'SESSION_MISMATCH')
    return { 'X-CSRF-Token': s.csrf_token, 'X-Biz-Session-Context': JSON.stringify({ actor_kind: s.actor_kind, platform_subject: s.platform_subject ?? '', user_id: s.user_id, active_tenant_id: s.active_tenant_id, context_version: s.context_version }) }
  }
  async function snapshot(context: BrowserContext) {
    const response = await context.request.get(data.base_url + endpoint + '?page=1&page_size=100', { headers: await headers(context) })
    expect(response.status()).toBe(200)
    const result = await response.json() as { tenantId: string; items?: MessageConfiguration[]; total?: string }
    const items = result.items ?? []
    requireThat(result.tenantId === n.tenant_a && Number(result.total ?? 0) === items.length, 'INCOMPLETE_LIST')
    return items
  }
  async function restore(context: BrowserContext) {
    verifyDatabase()
    const journal = readJournal(), rows = ownedConfigurations(data, await snapshot(context), journal.creation)
    const removed: Array<{ id: string; version: string; receiptId: string }> = []
    for (const row of rows) {
      const requestHeaders = await headers(context)
      const response = await context.request.post(data.base_url + endpoint + `/${encodeURIComponent(row.id)}/delete`, {
        headers: { ...requestHeaders, 'Idempotency-Key': `n184-recover-${randomUUID()}` }, data: { expectedVersion: row.version },
      })
      // Conflict or unknown outcome is a preparation failure, never a wider
      // reset or another write. The next attempt can independently read again.
      expect(response.status()).toBe(200)
      const receipt = await response.json() as MessageConfigurationReceipt
      requireThat(receipt.receiptId && receipt.tenantId === n.tenant_a && receipt.configurations.length === 1, 'INVALID_DELETE_RECEIPT')
      const deleted = receipt.configurations[0]!
      requireThat(deleted.id === row.id && deleted.groupId === n.site_id && deleted.tenantId === n.tenant_a && deleted.deleted && deleted.version === String(BigInt(row.version) + 1n), 'DELETE_RECEIPT_MISMATCH')
      const readback = await context.request.get(data.base_url + endpoint + '/' + encodeURIComponent(row.id), { headers: await headers(context) })
      expect(readback.status()).toBe(404)
      expect(await readback.text()).toContain('NOTIFICATION_NOT_FOUND')
      removed.push({ id: row.id, version: deleted.version, receiptId: receipt.receiptId })
    }
    expect(await snapshot(context)).toEqual([])
    expect(sql(`SELECT COUNT(*) FROM biz_notification_configuration_recipients r LEFT JOIN biz_notification_configurations c ON c.id=r.configuration_id AND c.tenant_id=r.tenant_id WHERE r.tenant_id='${n.tenant_a}' AND c.id IS NULL;`)).toBe('0')
    await info.attach('n184-attempt-recovery', { body: Buffer.from(JSON.stringify({ identity, playwright_retry: info.retry, removed, remaining: 0 })), contentType: 'application/json' })
    save(null)
    return removed
  }
  return {
    async prepare(context: BrowserContext) {
      verifyDatabase()
      if (info.retry > 0) await restore(context)
      else { expect(await snapshot(context)).toEqual([]); save(null) }
    },
    async rememberCreate(response: Response) {
      expect(response.status()).toBe(200)
      requireThat(response.request().method() === 'POST' && new URL(response.url()).pathname === '/api' + endpoint, 'NOT_CREATE_RESPONSE')
      const creation = await response.json() as MessageConfigurationReceipt
      ownedConfigurations(data, creation.configurations, creation)
      save(creation)
    },
    restore,
    creation: () => readJournal().creation,
  }
}
