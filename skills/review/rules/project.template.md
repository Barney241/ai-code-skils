# Project rules — TEMPLATE

Copy this to `project.md`, fill it in, and add `"project.md"` to `always_load` in `rules.json`.

This file is where everything true of **your** repository lives. The other rule documents stay
portable precisely because this one exists. Delete every section you have nothing real to put in —
an empty heading invites an agent to invent content for it.

Every entry should be a fact with a number or a command behind it. "We prefer small functions" is
not a rule; "the linter enforces a cyclomatic limit of 15, so do not restate it" is.

---

## P-1 · What the deterministic tooling already covers

List the checks that run automatically, so the reviewer knows what *not* to report (core rule D-2).

```
Formatter:    <e.g. gofumpt, prettier, black -- on save or in CI?>
Linters:      <the actual enabled list, not "we use a linter">
Type checker: <language / strictness>
Test command: <how tests run, and what is excluded>
```

**Are test files linted?** `<yes / no>` — if no, lint-class defects in test files are in scope for
the tests lane (T-7), because nothing else reports them.

---

## P-2 · Invariants where the naive check is wrong

The rules that produce false positives when applied literally. State the measured counts; a rule
without them cannot be applied.

Template:

> **`<the rule as usually stated>`** — but `<pattern>` appears `<N>` times legitimately, because
> `<reason>`. The rule actually covers `<narrower case>` only.

Example shape:

> **"Always use the model builder, never the setter"** — the codebase has 230 setter calls against
> 117 builder calls, and the hits are legitimate column-level updates. The rule covers whole-row
> writes only.

> **"Always use the shared logger, never the raw one"** — 151 existing files import the raw logger.
> Flag only a **new** import in a changed file.

---

## P-3 · Layer names and reference implementations

Map the generic names in `clean-arch.md` onto this repository.

```
usecase layer   -> <path pattern>
domain types    -> <path pattern>
delivery layers -> <list them; CA-6 needs to know every sibling>
storage layer   -> <path pattern>
composition root-> <the files where slices get wired, one per entrypoint>
entrypoints     -> <api / cron / queue / worker -- CA-8 checks all of them>
```

Reference implementations to diff against: `<paths that are known good>`

---

## P-4 · Risk paths that force `deep` mode

Paths where a mistake is expensive and hard to reverse. Replaces `risk_paths` in `rules.json`.

```
<schema migrations>
<wire contracts / event definitions>
<routing tables>
<anything billed per call>
<CI definitions>
```

---

## P-5 · Billed and rate-limited calls

Cost is a defect category (D-8). It needs local prices to be checkable.

| Call | Price | Guard |
|---|---|---|
| `<method>` | `<$ per call>` | `<how a new call site is gated, if at all>` |

A suppression that waives the guard must state the expected calls per day. A waiver without that
multiplication is itself a finding.

---

## P-6 · Mechanically checkable test conventions

The commands from T-6. Run each once against a known tree and record the expected count, so a
silently-empty match is obvious.

```bash
# <what this enforces>            (expect ~<N> files scanned)
<command>
```

Watch for `**` globs: `globstar` is off by default in bash, so `dir/**/*_test.go` matches a single
directory level. Prefer `find dir -name '*_test.go' -exec grep ... {} +`.

---

## P-7 · Known expensive defect classes

The failure modes this repository actually produces, ideally with counts from your own history.
This is the highest-value section and the one only you can write.

```
<e.g. cross-service contract drift -- N of M fixes over the last year>
<e.g. partial updates overwriting stored fields>
<e.g. a slice wired into one entrypoint but not another>
```
