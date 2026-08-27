# review — design notes

Why this skill is shaped the way it is, and what it was tested against.

---

## The problem it was built for

The common agentic-review setup fans out several "reviewer personas" over a diff and treats
agreement between them as confidence. Inspecting one such setup showed all thirteen reviewer slots
resolving to **a single agent definition with one generalist checklist**. Seven adjectives over one
prompt do not produce seven independent opinions — they produce *correlated* errors, and a vote
then reads that correlation as confirmation.

Two consequences follow, and both were observable:

- **Recall-maximizing with no precision gate.** Nothing removed a finding for being wrong, so the
  cost of inventing one was zero.
- **Nothing was measured.** With no shared schema and no durable log, there was no way to ask
  whether a review had ever found anything real.

## What replaces it

**Fan out over the work, not over personas.** Lanes activate on what the diff actually touches, and
each is given a boundary it must not cross. The blind-spot lane runs last, sees the others' output,
and is explicitly told that returning nothing is a good outcome.

**A verifier instead of a vote.** A separate, cheaper agent sees only the diff and the findings —
less context than the reviewer, deliberately — and may delete only what the diff *disproves*. Since
its errors are asymmetric (a deleted true finding is gone silently; a kept false one costs seconds),
the prompt is built entirely around that asymmetry. See
[`assets/verifier-prompt.md`](assets/verifier-prompt.md).

**Anchors, not line numbers.** Every finding quotes the added lines it refers to, and the line
number is derived by matching that text within the subject file. Line numbers are hallucinated
constantly; quoted source is not.

**One schema, one log.** [`assets/finding-schema.json`](assets/finding-schema.json), appended to a
log outside the worktree. Without it precision cannot be computed, and without that the rules never
improve.

---

## Evidence

Run end to end against a real merged commit (6 files, +220/−7, a cache-invalidation change):
**7 findings, all 7 survived verification** — 1 major, 5 minor, 1 info, across three lanes.

What each stage actually contributed:

- **Anchoring caught a mislabel.** One finding anchored to unchanged context while claiming
  `scope: new`. It was re-anchored to the added line that made it load-bearing and relabelled
  `pre-existing` — corrected, not dropped. The mechanical check found it; reading would not have.
- **Per-file anchoring caught an off-by-one.** A lane reported a symbol's declaration line; the
  anchor resolved one line lower, at the statement it actually quoted.
- **A predicted false positive was correctly declined.** A lane chose not to flag a nil-safety doc
  comment because the function directly above carried the identical line — local convention, not
  redundancy.
- **The lanes found a withheld finding independently.** One finding was deliberately kept out of
  the lane prompts to see whether they would reach it unprimed. The clean-arch lane filed it with
  language-server evidence naming all three call sites.
- **The verifier was calibrated, not assumed.** Two fabricated findings mixed into a real set were
  both deleted on the correct ground with the disproving line named, while all real findings were
  kept. See [`assets/calibration.md`](assets/calibration.md).

## Defects the rules had, found by running them

Turning the same pipeline on its own rule documents, across five review rounds, found seventeen
real defects. The two most instructive were commands that read correctly and verified nothing:

- A test-convention scan used `dir/**/*_test.go`. With `globstar` off — the bash default — that
  matched **one** file where `find` reports 543. The rule had been passing vacuously.
- A constructor check used `ast-grep 'if $X != nil'`, the **inverse** of the nil guard it claimed to
  verify, and counted unrelated `err` guards as satisfying it.

The first replacement for the second bug was itself wrong: it matched field *names* by suffix
(`Port|Repo|Store`) and returned zero fields against a reference implementation whose fields are
called `store`, `repair`, `nowFn` — reproducing the silent pass it was meant to remove. This is why
[`skills/README.md`](../README.md) insists every VERIFY command be run once against a real tree with
the count checked.

Three others were contradictions that let two reviewers reach opposite verdicts on the same file —
a scope rule and its exception stated as peers, a rule mandating findings about *missing* comments
while the output contract demanded a verbatim quote, and two documents setting different bars for
comment density.

---

## Sources

The design borrows deliberately. Where an idea came from someone else, it is theirs:

- **[`alibaba/open-code-review`](https://github.com/alibaba/open-code-review)** — the strongest
  influence. A hybrid of a deterministic pipeline and an LLM agent, staged as
  select → group → plan → review → anchor → verify → record, explicitly precision-first with recall
  traded away, at roughly a ninth the tokens of a general-purpose agent. The verbatim-anchor
  technique and the shape of the critic pass both come from here.
- **[CodeRabbit](https://coderabbit.ai)** — the three-axis finding taxonomy
  (category · severity · **effort**), publishing the verification transcript alongside the finding,
  and carrying learnings with provenance. The effort axis is the one most tools omit and the one
  that makes a long review triageable.
- **[PostHog's ReviewHog](https://posthog.com)** — the lane topology: parallel perspectives, then a
  single blind-spot pass conditioned on what the others found, then one validation judge. Also the
  framing that *an empty sweep is a valid, good outcome*, which is what makes precision-first
  behaviour socially possible.
- **Research on agentic code review context** — benchmarks in this area report a "contextual
  backwardness" effect, where naively retrieved repository context *degrades* review quality for
  most languages, and agents exhibit tunnel vision on the retrieved slice. This is why grouping here
  is deterministic and context is scoped per group rather than maximised.
- **Code graph models for repository understanding** (e.g. *Code Graph Model*,
  [arXiv:2505.16901](https://arxiv.org/abs/2505.16901)) — evaluated and deliberately **not** adopted.
  A language server's `references` and `implementation` queries already supply the call and
  implements edges a code graph would provide, at a fraction of the complexity and with no index to
  keep fresh. The graph is worth revisiting only if those queries stop being enough.

The failure analysis of persona fan-out, the anchoring rules, the calibration procedure, and the
project/portable split are from running this against a production monorepo.
