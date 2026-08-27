# Comments — value rules

Applies to: every diff, every language.

**The bar is value, not count.** A comment that carries something the code cannot is welcome and
must not be flagged for existing. A comment that restates the code, or points at something which
will move without it, is a liability — it is a claim every future edit has to keep true, and
nothing enforces it.

> **Precedence.** Some projects set a *count* bar ("the default number of comments in a diff is
> zero"). This document sets a **value** bar and governs the comments lane. The two agree on every
> deletion rule below; they differ only in that a comment earning its place under C-1 is not a
> finding here. If your project states a count bar, reconcile it — reviewers must not be choosing
> between two sources.

---

## C-1 · Keep: the comment says something the code cannot

A comment earns its place when a competent reader of this codebase would otherwise get it wrong.
These are the shapes worth writing:

- **An external constraint.** `// The provider returns 200 with an empty body when the record was deleted upstream.`
- **A deliberate deviation that looks like a mistake.** `// Unbuffered on purpose: the send must block until the reaper has drained.`
- **An ordering or timing dependency the types cannot express.** `// Must run before the cache warm; the warm reads the rows this writes.`
- **A unit or contract the signature cannot carry.** `// Revision is a millisecond CDC timestamp, not a counter.`
- **Why the obvious approach was rejected**, when the obvious approach is genuinely tempting and wrong here.

**DO NOT FLAG** any comment of these shapes. Reviewers who delete these destroy the only record of
a decision.

---

## C-2 · Delete: the comment describes the code

**ASSERT** No comment restates what the line below it does.

```go
// increment the counter          ← delete
counter++

// loop over the items            ← delete
for _, p := range items {
```

`severity: info`, `category: documentation`. The test: cover the comment and read the code. If you
learn nothing the code did not already say, it is restatement.

---

## C-3 · Delete: the comment points at something that will move

A comment referencing another location by name, number, or identifier is a dangling pointer the
compiler cannot check.

**ASSERT** No comment references:
- a **ticket id** — `// see PROJ-5215`, `// fixes JIRA-88`
- a **PR, commit, or branch** — `// added in #3390`, `// see commit abc123`
- **another file, function, or line by name**, where the reference is decorative rather than a
  genuine contract — `// mirrors the logic in itemMapper.go`
- a **migration id**, changeset number, or schema version
- a **design discussion** — `// as agreed in the architecture review`
- a **person** — `// per the reviewer's suggestion`

`severity: minor`, `category: documentation`. Each rots the moment the target moves, and nothing
will tell you.

**The reframe that fixes them:** state the *constraint*, not the *reference*.

```
// see PROJ-5215
```
becomes
```
// A partial refresh must not write zero over a stored field — the value is unrecoverable.
```

The second survives the ticket being closed, the PR being squashed, and the file being renamed.

**DO NOT FLAG** a `TODO` carrying a ticket id — that id is the tracking handle, not a pointer to an
explanation. A `TODO` *without* one is deleted under C-4.

**DO NOT FLAG** a cross-reference that is a genuine, checkable contract — a struct tag naming a
wire format, a link to an RFC or vendor doc, a compiler directive, or a published identifier
something else binds to (a metric name on a dashboard, an event type on the wire). Those are
load-bearing.

---

## C-4 · Delete: narration of the change itself

**ASSERT** No comment describes the edit rather than the code.

```go
// changed to use errors.Is        ← delete
// refactored this to be cleaner   ← delete
// TODO: clean this up later       ← delete unless it carries a ticket id
```

What changed and why belongs in the commit message and the PR body, which are versioned and cannot
rot in place. `severity: info`.

---

## C-5 · Delete: banners and structural decoration

```go
// ============ HELPERS ============    ← delete
// --- end of validation ---            ← delete
```

If a file needs section banners to be navigable, the finding is the file's size, not the banner.
`severity: info`.

---

## C-6 · Delete: commented-out code

`severity: minor`. Version control has it. A commented-out block is indistinguishable from code
disabled by mistake.

---

## C-7 · Length is a signal about the code

**ASSERT** A comment needing more than one or two lines usually means the code should be clearer
instead — rename the symbol, extract the function, or let the type carry the meaning.

`severity: info`, and the finding proposes the rename or extraction rather than asking for a
shorter comment.

**DO NOT FLAG** a genuinely irreducible explanation — a subtle algorithm, a protocol quirk, a
wire-format constraint. Some things take three lines and that is correct. Exported symbols keep
their doc comment; that is API surface, not commentary.

---

## C-8 · Flag the missing comment too

This lane is not only deletion. **ASSERT** that a genuinely non-obvious *why* is documented.

If the diff contains a magic constant, a deliberate ordering, a retry bound, a timeout, or a
workaround whose reason is not visible from the code, the absence of one explaining line is
`severity: minor`, `category: documentation`. Say what question a future reader will ask.

---

## Output contract for this lane

A finding about a comment that **exists** cites it verbatim and gives the rule id plus what the
reader learns from the code alone.

A **C-8** finding has no comment to quote. It cites the code location instead — the same verbatim
added lines every other finding anchors to — and states the question a future reader will ask that
the code cannot answer. Requiring a quote here would make the one rule about absence unreportable.

Never propose a rewrite that merely shortens a restatement — if it restates, it goes.

**Respect local convention.** A doc line that mirrors the one on the function directly above it is
convention, not redundancy. Read the neighbours before flagging a comment as boilerplate.
