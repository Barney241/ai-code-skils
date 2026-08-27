# ai-code-skils

Portable skills for AI coding agents — written once, usable from Claude Code, Cursor, Codex,
Copilot, Aider, Windsurf, or a plain chat window.

A *skill* here is a directory of markdown that tells an agent how to do one job properly. No
runtime, no dependencies, no API keys. The agent reads it and follows it.

---

## Principles

These are the rules the skills in this repo are built on. They are here because each one was
learned by watching an agent get it wrong.

**1. Precision over recall.** A false finding costs the reader's trust, and trust is what makes
the *next* finding get read. An empty result is a valid, good outcome; padding is not. Recall is
deliberately traded away — a reviewer that reports nine real problems and one invention is worse
than one that reports six real problems and nothing else.

**2. Evidence before claims.** Never infer behaviour from a name, an import, or a plausible
story. Run the command; cite what it returned. A claim about anything with state — a call site,
a config value, a row, a CI result — is re-established at the moment it is made, not recalled.

**3. Deterministic work does not need a model.** Selecting files, matching an anchor, counting
coverage, computing a hash — if a shell command decides it, a command decides it. Model
attention is the scarce resource; spend it on judgment.

**4. Error costs are asymmetric, so filters must be too.** Discarding a true finding is silent
and permanent — nobody learns it existed. Keeping a false one costs a few seconds of reading.
Any stage that removes findings must be built around that asymmetry, and must be allowed to
delete only on grounds it can *prove*, never on "I couldn't verify this" or "seems low value".

**5. Verify the verifier.** A filter that approves everything and a filter that works produce
identical output when the input is clean. Calibrate with deliberately-planted bad inputs, or you
do not know which one you have.

**6. Anchor to content, not coordinates.** Line numbers are hallucinated confidently and
constantly. Quoted source text is not. Every finding carries the verbatim lines it refers to, and
the line number is *derived* from matching that text — never asserted.

**7. One contract, or you cannot count anything.** When four tools each invent their own severity
vocabulary, nothing can be aggregated, so nothing improves. Fix the schema first; measurement is
what turns opinion into evidence.

**8. Portable core, local traps.** Universal rules ship in the skill. The things that are only
true of *your* repository — the linter set, the ORM idiom, the one function everybody misuses —
live in a project file you own and the skill loads. Mixing them makes the skill unshippable.

**9. Fan out over the work, not over personas.** Seven "reviewers" backed by one prompt with
seven adjectives produce *correlated* errors, and agreement between them then reads as
confirmation. Split on what the change actually touches instead, and give each lane a boundary it
must not cross.

**10. Surface or escalate — never dismiss.** "Out of scope", "pre-existing pattern", "tracked
separately" with no ticket, "cascades through N call sites" — these are rationalizations, not
judgments. A reviewer surfaces; a human decides whether the surface is worth fixing.

**11. Subscription-safe by construction.** Everything runs in the session you are already paying
for. No skill here requires a metered API key, a CI runner, or a webhook. If a workflow only
works by billing per push, it does not belong in this repo.

---

## Skills

| Skill | What it does |
|---|---|
| [`review`](skills/review) | Precision-first code review: `select → group → plan → review → anchor → verify → record`, with path-scoped rule docs and a fact-checking pass that can only delete what the diff disproves. |
| [`open-pr`](skills/open-pr) | Owns the step from committed to watched: preflight, issue linking, a local review pass *before* the push, then a draft PR handed straight to `watch-pr`. |
| [`watch-pr`](skills/watch-pr) | Babysits a PR until it is genuinely green: CI triage with a three-attempt cap on unrelated failures, feedback resolution with banned rationalizations, one push per fix round. Ships [`prwatch`](skills/watch-pr/tools/prwatch), a Go tool that computes PR state and one verdict, and rations review-bot quota through a ledger. |

The three chain: `review` → `open-pr` → `watch-pr`, with no user-gated pause between them. Each is
usable on its own.

More to come. The layout below is built so adding one is mechanical.

---

## Install

```bash
./install.sh review /path/to/your/repo
```

