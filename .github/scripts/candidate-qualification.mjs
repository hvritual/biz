import { appendFile, readFile, writeFile } from 'node:fs/promises'

const ansi = /\u001b\[[0-9;]*m/g
const failureHeadline = /(?:Error:\s*expect|AssertionError|panic:|--- FAIL:|##\[error\])/i
const testLocation = /((?:web\/|internal\/|integration\/|e2e\/|tests\/)?[A-Za-z0-9_.\/-]+\.(?:spec|test)\.(?:ts|tsx|js|mjs|cjs)|[A-Za-z0-9_.\/-]+_test\.go):(\d+)(?::\d+)?/g
const diagnosticLocation = /((?:web|src|internal|integration|e2e|tests)\/[A-Za-z0-9_.\/-]+\.(?:vue|ts|tsx|js|mjs|cjs|go|json)):(\d+)(?::\d+)?:\s*(.+)$/

export function latestRequiredRuns(runs, required) {
  const selected = new Map()
  for (const run of runs) {
    if (!required.includes(run.name)) continue
    const current = selected.get(run.name)
    if (!current || Number(run.id) > Number(current.id)) selected.set(run.name, run)
  }
  return selected
}

function normalizedHeadline(log) {
  return log
    .replace(ansi, '')
    .split('\n')
    .map((line) => line.trim())
    .find((line) => failureHeadline.test(line))
    ?.replace(/^.*?Error:\s*/, 'Error: ')
    .replace(/\s+/g, ' ')
    .slice(0, 180) ?? 'failed'
}

export function extractFailureSignature(log, fallback = 'workflow failure') {
  const clean = String(log ?? '').replace(ansi, '')
  const matches = [...clean.matchAll(testLocation)]
  const location = matches.at(-1)
  const headline = normalizedHeadline(clean)
  if (!location) return `${fallback} | ${headline}`
  let path = location[1]
  path = path.replace(/^.*?(?=(?:web|internal|integration|e2e|tests)\/)/, '')
  return `${path}:${location[2]} | ${headline}`
}

function diagnosticRule(message) {
  if (/E2E binds product behavior to engineering copy/i.test(message)) return 'UI-E2E-ENGINEERING-COPY'
  if (/product UI exposes engineering language/i.test(message)) return 'UI-PRODUCT-ENGINEERING-LANGUAGE'
  if (/product template displays a raw backend field|product attribute displays a raw backend field/i.test(message)) return 'UI-RAW-BACKEND-DISPLAY'
  if (/backend-returned terms must use backendTermLabel/i.test(message)) return 'UI-BACKEND-TERM-PROJECTION'
  if (/backend errors must use backendErrorFallback/i.test(message)) return 'UI-BACKEND-ERROR-PROJECTION'
  return 'STATIC-DIAGNOSTIC'
}

function diagnosticDetail(message) {
  const label = message.match(/\(([^()]+)\)\s*$/)?.[1]
  return label ?? message.replace(/\s+/g, ' ').slice(0, 180)
}

export function extractFailureSignatures(log, fallback = 'workflow failure') {
  const clean = String(log ?? '').replace(ansi, '')
  const signatures = []
  for (const line of clean.split('\n')) {
    const match = line.match(diagnosticLocation)
    if (!match) continue
    const message = match[3].trim()
    if (!/E2E binds product behavior to engineering copy|product UI exposes engineering language|product (?:template|attribute) displays a raw backend field|backend-returned terms must use backendTermLabel|backend errors must use backendErrorFallback/i.test(message)) continue
    signatures.push(diagnosticRule(message) + ' | ' + match[1] + ':' + match[2] + ' | ' + diagnosticDetail(message))
  }
  return signatures.length ? [...new Set(signatures)] : [extractFailureSignature(clean, fallback)]
}

export function fallbackFailureSignature(steps = []) {
  const failedSteps = steps
    .filter((step) => step.conclusion === 'failure')
    .map((step) => String(step.name ?? '').trim())
    .filter(Boolean)
    .sort()
  return 'CI-LOG-UNAVAILABLE | ' + (failedSteps.join(' + ') || 'failed job')
}
export function groupFailureSignatures(entries) {
  const groups = new Map()
  for (const entry of entries) {
    const current = groups.get(entry.signature) ?? { signature: entry.signature, workflows: new Set(), jobs: new Set() }
    current.workflows.add(entry.workflow)
    current.jobs.add(entry.job)
    groups.set(entry.signature, current)
  }
  return [...groups.values()].map((item) => ({
    signature: item.signature,
    workflows: [...item.workflows].sort(),
    jobs: [...item.jobs].sort(),
  }))
}

export function qualificationSnapshot(required, selected) {
  const missing = required.filter((name) => !selected.has(name))
  const active = required.filter((name) => {
    const run = selected.get(name)
    return run && run.status !== 'completed'
  })
  const failed = required.filter((name) => {
    const run = selected.get(name)
    return run?.status === 'completed' && run.conclusion !== 'success'
  })
  const success = required.filter((name) => selected.get(name)?.conclusion === 'success')
  return { missing, active, failed, success }
}

async function sleep(ms) {
  await new Promise((resolve) => setTimeout(resolve, ms))
}

async function api(path, token, { text = false } = {}) {
  const response = await fetch(`https://api.github.com${path}`, {
    headers: {
      Accept: 'application/vnd.github+json',
      Authorization: `Bearer ${token}`,
      'X-GitHub-Api-Version': '2022-11-28',
    },
    redirect: 'follow',
  })
  if (!response.ok) throw new Error(`GitHub API ${response.status}: ${path}`)
  return text ? response.text() : response.json()
}

async function allRuns(repository, sha, token) {
  const encoded = encodeURIComponent(sha)
  const payload = await api(`/repos/${repository}/actions/runs?head_sha=${encoded}&event=pull_request&per_page=100`, token)
  return payload.workflow_runs ?? []
}

async function candidateHead(repository, prNumber, token) {
  const pull = await api(`/repos/${repository}/pulls/${prNumber}`, token)
  return pull.head?.sha ?? ''
}

async function jobLog(repository, jobId, token, attempts = 5) {
  let lastError
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      return await api('/repos/' + repository + '/actions/jobs/' + jobId + '/logs', token, { text: true })
    } catch (error) {
      lastError = error
      if (attempt < attempts) await sleep(attempt * 1500)
    }
  }
  throw lastError
}
async function failureEntries(repository, failedRuns, token) {
  const entries = []
  for (const run of failedRuns) {
    const payload = await api('/repos/' + repository + '/actions/runs/' + run.id + '/jobs?filter=latest&per_page=100', token)
    const failedJobs = (payload.jobs ?? []).filter((job) => !['success', 'skipped'].includes(job.conclusion))
    if (!failedJobs.length) {
      entries.push({ workflow: run.name, job: 'workflow', signature: 'CI-WORKFLOW-FAILURE | no failed job metadata' })
      continue
    }
    for (const job of failedJobs) {
      try {
        const log = await jobLog(repository, job.id, token)
        for (const signature of extractFailureSignatures(log, fallbackFailureSignature(job.steps ?? []))) {
          entries.push({ workflow: run.name, job: job.name, signature })
        }
      } catch {
        entries.push({ workflow: run.name, job: job.name, signature: fallbackFailureSignature(job.steps ?? []) })
      }
    }
  }
  return entries
}
function markdownSummary({ candidate, required, snapshot, groups, headChanged, qualification }) {
  const lines = [
    '# Candidate Qualification',
    '',
    `- Candidate SHA: \`${candidate}\``,
    `- Required workflows: **${required.length}**`,
    `- Success: **${snapshot.success.length}**`,
    `- Failure: **${snapshot.failed.length}**`,
    `- Active: **${snapshot.active.length}**`,
    `- Missing: **${snapshot.missing.length}**`,
    `- HEAD changed: **${headChanged ? 'true' : 'false'}**`,
    `- Root-cause signatures: **${groups.length}**`,
    `- Qualification: **${qualification}**`,
    '',
  ]
  if (snapshot.missing.length) lines.push('## Missing workflows', '', ...snapshot.missing.map((name) => `- ${name}`), '')
  if (snapshot.active.length) lines.push('## Active workflows', '', ...snapshot.active.map((name) => `- ${name}`), '')
  if (groups.length) {
    lines.push('## Root-cause signatures', '', '| Signature | Workflows | Jobs |', '|---|---|---|')
    for (const group of groups) lines.push(`| ${group.signature.replaceAll('|', '\\|')} | ${group.workflows.join('<br>')} | ${group.jobs.join('<br>')} |`)
    lines.push('')
  }
  return lines.join('\n')
}

