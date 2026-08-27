# Tests — deploy-safety rules

Applies to: every diff. The question this lane answers is not "is coverage up" — it is
**"if this ships and is wrong, what catches it, and at which layer?"**

Coverage percentage is not a finding. A test that executes a line without asserting behaviour is
worse than no test: it makes the line look protected. Never ask for a test to raise a number; ask
for a test because a specific failure would otherwise reach production silently.

---

## T-1 · Pick the layer from the change, not from habit

| What changed | Layer that makes it safe | Why lower layers do not |
|---|---|---|
| Pure function, mapper, validation, encoding, arithmetic | **Unit** | No I/O to get wrong |
| Anything touching a real datastore, query builder, cache, search index, or message broker | **Integration**, against the real dependency | A fake cannot reproduce a constraint violation, a partial write, or a driver error |
| An endpoint, resolver, or anything a client calls | **End-to-end** | Schema, wiring and dependency injection are only exercised together |
| A cross-service contract or event payload | **End-to-end, on both sides** | The single most expensive defect class in any distributed system |
| A schema migration | **Integration, run twice** | It must be safe to re-run on every deploy |
| A cron, reaper, queue consumer, or backfill | **Integration with a real store** | Idempotency and drain behaviour are invisible to a unit test |

**ASSERT** For each behavioural change in the diff, a test exists at the layer this table names.

**VERIFY** Name the test file and function. If you cannot name it, that is the finding:
`severity: major`, `category: test`, stated with the specific failure that would escape.

**DO NOT FLAG** a missing end-to-end test for a change with no client-visible surface, or a missing
unit test for a function whose only logic is delegation.

---

## T-2 · A bug fix without a regression test is incomplete

**ASSERT** Every fix adds a test that **fails against the pre-fix code**.

**VERIFY** Read the test and ask: would this have been red before the fix? If it only asserts the
new happy path, it would have passed before too, and it is not a regression test.

`severity: major` — without it, the same bug is free to come back.

**DO NOT FLAG** when the fix is a pure refactor with no behaviour change, or when the regression is
genuinely only observable in production infrastructure — but say which, and say why a test cannot
reach it.

---

## T-3 · The test must fail when the code breaks

**ASSERT** Assertions constrain the behaviour that matters.

**VERIFY** Mentally break the changed code — flip a condition, drop a field, return the zero value.
If every assertion still passes, the test is decorative. `severity: major`, `category: test`.

Shapes that pass while broken:

- Conditional assertions that skip nil or absent fields — these hide missing-overwrite bugs, the
  "a partial update erased a stored value" class.
- Asserting only that no error was returned.
- Field-by-field assertions that omit the field the change actually touched.
- Golden files regenerated from the new output in the same commit.
- A test asserting through a wrapper the production path no longer calls.

---

## T-4 · Full-object comparison, table-driven

**ASSERT** Multiple scenarios share one table and a loop, with one assertion on the whole expected
object rather than field by field. A field-by-field test silently stops covering any field added
later.

Dynamic values — timestamps, generated ids, revisions — are copied from actual into expected, then
constrained separately.

**DO NOT FLAG** a single-scenario test for a single-behaviour function. The table rule is about
multi-scenario functions.

---

## T-5 · Real dependencies; mocks are a decision, not a default

**ASSERT** Databases, caches, brokers and search indexes come from disposable real instances
(containers). Only third-party external services are mocked.

A mock standing in for an internal dependency is `severity: major`, `category: test`, and the
finding asks for an explicit decision rather than assuming one.

**DO NOT FLAG** no-op or fake implementations passed to a constructor purely to satisfy non-nil
validation — that is required by the constructor rules, and it is not mocking.

---

## T-6 · The mechanically checkable rules are the valuable ones

Every project has two or three test conventions that a command can check and nothing currently
enforces — "e2e must use the generated client, never raw request strings", "test data goes through
the helpers, never raw SQL".

Put yours in `project.md` with the exact command. Two warnings, both learned the hard way:

- **Verify the command matches something.** A rule whose command silently matches nothing reports
  clean forever. `dir/**/*_test.go` matched *one* file instead of 543 because `globstar` is off by
  default in bash. Prefer `find dir -name '*_test.go' -exec grep ... {} +`.
- **Check the count is plausible** the first time you run it, against a tree you know.

---

## T-7 · Test files may be unlinted

Many projects exclude test files from linting. Where that is true, ordinary lint-class defects in
test files — unchecked errors, unused variables, shadowing, unclosed resources — **are in scope for
this lane**, because nothing else will ever report them.

**DO NOT FLAG** these in non-test files, where the linters own them and restating is noise. Confirm
your project's setting before relying on this.

---

## Output contract for this lane

A test finding must name:

- the **layer** it belongs at, from T-1;
- the **concrete failure** that reaches production without it — an input, a state, and the wrong
  output;
- for T-2, why the proposed test would have been red before the fix.

"Add tests for this" is not a finding. "A sync with a nil `Description` overwrites the stored
value; no test covers the nil branch" is a finding.
