---
name: github-actions-run-errors
description: Use when the user gives a GitHub Actions run number, job name, or failing section and wants you to find the relevant logs, isolate the failing job or test, and explain the real error from GitHub Actions output using the gh CLI.
---

# GitHub Actions Run Errors

Use this skill when the user points at a GitHub Actions run and wants the actual failure, not a generic run summary.

## Inputs to confirm

- Run number
- Repository (`OWNER/REPO`) — if not given, infer with `gh repo view --json nameWithOwner -q .nameWithOwner`
- Optional focus:
  - job name
  - job ID
  - test name
  - section name from the Actions UI

## Prerequisites

Before doing anything, verify `gh` is available:

```bash
gh --version
```

If the command is not found, stop immediately and tell the user:
> `gh` CLI is not installed. Install it from https://cli.github.com and authenticate with `gh auth login` before retrying.

Do not proceed with any other steps until this check passes.

## Workflow

1. Start with the run summary.

Use:

```bash
gh run view <RUN> --repo <OWNER/REPO>
```

If you need machine-readable output and the environment allows it:

```bash
gh run view <RUN> --repo <OWNER/REPO> --json status,conclusion,displayTitle,jobs,url
```

Goal:

- confirm whether the run is still in progress
- find the failed job or the user-named section
- capture the job ID if logs must be fetched directly

2. Fetch the job log, not the whole run log, whenever possible.

Preferred:

```bash
gh run view <RUN> --repo <OWNER/REPO> --job <JOB_ID> --log
```

If `gh run view --log` says the run is still in progress or logs are unavailable, fall back to the job log API:

```bash
gh api repos/<OWNER/REPO>/actions/jobs/<JOB_ID>/logs
```

3. Clean the log before filtering.

`gh run view --log` output has three sources of noise:
- null bytes (corrupt grep output)
- ANSI color escape codes (`[38;5;9m`, `[0m`, etc.)
- tab-separated prefix on every line: `<job name>\t<step name>\t<timestamp> <content>`

Strip all three:

```bash
gh run view <RUN> --repo <OWNER/REPO> --job <JOB_ID> --log 2>&1 \
  | tr -d ‘\000’ \
  | sed $’s/\033\[[0-9;]*m//g’ \
  | awk -F’\t’ ‘{print $3}’
```

4. **Start from the tail.** The failure summary is always near the end of the log. Jump there first:

```bash
... | tail -300
```

If that confirms a failure, search backwards for the root cause. Only scan the full log if the tail doesn’t have enough context.

5. Filter aggressively if the tail is not enough.

Start with the user’s target if they gave one:

```bash
... | rg -n -C 8 "<section name>|<test name>|<job name>"
```

Then use failure patterns:

```bash
... | rg -n -C 8 \
  "Summarizing [0-9]+ Failures|\[FAIL\]|\[FAILED\]|FAIL!|Timed out after|panic:|fatal:|Unexpected error|Full Stack Trace|Process completed with exit code|object has been modified|Conflict"
```

If a tool call returns "output too large" and saves to a file path, run `tail -300 <saved_path>` on that file — do not re-run the full log fetch.

6. Prefer the final failure block over progress reports.

In Ginkgo logs especially:

- progress reports often show where the test is currently stuck, not the final failure site
- the most reliable block is near `Summarizing X Failures`
- the final `[FAIL]` line usually points to the real local file and line

7. Correlate the failing file and line locally.

Once the log points to a local file like `test/e2e/ytsaurus_controller_test.go:2522`, open that file locally and inspect the assertion around that line. This is often faster than rereading more log output.

8. Separate signal from noise.

Call out:

- the failing job and job ID
- the failing spec/test/step
- the exact file:line from the failure block
- the concrete error text
- the likely root cause

Also explicitly label noisy but non-root-cause messages when relevant, for example:

- readiness probe warnings
- retries
- secondary cleanup failures
- progress-report stack traces that are superseded by the final failure summary

## Fast paths

### User gives only a run number

1. Infer repo: `gh repo view --json nameWithOwner -q .nameWithOwner`
2. Run `gh run view <RUN> --repo <OWNER/REPO>` — identify failed jobs and their IDs
3. Fetch the failed job log, clean it, pipe to `tail -300`
4. If the failure summary is visible, done — map it to the local source line
5. Otherwise grep with failure patterns on the full log

### User gives run number and a section name

1. Find the matching job in the run summary
2. Open only that job log
3. Filter by the section name plus failure patterns
4. Confirm the final `[FAIL]` location

### User gives a run number but logs are still unavailable

1. Use `gh run view` to get the job ID
2. Fetch `gh api repos/<OWNER/REPO>/actions/jobs/<JOB_ID>/logs`
3. Filter live logs for the stuck step or failure pattern
4. State clearly that the run is still in progress if the failure is not yet final

## Output format

Keep the response compact and concrete:

- run URL or run number
- failed job name and job ID
- failing spec/test/step
- failing file:line
- exact error in one or two lines
- likely cause

If the job is still running, say that explicitly and distinguish:

- current stuck step
- final confirmed failure, if any

## Good defaults

- Prefer `gh` over browser links.
- Prefer job logs over full run logs.
- Prefer `rg` with context over reading the full log.
- Prefer the final failure summary over intermediate progress output.
- Prefer local source inspection once you have a file:line from the log.
