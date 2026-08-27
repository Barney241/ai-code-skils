# Core review rules — always loaded

Loaded for every file in every review, in addition to any path-matched document.

These are **policy** rules — they govern how the reviewer behaves. The path-scoped documents
(`clean-arch.md`, `tests.md`, `comments.md`) hold **detection** rules, and those use the
`ASSERT → VERIFY → DO NOT FLAG` shape because each one has to be provable against a specific file.
A policy rule has nothing to grep for; do not force it into that shape.

---

## D-1 · Precision over recall

Report only defects likely to be real in the changed code and its reachable context. A false
positive costs the reader's trust, and trust is what makes the next finding get read.

When you are genuinely unsure whether an issue matters, **drop it**. An empty finding list is a
valid, good outcome. Padding is not.

## D-2 · Do not duplicate deterministic tooling

Compilers, type checkers, formatters and linters already run on this code, and they are right more
often than you are about the things they cover. Do not file anything they decide reliably —
unless the diff shows a concrete user-visible consequence they will not express.

List the checks your repository actually runs in `project.md`, so this rule has teeth instead of
being a vague instruction to be quiet.

**The common exception:** many projects exclude test files from linting. Where that is true,
ordinary lint-class defects in test files *are* in scope, because nothing else will ever report
them. Confirm before relying on it.

## D-3 · Evidence before a non-local claim

Do not infer concurrent invocation, attacker control, resource ownership, or an error contract
from a function name or a package import. Establish it first:

```bash
# Who calls this? Who implements it? Use the language server, not grep --
# grep matches every similarly-named symbol and answers the wrong question.
gopls references <abs-path>:<line>:<col>       # Go; equivalent exists for most languages
gopls implementation <abs-path>:<line>:<col>

# Structural search, when you want a code shape rather than a string
ast-grep -p '<pattern>' -l <lang> <dir>

# Literal text -- log strings, config keys, error messages
grep -rn '<text>' --include='<glob>'
```

Language servers generally need an **absolute** path; a relative one can silently return nothing.
The first call is slow while the workspace loads.

A finding at `severity: critical` or `major` without an `evidence` array is rejected by the
coordinator, not published.

## D-4 · Review the change, not the file

Focus on added and modified lines. Deleted code is context only. Do not comment on unchanged code
unless the change makes it wrong — and then say how.

## D-5 · Scope is a label, not a downgrade

**D-5 is the exception to D-4, and it is narrow.** A finding on unchanged code in a touched file is
permitted only when the change makes that code newly wrong or newly load-bearing. It is then
anchored to the **added line responsible**, not the untouched line, and marked
`scope: pre-existing`. Unchanged code the diff does not disturb stays out of the review.

Where it does apply, report at real severity. The implementer or the human decides what to do
about it; a reviewer surfaces, it does not decide whether the surface is worth fixing.

Never author a dismissal: "out of scope", "tracked separately", "pre-existing pattern", "cascades
through N call sites". If a fix genuinely exceeds this change, the finding says so and escalates —
it is not dropped.

## D-6 · Generated and vendored code

Excluded by **path**, listed in `rules.json`. Typical shapes:

```
**/gen/**  ·  **/generated/**  ·  **/*.pb.go  ·  **/*_gen.go  ·  **/*.g.dart
vendor/**  ·  node_modules/**  ·  **/testdata/**  ·  lockfiles  ·  **/*.snap
```

**Do not exclude on a `Code generated … DO NOT EDIT` header.** Scaffolding tools routinely emit
that header into files whose bodies are then hand-written — resolver stubs are the classic case.
Marker-based filtering silently skips real, reviewable code, and nothing tells you it happened.

## D-7 · Repository invariants where the naive check is wrong

Every codebase has two or three rules that produce hundreds of false positives when applied
literally — a pattern that is banned in new code but present in hundreds of existing files, or an
API that is forbidden in one direction and correct in another.

These belong in `project.md`, stated with the *measured* counts. A rule like "never use X" is
useless when X appears 230 times legitimately; "never use X for whole-row writes, though
column-level X is correct" is a rule that can be applied.

## D-8 · Cost is a defect category

A new call site on a billed API, an eager fetch inside a loop, an N+1 across a network boundary,
a retry with no ceiling — these are defects with an invoice attached, and they are invisible to
every linter.

`category: cost`. Require the arithmetic: the per-call price and the expected calls per day. A
suppression comment that waives a cost guard without that multiplication is itself a finding.
