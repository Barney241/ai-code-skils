---
name: watch-pr
description: "Babysit a PR until it is green: poll CI, triage failures, resolve review feedback, push fixes, loop. Use after opening a PR, when asked to watch or babysit one, or when CI goes red on a PR you own."
---

# Watch a PR until green

Poll CI, pick up review feedback, fix or rebut with evidence, push once per round, repeat.

**Green is a state you reach, not a state you declare.** The most common way a PR gets reported as
ready is by checking the wrong signal — see Phase 2.5.

---

## Invariants

- **Fixes go into the PR's branch**, never by switching branches on someone else's checkout.
- **Never force-push, amend, or rewrite history.** Push new commits.
- **Never merge.** Merging is a human decision even when the PR is green.
- **One PR-state reader.** Never hand-write a poll loop — they die with the session and each one
  re-derives the same wrong answer, usually by trusting a status check that is green even when
  the review never ran. Use [`tools/prwatch`](tools/prwatch).
- **One push per fix round.** Every push starts a pipeline and cancels the running one. Batch the
  whole round, then push once.
- **Loop ends** only when every stop condition below is met.

---

## Phase 1 · Establish the target

```bash
PR=<number>
gh pr view "$PR" --json headRefName,headRefOid,url,state,isDraft,mergeable
```

Confirm you have a checkout of that branch to make fixes in. Pin nothing else — the head moves, and
state should be re-read rather than remembered.

## Phase 2 · The poll loop

**Use the tool, not a hand-written loop.** [`tools/prwatch`](tools/prwatch) computes the whole state
as one JSON document with a single `verdict` to branch on. One tick is one call:

```bash
go run ./tools/prwatch status "$PR"
```

Build it once if you prefer: `go build -o ~/.local/bin/prwatch ./tools/prwatch`. It needs Go 1.23+
and an authenticated `gh`; set `PRWATCH_REPO` if the cwd is not the target repository.

| `verdict` | Action |
|---|---|
| `DRAFT` | Mark ready once local gates pass, then trigger review. |
| `CONFLICTED` | Rebase onto the base branch, re-run gates, push. |
| `CI_RED` | Phase 3 — CI triage. |
| `FEEDBACK_PENDING` | Phase 4 — feedback. |
| `CI_PENDING` | Wait and re-tick at the cadence below. |
| `RATE_LIMITED` | Sleep until `coderabbit.next_review_at`. Never guess the wait. |
| `BLOCKED_UNREVIEWED` | Phase 2.5 — spend a review through the ledger. |
| `GREEN` | **Done.** Report the final state and the URL. Do not merge. |

Everything the verdict is derived from is in the same document — CI rollup, bot and human threads
counted separately, whether the bot reviewed *this* head, and the rate-limit window the bot itself
stated. Read that; do not re-derive it.

<details>
<summary>Without Go: an approximation using <code>gh</code> and <code>jq</code></summary>

```bash
gh pr view "$PR" --json isDraft,mergeable,reviewDecision,statusCheckRollup,reviews,reviewThreads,commits \
  --jq '{
    draft: .isDraft,
    mergeable: .mergeable,
    decision: .reviewDecision,
    failing: [.statusCheckRollup[]? | select(.conclusion=="FAILURE")] | length,
    pending: [.statusCheckRollup[]? | select(.status=="IN_PROGRESS" or .status=="QUEUED")] | length,
    unresolved: [.reviewThreads[]? | select(.isResolved==false)] | length,
    last_review: (.reviews | map(select(.author.login | test("coderabbit"; "i"))) | last | .submittedAt),
    head_at: (.commits | last | .commit.committedDate)
  }'
```

`last_review > head_at` is the review gate. This has no quota ledger, so you must track review
spending yourself — which is the part that goes wrong.
</details>

## Phase 2.5 · The review gate

**A green "review" status check does not mean a review happened.** Many bots leave that check green
when the review was skipped for rate limits. The only evidence is a review whose timestamp is later
than the head commit's — `prwatch` reports this as `coderabbit.reviewed_head`.

A PR with zero reviews also has zero unresolved threads, so "nothing failing and nothing open"
passes trivially on a PR nobody looked at. That is the trap this phase exists for.

**Spend review quota only through the ledger:**

```bash
go run ./tools/prwatch ping "$PR"
```

It refuses, with a reason, unless the slot is genuinely free — 30 minutes since the last ping
anywhere, past any stated `next_review_at`, this SHA never pinged, nothing in flight, not a draft.
Quota is a *shared serial resource across all PRs*, not a per-PR budget, which is why it is spent in
one place. **Print the refusal rather than retrying**; a refusal is information.

**Verify the ping landed.** About 90 seconds later, re-read the PR comments. A reply containing
"Action not completed" or "Review rate limited" means it was **refused, not queued** — the review
did not happen. Do not record it as reviewed; back off to `next_review_at`.

After **three honoured attempts** with no review landing, stop and report the PR as **unreviewed**
rather than green.