async function writeOutputs(values) {
  if (!process.env.GITHUB_OUTPUT) return
  await appendFile(process.env.GITHUB_OUTPUT, Object.entries(values).map(([key, value]) => `${key}=${value}\n`).join(''))
}

async function main() {
  const repository = process.env.GITHUB_REPOSITORY
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN
  const candidate = process.env.CANDIDATE_SHA
  const prNumber = process.env.PR_NUMBER
  if (!repository || !token || !candidate || !prNumber) throw new Error('Candidate qualification environment is incomplete')

  const config = JSON.parse(await readFile(new URL('../candidate-qualification.json', import.meta.url), 'utf8'))
  const required = config.required_workflows
  let finalSnapshot = { missing: [...required], active: [], failed: [], success: [] }
  let headChanged = false

  for (let attempt = 1; attempt <= config.max_attempts; attempt += 1) {
    const liveHead = await candidateHead(repository, prNumber, token)
    if (liveHead !== candidate) {
      headChanged = true
      break
    }
    const selected = latestRequiredRuns(await allRuns(repository, candidate, token), required)
    finalSnapshot = qualificationSnapshot(required, selected)
    console.log(`candidate=${candidate} attempt=${attempt} success=${finalSnapshot.success.length} failure=${finalSnapshot.failed.length} active=${finalSnapshot.active.length} missing=${finalSnapshot.missing.length}`)
    if (!finalSnapshot.missing.length && !finalSnapshot.active.length) {
      const failedRuns = finalSnapshot.failed.map((name) => selected.get(name)).filter(Boolean)
      const groups = groupFailureSignatures(await failureEntries(repository, failedRuns, token))
      const qualification = finalSnapshot.failed.length === 0 ? 'PASS' : 'FAIL'
      const summary = markdownSummary({ candidate, required, snapshot: finalSnapshot, groups, headChanged: false, qualification })
      if (process.env.GITHUB_STEP_SUMMARY) await writeFile(process.env.GITHUB_STEP_SUMMARY, summary)
      console.log(`CANDIDATE_SHA=${candidate}`)
      console.log(`WORKFLOWS=${required.length}`)
      console.log(`SUCCESS=${finalSnapshot.success.length}`)
      console.log(`FAILURE=${finalSnapshot.failed.length}`)
      console.log('ACTIVE=0')
      console.log('HEAD_CHANGED=false')
      console.log(`ROOT_CAUSE_SIGNATURES=${groups.length}`)
      console.log(`QUALIFICATION=${qualification}`)
      await writeOutputs({
        candidate_sha: candidate,
        workflows: required.length,
        success: finalSnapshot.success.length,
        failure: finalSnapshot.failed.length,
        active: 0,
        head_changed: false,
        root_cause_signatures: groups.length,
        qualification,
      })
      if (qualification !== 'PASS') process.exitCode = 1
      return
    }
    await sleep(config.poll_seconds * 1000)
  }

  const qualification = headChanged ? 'STALE' : 'TIMEOUT'
  const groups = []
  const summary = markdownSummary({ candidate, required, snapshot: finalSnapshot, groups, headChanged, qualification })
  if (process.env.GITHUB_STEP_SUMMARY) await writeFile(process.env.GITHUB_STEP_SUMMARY, summary)
  console.log(`CANDIDATE_SHA=${candidate}`)
  console.log(`WORKFLOWS=${required.length}`)
  console.log(`SUCCESS=${finalSnapshot.success.length}`)
  console.log(`FAILURE=${finalSnapshot.failed.length}`)
  console.log(`ACTIVE=${finalSnapshot.active.length}`)
  console.log(`HEAD_CHANGED=${headChanged}`)
  console.log('ROOT_CAUSE_SIGNATURES=0')
  console.log(`QUALIFICATION=${qualification}`)
  await writeOutputs({
    candidate_sha: candidate,
    workflows: required.length,
    success: finalSnapshot.success.length,
    failure: finalSnapshot.failed.length,
    active: finalSnapshot.active.length,
    head_changed: headChanged,
    root_cause_signatures: 0,
    qualification,
  })
  process.exitCode = 1
}

if (process.argv[1] && import.meta.url === new URL(`file://${process.argv[1]}`).href) {
  main().catch((error) => {
    console.error(error)
    process.exitCode = 1
  })
}
