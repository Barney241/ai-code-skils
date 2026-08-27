# Adapters — using these skills in any AI system

The skills are plain markdown. Every tool below can read them; they differ only in *which file* the
tool looks at and *when* it loads it.

Two loading models, and it is worth knowing which one you are dealing with:

- **On-demand** (Claude Code skills, Cursor agent-requested rules) — the tool reads the
  `description` and decides whether to load the body. Cheap; keep the description trigger-shaped.
- **Always-on** (`AGENTS.md`, `CLAUDE.md`, Cursor `alwaysApply`, Copilot instructions) — the content
  sits in every request. Reference the skill from there rather than pasting it, or you pay for it on
  every turn.

---

## Claude Code

```bash
./install.sh review /path/to/repo
```

Produces `.agents/skills/review/` plus the `.claude/skills/review` symlink. Claude Code discovers
skills under `.claude/skills/`; `.agents/` is the tool-neutral copy every other adapter points at.
Invoke with `/review`, or let the agent match the `description`.

## Cursor

Cursor reads `.cursor/rules/*.mdc`. Create a thin pointer rather than duplicating the skill:

```markdown
---
description: Precision-first code review before committing
globs:
alwaysApply: false
---

Follow the pipeline in `.agents/skills/review/SKILL.md`, with the rule documents in
`.agents/skills/review/rules/`. Load them now and follow them exactly.
```

`alwaysApply: false` with a `description` makes it agent-requested — the equivalent of a skill.

## AGENTS.md (Codex, Amp, Jules, and a growing list)

Append a pointer to the repository's `AGENTS.md`:

```markdown
## Code review

Before committing, follow `.agents/skills/review/SKILL.md`. Rules are in
`.agents/skills/review/rules/`; the verifier prompt is in `assets/verifier-prompt.md` and must be
passed verbatim to a separate agent.
```

## GitHub Copilot

`.github/copilot-instructions.md` is always-on, so keep the pointer short:

```markdown
For code review, follow `.agents/skills/review/SKILL.md` and its `rules/` directory.
```

Path-scoped variants go in `.github/instructions/*.instructions.md` with a `applyTo` frontmatter
glob, which maps well onto this skill's per-path rule documents.

## Aider, Windsurf, Continue, Zed

All support a project instruction file (`CONVENTIONS.md`, `.windsurfrules`, `.continuerules`,
`.rules`). Same pattern: point at `.agents/skills/<name>/SKILL.md` rather than pasting it.

## A plain chat window

No project files, no tooling. Paste, in order:

1. `skills/review/SKILL.md`
2. the rule documents that match what you changed — `rules/rules.json` says which
3. your diff (`git diff <base>...HEAD`)

Then run the verify stage as a **second, separate message in a fresh conversation**, pasting only
`assets/verifier-prompt.md`, the diff, and the findings. The isolation is the mechanism — a
verifier that has already seen the reviewer's reasoning will agree with it.

---

## Porting notes

- **The verifier must be a separate context.** Sub-agent, second window, second API call — any of
  these work. What does not work is asking the same conversation to check itself.
- **Stages 0, 2, 5 and 7 are shell, not model.** Selecting files, grouping, anchoring and recording
  are deterministic. Any tool that can run commands should; any tool that cannot should have the
  results pasted in.
- **`description` is the trigger.** Tools that route on it need it to say *when to use this*, not
  what it is.
- **Nothing here needs a metered key.** If an adapter only works by billing per push, it is the
  wrong adapter.
