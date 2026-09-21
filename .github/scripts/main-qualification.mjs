import { appendFile, readFile } from 'node:fs/promises'

export function selectAssociatedMergedPr(pulls, mainSha) {
  return (pulls ?? [])
    .filter((pr) => pr.merged_at && pr.merge_commit_sha === mainSha && pr.base?.ref === 'main')
    .sort((a, b) => Number(b.number) - Number(a.number))[0] ?? null
}

export function selectSuccessfulMergeGate(runs, candidateSha) {
  return (runs ?? [])
    .filter((run) =>
      run.name === 'PR Merge Gate' &&
      run.head_sha === candidateSha &&
      run.event === 'pull_request' &&
      run.status === 'completed' &&
      run.conclusion === 'success'
    )
    .sort((a, b) => Number(b.id) - Number(a.id))[0] ?? null
}

async function api(path, token, accept = 'application/vnd.github+json') {
  const response = await fetch('https://api.github.com' + path, {
    headers: {
      Accept: accept,
      Authorization: 'Bearer ' + token,
      'X-GitHub-Api-Version': '2022-11-28',
    },
  })
  if (!response.ok) throw new Error('GitHub API ' + response.status + ': ' + path)
  return response.json()
}

async function writeOutputs(values) {
  if (!process.env.GITHUB_OUTPUT) return
  await appendFile(
    process.env.GITHUB_OUTPUT,
    Object.entries(values).map(([key, value]) => key + '=' + value + '\n').join(''),
  )
}

async function main() {
  const repository = process.env.GITHUB_REPOSITORY
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN
  const mainSha = process.env.MAIN_SHA || process.env.GITHUB_SHA
  if (!repository || !token || !mainSha) throw new Error('Main qualification environment is incomplete')

  const pulls = await api(
    '/repos/' + repository + '/commits/' + encodeURIComponent(mainSha) + '/pulls?per_page=100',
    token,
  )
  const pull = selectAssociatedMergedPr(pulls, mainSha)
  if (!pull) {
    console.error('MAIN_UNTRUSTED_DIRECT_PUSH main=' + mainSha)
    await writeOutputs({
      qualification: 'MAIN_UNTRUSTED_DIRECT_PUSH',
      main_sha: mainSha,
      verified: 'false',
    })
    process.exitCode = 1
    return
  }

  const candidateSha = pull.head?.sha ?? ''
  if (!candidateSha) throw new Error('Associated merged PR has no candidate head SHA')

  const payload = await api(
    '/repos/' + repository + '/actions/runs?event=pull_request&head_sha=' +
      encodeURIComponent(candidateSha) + '&per_page=100',
    token,
  )
  const mergeGate = selectSuccessfulMergeGate(payload.workflow_runs, candidateSha)
  if (!mergeGate) {
    console.error(
      'MAIN_MERGE_GATE_PROOF_MISSING main=' + mainSha +
      ' pr=' + pull.number + ' candidate=' + candidateSha,
    )
    await writeOutputs({
      qualification: 'MAIN_MERGE_GATE_PROOF_MISSING',
      main_sha: mainSha,
      pr_number: pull.number,
      candidate_sha: candidateSha,
      verified: 'false',
    })
    process.exitCode = 1
    return
  }

  const values = {
    qualification: 'MAIN_VERIFIED',
    main_sha: mainSha,
    pr_number: pull.number,
    candidate_sha: candidateSha,
    merge_gate_run_id: mergeGate.id,
    verified: 'true',
  }
  console.log(
    'MAIN_VERIFIED main=' + mainSha +
    ' pr=' + pull.number +
    ' candidate=' + candidateSha +
    ' merge_gate_run=' + mergeGate.id,
  )
  await writeOutputs(values)

  if (process.env.GITHUB_STEP_SUMMARY) {
    const lines = [
      '# Main Qualification',
      '',
      '- State: **MAIN_VERIFIED**',
      '- Main SHA: `' + mainSha + '`',
      '- Merged PR: **#' + pull.number + '**',
      '- Candidate SHA: `' + candidateSha + '`',
      '- PR Merge Gate run: **' + mergeGate.id + '**',
      '',
      'The main tip is verified by proof reuse from the exact pre-merge candidate; full regression is not rerun after merge.',
      '',
    ]
    await appendFile(process.env.GITHUB_STEP_SUMMARY, lines.join('\n'))
  }
}

if (process.argv[1] && import.meta.url === new URL('file://' + process.argv[1]).href) {
  main().catch((error) => {
    console.error(error)
    process.exitCode = 1
  })
}
