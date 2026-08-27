# prwatch

One source of truth for pull-request state. Every skill that needs to know "is this PR done?" asks
this tool instead of re-deriving it from `gh`.

```bash
go run . status <pr>   # full state as JSON, with one verdict
go run . sweep         # one line per open PR of yours, oldest first
go run . ping <pr>     # spend one review, if the quota ledger allows
go run . budget        # when the next review slot frees up
```

Install it once instead of `go run`:

```bash
go build -o ~/.local/bin/prwatch .
```

## Why a tool and not a prompt

Polling is deterministic work, so it should not cost model attention or context — principle 3 in
the root README. Before this existed, sessions hand-wrote a poller per PR (one job directory held
**eight**, four for the same PR) and each re-derived "done" from raw API calls, wrongly, in the same
two ways:

1. **A green review status check is not a review.** CodeRabbit's `reviews.fail_commit_status`
   defaults to false, so the check reports SUCCESS even when the review was skipped for rate limits.
   Measured on one repo: three open PRs with a green check and zero reviews, and 13 of the previous
   20 merged PRs never reviewed at all.
2. **"No unresolved threads" is not evidence.** Zero reviews also means zero threads, so the
   condition is trivially satisfied on a PR nobody looked at.

`status` answers both correctly: `coderabbit.reviewed_head` is true only when a review exists whose
`submittedAt` is later than the head commit's `committedDate`.

## Verdicts

Branch on `verdict`. It is ranked by what to do next, most blocking first.

| Verdict | Meaning | What the caller does |
|---|---|---|
| `DRAFT` | PR is a draft | nothing expected; mark ready when local gates pass |
| `CONFLICTED` | conflicts with base | rebase before anything else |
| `CI_RED` | a check failed | triage per `watch-pr` Phase 3 (three-attempt rule) |
| `FEEDBACK_PENDING` | unresolved threads, or changes requested | fix per `watch-pr` Phase 4 |
| `CI_PENDING` | checks still running | wait |
| `RATE_LIMITED` | head unreviewed, quota exhausted | sleep until `coderabbit.next_review_at` |
| `BLOCKED_UNREVIEWED` | head unreviewed, quota available | `prwatch ping` |
| `GREEN` | CI green, head reviewed, no feedback | report and stop. **Never merge** |

Feedback outranks a still-running CI because findings can be fixed while checks finish. A draft
outranks everything, because nothing is expected of an unfinished PR.

## The quota ledger

Review bots refill quota hourly, and fair-usage ladders cut that to 1–2/hour at a busy repo's PR
volume. The quota is therefore a **shared serial resource across all PRs**, which is why spending it
belongs in one place rather than in each watcher.

`ping` refuses — printing the reason rather than swallowing it — unless all hold:

- ≥ 30 minutes since the last ping anywhere (`MinPingInterval`)
- past `next_review_at`, when the bot stated one (parsed from its own comment; never guessed)
- this head SHA has never been pinged (bots will not re-review a reviewed commit)
- no ping still in flight (< 15 minutes, not yet landed)
- the PR is not a draft

A refusal is information, not an obstacle. Print it; do not retry around it.

The ledger lives at `<git-common-dir>/prwatch-ledger.json`, so it is shared by every worktree of the
clone and never committed.

**After pinging, verify it landed.** A reply containing "Action not completed" means the ping was
*refused*, not queued — the review did not happen and the slot was not consumed.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PRWATCH_REPO` | `gh repo view` on the cwd | `owner/name` to act on |
| `PRWATCH_BOT` | `coderabbit` | login prefix of the review bot whose reviews count |
| `PRWATCH_PING_COMMENT` | `@coderabbitai review` | what `ping` posts |

`PRWATCH_BOT` gets you review detection for another bot, but **not** quota awareness: the
rate-limit reader parses CodeRabbit's specific comment wording ("Review limit reached", "Next review
available in: …"). For another bot that section is inert, and `ping` falls back to the 30-minute
floor alone.

## Notes

- CI state comes from the GraphQL `statusCheckRollup`, not `actions/runs?branch=`, which behaves
  inconsistently under a fine-grained PAT.
- Human and bot threads are counted separately: never resolve a human's thread on their behalf.
- `sweep` is what makes a multi-PR watcher possible — since quota is spent in one place, PRs can be
  ordered oldest-first. Independent per-PR watchers cannot coordinate a shared bucket.
- The tool never merges and never force-pushes. Merging is a human decision even at `GREEN`.

## Requirements

Go 1.23+ and `gh` authenticated. No other dependencies; the module has no external requirements.