**Local alternative while you wait.** A review CLI usually draws on a separate quota pool from the
PR bot. Run it on the current diff and fix what it finds; the bot's eventual review then becomes a
second opinion instead of the blocker.

## Phase 3 · CI failure triage

The goal is green CI, not explained-away CI.

Classify the failure first:

| Source | Action |
|---|---|
| **Your own test or your own changed file** | Fix it. No iteration cap. "Flaky" is not a diagnosis you get to make about code you just wrote. |
| **Genuinely unrelated test or infrastructure** | The three-attempt loop below. |

**A failure naming a hardcoded timeout is evidence something never arrived — do not raise the
timeout.** A wait that expired means an item never showed, a hop dropped it, or a dependency never
came up. A longer wait buys a slower red. Find what was missing in the failing window.

### Three-attempt loop, for genuinely unrelated failures

1. Read the logs, form a plausible cause, fix it, verify locally, push. Resume polling.
2. Failure repeats → different angle. Push. Resume.
3. Different angle again. Push. Resume.
4. **Only after three distinct attempts**, stop and comment on the PR with: what failed (job, link,
   error excerpt), the three attempts and why each did not resolve it, and a direct question to the
   human — retry, disable, merge anyway, or another direction.

Re-running CI without a code change **is** attempt one, not a free retry. If a fix makes CI worse,
revert it, count it, try another angle.

**Banned dismissals** — do not author these in comments or commit messages:
"flaky test" without three attempts and evidence · "pre-existing CI issue" · "infrastructure
problem" with no actionable ticket · "transient, a re-run will fix it".

Never use a skip directive, a lint suppression, or a disabled check to get past red CI without
explicit human approval.

## Phase 4 · Feedback resolution

The goal is fixing valid feedback, not writing rationalizations.

| Category | Action |
|---|---|
| Bug, safety, correctness | Fix, commit, push. Do not resolve the thread yourself — let the reviewer resolve on re-review. |
| Style or cleanup | Fix if cheap. Reject only against a documented convention, and cite it. |
| Genuine false positive | Reply with **concrete evidence** — the signature, the command and its output, the behaviour trace. Never a bare "false positive". |
| Genuine scope escalation | Escalate with the actual cost and a specific question. Do **not** auto-defer. |

**Default: fix it.** When in doubt, treat the comment as actionable. A valid finding in a file your
PR already touches is in scope. Do not rebut feedback you have not verified.

**Banned rationalization replies:** "out of scope" when the file is in the diff · "tracked as a
separate refactor" with no ticket link · "pre-existing pattern in N siblings" · "cascades through N
call sites" as a dismissal (cite the count when *escalating*, not when dismissing) · "never nil by
construction" · "tech debt worth a follow-up" with no follow-up.

If your first instinct is one of those, the correct action is fix or escalate — not reply.

### Pushing a fix round

1. Fix **every** finding in the round.
2. Run the project's pre-commit gates.
3. Run the local review CLI on the fix — separate quota, catches what the bot would say.
4. Commit citing the review source: `fix(<scope>): <short fix> (PR feedback round N)`.
5. Push **once**.
6. Request exactly one re-review of the batch, through the ledger:
   `go run ./tools/prwatch ping "$PR"` — never one request per commit, and never by hand.

### When the bot will not re-review

If the head has already been reviewed, most bots refuse a second pass on the same commit. Threads
describing code you have since changed are then stale, and you cannot resolve them yourself. Reply
to each with the specific fix and its evidence — that is what lets the reviewer resolve them, and it
leaves a human reader the reasoning too.

## Cadence

Default **900s**. Typical CI is 10–25 minutes, so one or two ticks per run. Bots usually comment
within 2–5 minutes of a push. Polling faster buys nothing and costs a full context replay per tick.

Give every wait a reason that names the state — "e2e running, others green", not "waiting".

## Stop conditions

Exit and report only when **all** hold:

1. CI complete, nothing failing and nothing pending.
2. A review whose timestamp is later than the head commit's.
3. The review decision is not "changes requested".
4. No unresolved threads, from bots or humans.

Condition 2 is the one usually missing, and it is not satisfiable by accident.

Final report:

```
PR #<N> ready.
- CI: <state>
- Review: <decision> (<reviewer>)
- Mergeable: <mergeable>
- URL: <url>
```

**Always include the URL** in every report — final, escalation, or interim. Never just the number.

Do **not** merge. The human merges.

---

## Anti-patterns

- Reporting a PR green when the head was never reviewed. The check lies; read the reviews.
- Hand-writing a poll loop instead of using one state reader.
- Pushing per commit instead of per fix round — each push discards a running pipeline.
- Emitting a progress line per check. Report failures immediately, then one summary.
- Polling faster than the cadence for the same answer.
- Resolving a review thread on the reviewer's behalf.
- Force-pushing or amending to hide an earlier broken commit.
- Marking a failure flaky or infrastructural without three attempts and evidence.
- Merging.
