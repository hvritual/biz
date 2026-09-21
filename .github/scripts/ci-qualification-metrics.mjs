import { appendFile } from 'node:fs/promises'

async function api(path, token) {
  const response = await fetch('https://api.github.com' + path, {
    headers: {
      Accept: 'application/vnd.github+json',
      Authorization: 'Bearer ' + token,
      'X-GitHub-Api-Version': '2022-11-28',
    },
  })
  if (!response.ok) throw new Error('GitHub API ' + response.status + ': ' + path)
  return response.json()
}

function durationMs(start, end) {
  if (!start || !end) return 0
  return Math.max(0, Date.parse(end) - Date.parse(start))
}

function average(values) {
  if (!values.length) return 0
  return Math.round(values.reduce((sum, value) => sum + value, 0) / values.length)
}

async function main() {
  const repository = process.env.GITHUB_REPOSITORY
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN
  const candidate = process.env.CANDIDATE_SHA
  const prNumber = Number(process.env.PR_NUMBER || 0)
  if (!repository || !token || !candidate || !prNumber) throw new Error('CI metrics environment is incomplete')

  const pull = await api('/repos/' + repository + '/pulls/' + prNumber, token)
  const headBranch = pull.head?.ref ?? ''
  const payload = await api('/repos/' + repository + '/actions/runs?event=pull_request&branch=' + encodeURIComponent(headBranch) + '&per_page=100', token)
  const runs = payload.workflow_runs ?? []
  const belongsToPr = (run) => (run.pull_requests ?? []).some((item) => Number(item.number) === prNumber) || run.head_branch === headBranch

  const prRuns = runs.filter(belongsToPr)
  const current = prRuns.filter((run) => run.head_sha === candidate)
  const stale = prRuns.filter((run) => run.head_sha !== candidate)
  const staleCancelled = stale.filter((run) => run.conclusion === 'cancelled')
  const queueWait = current.filter((run) => run.run_started_at).map((run) => durationMs(run.created_at, run.run_started_at))
  const execution = current.filter((run) => run.run_started_at && run.updated_at).map((run) => durationMs(run.run_started_at, run.updated_at))

  const metrics = {
    candidate_sha: candidate,
    pr_number: prNumber,
    workflow_fanout_current_candidate: current.length,
    stale_runs_observed: stale.length,
    stale_runs_cancelled: staleCancelled.length,
    average_runner_wait_ms: average(queueWait),
    average_execution_ms: average(execution),
  }

  console.log('CI_QUALIFICATION_METRICS=' + JSON.stringify(metrics))
  if (process.env.GITHUB_STEP_SUMMARY) {
    const lines = [
      '## CI qualification metrics',
      '',
      '- Candidate: ' + candidate,
      '- Current workflow fan-out: **' + metrics.workflow_fanout_current_candidate + '**',
      '- Stale runs observed: **' + metrics.stale_runs_observed + '**',
      '- Stale runs cancelled: **' + metrics.stale_runs_cancelled + '**',
      '- Average runner wait: **' + metrics.average_runner_wait_ms + ' ms**',
      '- Average execution: **' + metrics.average_execution_ms + ' ms**',
      '',
    ]
    await appendFile(process.env.GITHUB_STEP_SUMMARY, lines.join('\n'))
  }
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
