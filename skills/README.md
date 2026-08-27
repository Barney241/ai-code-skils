# The skill contract

A skill is a directory under `skills/`. It is valid if it contains a `SKILL.md` whose first lines
are YAML frontmatter with `name` and `description`.

```
skills/<name>/
  SKILL.md              required
  README.md             optional — for humans: why it exists, what it was tested against
  rules/                optional — path-scoped rule documents
  assets/               optional — prompts, schemas, checklists
  tools/                optional — real programs the skill calls
```

`tools/` is where a skill puts work that should not cost model attention. Polling an API, computing
a verdict, tracking a quota — these are deterministic, so they belong in a program the agent *calls*
rather than in instructions the agent *follows*. `watch-pr` ships
[`prwatch`](watch-pr/tools/prwatch) for exactly this reason: before it existed, sessions hand-wrote
a poller per PR and each one re-derived "is this done?" wrongly, in the same two ways.

Keep tools dependency-light and independently runnable. A tool that only works inside one repo is a
tool nobody else can use.

## SKILL.md

```markdown
---
name: review
description: "One sentence an agent can match a task against. Say when to use it, not just what it is."
---

# Review

Instructions the agent follows, in the order it should follow them.
```

`description` is the only thing most agents see when deciding whether to load the skill, so it
must contain the trigger, not a summary. "Precision-first local code review. Use before every
commit, when asked to review local changes, or when the commit gate blocks" beats "A code review
skill".

## Writing rules that hold

Every rule in this repo follows the same three-part shape, because a rule without the second and
third parts produces confident nonsense:

- **ASSERT** — what must be true, stated so it can be false.
- **VERIFY** — the command that establishes it, and what its output means.
- **DO NOT FLAG** — the case where flagging this would be wrong.

The third part is the one people skip and the one that decides whether anybody keeps using the
skill. A rule with no carve-out fires on every legitimate exception until the reader learns to
ignore it.

Two failure modes worth naming, both found by running these rules against a real repository:

- **A command that silently matches nothing** is worse than no rule — it reports clean forever.
  `odin/**/*_test.go` matched *one* file instead of 543 because `globstar` is off by default.
  Run every VERIFY command once against a real tree and check the count is plausible.
- **A pattern matched against names instead of structure** breaks the moment naming differs. A
  field regex keyed on `Port|Repo|Store` suffixes returned zero fields on a codebase whose fields
  were called `store`, `repair`, `nowFn`.

## Portable vs. project-specific

Anything true of every codebase goes in `rules/`. Anything true only of *your* repository — the
enabled linters, the ORM idiom, the function everybody misuses, the cost of a billed API call —
goes in a `rules/project.md` that the user writes from `rules/project.template.md`.

A skill that hardcodes one repository's facts cannot be shared, and a skill with no place to put
them gets forked immediately. Both files, loaded together.

## Adding a skill

1. `mkdir -p skills/<name>` and write `SKILL.md` with frontmatter.
2. Put universal rules in `rules/`, and a `project.template.md` if the skill needs local facts.
3. Add a row to the table in the root `README.md`.
4. `./install.sh --list` should now show it. No registration step, no manifest to update.