That copies the skill to `<repo>/.agents/skills/review/` and creates the
`<repo>/.claude/skills/review` symlink so Claude Code registers it. Both locations are the
convention: `.agents/` is the tool-neutral source of truth, `.claude/skills/` is the symlink that
makes Claude Code see it.

Other flags:

```bash
./install.sh --list                      # show available skills
./install.sh --all /path/to/repo         # install everything
./install.sh --link review /path/to/repo # symlink instead of copy, to track upstream
./install.sh --uninstall review /path/to/repo
```

`--link` points `.agents/skills/<name>` at this checkout, so `git pull` here updates every repo
you installed into. Use it for your own machine; use the default copy when the skill needs
repo-specific edits.

For anything that is not Claude Code, see [`adapters/`](adapters) — the same markdown works, only
the file it has to live in changes.

---

## Layout

```
skills/<name>/
  SKILL.md              required — frontmatter + the instructions the agent follows
  README.md             optional — rationale, evidence, design notes for humans
  rules/                optional — path-scoped rule documents the skill loads
  assets/               optional — prompts, schemas, checklists
  tools/                optional — real programs the skill calls
install.sh              installs a skill into a target repo
adapters/               how to load a skill in each AI system
```

`tools/` follows from principle 3: deterministic work should not cost model attention. Polling an
API and computing a verdict is a program's job, so `watch-pr` ships one rather than describing a
loop for the agent to run by hand.

**Dependencies:** the skills themselves are plain markdown and need nothing. `prwatch` is the only
program in the repo — Go 1.23+ and an authenticated `gh`, both optional, with a documented `gh`-only
fallback if you would rather not build it.

A skill is valid if `SKILL.md` exists and opens with YAML frontmatter carrying `name` and
`description`. Everything else is convention. See [`skills/README.md`](skills/README.md) for the
full contract and how to add one.

---

## Why this exists

Most agent instructions are written as prose in one enormous file, are never measured, and drift
until nobody trusts them. The skills here are built to the opposite standard: each rule states
what must be true, the command that establishes it, and the case where flagging it would be
wrong. That structure is what makes a finding checkable instead of merely confident.

---

## Sources

These ideas are not original to this repo, and the borrowing is deliberate. Each skill's README
credits its own influences in detail; the ones that shaped the principles above:

- **[`alibaba/open-code-review`](https://github.com/alibaba/open-code-review)** — a hybrid of a
  deterministic pipeline and an LLM agent, staged and explicitly precision-first, at a fraction of
  a general agent's tokens. Principles 1, 3 and 6 come from studying it.
- **[CodeRabbit](https://coderabbit.ai)** — the category · severity · **effort** taxonomy, and
  publishing the verification transcript next to the finding. Principle 7.
- **[PostHog's ReviewHog](https://posthog.com)** — parallel lanes, then a blind-spot pass
  conditioned on their output, then a validation judge; and the framing that an empty sweep is a
  good result. Principles 1 and 9.
- **Agentic code-review benchmarks** — the reported "contextual backwardness" effect, where naively
  retrieved repository context *degrades* review quality for most languages. Principle 8, and the
  reason grouping here is deterministic and context is scoped per group.
- **Code graph models for repository understanding** (e.g. *Code Graph Model*,
  [arXiv:2505.16901](https://arxiv.org/abs/2505.16901)) — evaluated and deliberately not adopted; a
  language server already answers the call-and-implements questions a graph would, with no index to
  keep fresh.

Anything not credited — the persona-correlation analysis, the anchoring rules, the calibration
procedure, the portable/project split — came from running these skills against a production
monorepo and watching what broke.

## Contributions

This is a personal repository, public so it can be read and forked — but **not open to pull
requests**, and issues are not tracked. Fork it and make it yours; that is the intended use.

If you do fork it, the one rule worth keeping is the bar in
[`skills/README.md`](skills/README.md): every rule needs `ASSERT`, `VERIFY` and `DO NOT FLAG`, and
every `VERIFY` command must be run once against a real repository to prove it matches something. A
rule whose command silently matches nothing reports clean forever, which is worse than no rule.

## License

MIT — see [LICENSE](LICENSE). Use it, fork it, ship it in your own tooling.
