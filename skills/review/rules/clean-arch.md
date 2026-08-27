# Clean architecture — verification rules

Applies to: any file declaring a usecase, port, domain type, or delivery handler.

Other documents *state* these rules. **This one is how a reviewer proves they hold.** Every rule is
`ASSERT` (what must be true) → `VERIFY` (how to establish it, with a command) → `DO NOT FLAG` (the
carve-out). A finding on any rule here is not filed without the evidence its VERIFY step produces.

Naming below uses `usecase` / `domain` / delivery / storage as generic layer names; substitute your
project's. Record the equivalents, and your reference implementations, in `project.md`.

**Test files are out of scope for CA-1…CA-9.** Test code reaches across layers on purpose — a
wiring test importing an adapter is the test doing its job, not a dependency inversion. Read test
files as evidence when a rule needs them, then hand them to the tests lane. Say so in the coverage
accounting rather than recording them as unreviewed.

---

## CA-1 · Dependency direction points inward

**ASSERT** The usecase layer imports only its domain, the standard library, and observability. It
never imports a delivery framework or a storage driver.

**VERIFY** Read the import block:

```bash
sed -n '/^import (/,/^)/p' <path>/usecase.go
```

Any HTTP router, schema framework, SQL package, or database driver in the usecase layer is
`severity: major`, `category: maintainability`.

**DO NOT FLAG** `context`, logging, tracing, `errors`, `time`. Those are cross-cutting and
permitted inward.

---

## CA-2 · Interfaces are defined where consumed

**ASSERT** Every external dependency a usecase needs is declared as a small interface **in the
usecase file**, named for behaviour (`store`, `repository`, `tokenIssuer`), not for technology
(`postgresStore`, `redisClient`).

**VERIFY** List the interfaces declared in the usecase file, then find their implementers with the
language server. The implementer belongs in the storage or adapter file.

**DO NOT FLAG** an interface declared elsewhere when it is a genuinely shared contract already
consumed by two or more slices — say so and cite both consumers.

---

## CA-3 · Model separation is strict

**ASSERT** Domain types carry **no** framework tags. Serialization tags live on DTOs in the
delivery layer; storage tags live on records in the storage layer.

**VERIFY**

```bash
grep -nE '`(json|db|bson|protobuf|gorm|yaml):' <path>/domain.go
```

Any hit in the domain file is `severity: major` — a database or API schema change will break the
other side.

**DO NOT FLAG** tags on types in delivery or storage files. That is where they belong.

---

## CA-4 · Usecases carry business intent

**ASSERT** Usecase method names describe an operation in the domain's language (`RegisterToken`,
`OnboardNewUser`), not a CRUD proxy (`UpsertAuth`, `SaveUser`).

**VERIFY** List the exported methods and read them as a sentence someone from the product side
would recognise. A method that only forwards arguments to the store with no validation, no
invariant and no orchestration is `severity: minor` — say which invariant is missing.

**DO NOT FLAG** a genuinely thin read that exists to keep the delivery layer off the store. That is
the pattern working.

---

## CA-5 · Errors belong to the domain

**ASSERT** Sentinel errors are declared in the domain. Storage translates driver errors into them;
delivery translates them into responses. No driver error escapes into a usecase or a handler.

**VERIFY** Search for driver error symbols outside the storage file. A hit is `severity: major`,
`category: bug` — the caller cannot branch on it without importing the driver, which breaks CA-1.

**DO NOT FLAG** the translation site itself in the storage file. That is the correct and only
place.

---

## CA-6 · Delivery layers reuse the usecase, never re-derive

**This is the rule that rots the architecture in practice, and it is mechanically checkable.** A
second delivery layer that computes a field itself silently ships weaker output that drifts from
the canonical layer.

A documented case: a secondary API returned a description straight off a near-empty column while
the primary resolver called a usecase that assembled it from a cache. Hundreds of thousands of
records were invisible on the second surface. No error was raised anywhere, and no test failed.

**ASSERT** For every field a new or changed delivery layer returns, the value comes from the same
usecase call a sibling delivery layer uses for that field.

**VERIFY — field by field, and this is not optional for a new delivery layer**

1. List the fields the changed layer returns.
2. For each, find the sibling delivery layer that also returns it.
3. If the sibling routes through a usecase and the changed layer reads a raw struct field or
   column, that is the defect. `severity: critical`, `category: consistency`.
4. Evidence required: the sibling's `file:line` and the usecase method it calls.

**DO NOT FLAG** a field with exactly one delivery layer, or a field whose sibling also reads it raw
— then both are raw and there is no drift to report (note it as `info` if the field looks derived).

---

## CA-7 · Constructors validate, methods do not

**ASSERT** The constructor returns a value and an error, validates every **interface** dependency
non-nil, and never panics. Methods contain no runtime nil guards.

**VERIFY**

```bash
# 1. the struct's fields -- the type column tells you which are interfaces.
#    Do not pattern-match field NAMES: real codebases call them store, repair,
#    nowFn, so any suffix regex silently matches nothing.
sed -n '/^type Usecase struct/,/^}/p' <path>/usecase.go

# 2. each one guarded inside the constructor, in either comparison direction
ast-grep -p 'if $DEP == nil { $$$ }' -l go <path>/usecase.go
ast-grep -p 'if nil == $DEP { $$$ }' -l go <path>/usecase.go
```

Compare the two lists by field name. A guard on an input argument, an error, or a concrete
dependency does not satisfy the rule and must not be counted as one. A constructor missing a
non-nil check is `severity: major`. A runtime nil guard **inside a method** is `severity: minor`
and the fix is deletion — the constructor already guarantees it.

**DO NOT FLAG** missing nil checks on concrete struct or value dependencies; the rule covers
interface dependencies. And do not demand a nil guard the constructor makes impossible — that
inverts the rule.

---

## CA-8 · The slice is wired into every entrypoint

**ASSERT** A new slice is registered in the composition root for **all** entrypoints the service
has — API server, cron runner, queue consumer, worker.

**VERIFY**

```bash
# the slice's own constructor names, alternated -- -E is required;
# a bare | is a literal character to basic grep and matches nothing.
grep -rnE 'NewThingUsecase|NewThingStore' <composition-root-dir>/
```

Present in one entrypoint and absent from another that needs it is `severity: major`,
`category: consistency` — the feature is silently dead in that process. Composition roots are a
measured failure site in most services, not a hypothetical.

**DO NOT FLAG** absence from an entrypoint that genuinely does not use the domain — but say which
one and why you concluded it does not.

---

## CA-9 · Migrated legacy code is deleted

**ASSERT** When a slice replaces a legacy path, the legacy implementation is removed, not left as a
shim.

**VERIFY** Language-server references on the legacy symbol. Zero non-test callers plus a live
replacement means it should have been deleted in this change.

Watch for the subtler version: a symbol kept as a thin wrapper that *only tests* call. The tests
then assert against a path production no longer runs, and a change to the real path leaves them
green.

**DO NOT FLAG** a shim explicitly retained for a parity test during a migration — check for the
parity test before flagging.

---

## Output contract for this lane

A clean-architecture finding is filed only with:

- the rule id (`CA-3`),
- the command run and what it returned,
- for CA-6, the sibling delivery layer's `file:line` and its usecase call.

A finding that says "this violates clean architecture" without naming a rule id and its evidence is
not a finding. Drop it.
