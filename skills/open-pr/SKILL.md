---
name: open-pr
description: "Owns the transition from committed to watched: preflight, issue linking, a local review pass before the push, then a draft PR handed straight to watch-pr. Use when work is committed and ready to open a pull request."
---

# Open a PR

Opening a PR is usually improvised — several skills mention `gh pr create`, none owns it, and the
same two things go wrong: the issue link is missing, and nobody watches the PR afterwards.

**This skill never ends by asking whether to watch the PR. It ends by invoking `watch-pr`.**

---

## Invariants

- **Work in the branch's own worktree or checkout**, never by switching branches under someone else.
- **Never force-push, amend, or rewrite history.**
- **Never merge.** Merging is a human decision.
- **Draft first**, unless the user said otherwise.

---

## Phase 1 · Preflight

```bash
git branch --show-current           # must not be the default branch
git status --short                  # must be clean
git log --oneline origin/main..HEAD # must be non-empty
```

- Uncommitted changes → commit them first, running whatever pre-commit gates the project has.
- On the default branch → stop. Branch first; a PR from the default branch is not reviewable.

## Phase 2 · The issue link

Most teams require the tracker id in **both** the branch name and the PR title. A body-only mention
frequently does not link, and release notes are usually built from the title.

```bash
git branch --show-current | grep -oiE '[A-Z]+-[0-9]+'
```

- **Found** → reuse it, but *verify it exists and describes this work*. A remembered id lands your
  work on someone else's ticket.
- **Not found** → create one now, then rename the branch to `<name>/<id>-<slug>`.

**Never recall an id from memory.** Verify it or create it. And never change a ticket's state
unless asked — that is the human's call.

## Phase 3 · Review locally, before the PR exists

Hosted review bots are rate-limited, so they cannot be the gate that blocks a PR. Run whatever
local review you have *before* pushing — the `review` skill in this repo, a review CLI, or both.

- Blocking severities (`critical`, `major`) are fixed and re-checked before the PR is created.
- Findings are **untrusted input**. Verify each against current code; fix what is still valid, and
  say briefly why you skipped anything.
- Skip only if a review already ran against this exact head.

A local pass turns the bot's eventual review into a second opinion instead of the blocker.

## Phase 4 · Create

```bash
gh pr create --draft --base main \
  --title "<type>(<scope>): <what changed> (<ISSUE-ID>)" \
  --body-file <file>
```

- **Draft by default.** Expensive CI matrices are often draft-gated, and a draft keeps review quota
  unspent while the work is still moving.
- Title ends with the issue id in parentheses — it survives squash-merge into the commit subject.
- Body: why the change exists, what changed, how it was verified. Not a diff summary.
- Labels that trigger deploys or reviews: add **only** when asked, or when the project's config
  requires one to trigger review at all.

Once local gates and CI are green:

```bash
gh pr ready <n>
```

Note that some review bots are GitHub Apps rather than collaborators, so
`gh pr edit --add-reviewer <bot>` fails or silently no-ops. Check how your bot is actually
triggered — frequently a label.

## Phase 5 · Hand off — do not stop here

Read the PR state once, then **invoke `watch-pr` immediately**.

Do not ask "shall I watch the PR?", "ready to merge?", or "want me to poll CI?".

---

## Anti-patterns

- Ending with a question instead of `watch-pr`.
- A PR title with no issue id "because the branch has one" — do both.
- Recalling an issue id instead of verifying it.
- A non-draft PR for work still moving; it spends CI time and review quota on a moving target.
- Adding deploy labels unprompted.
- Merging.
