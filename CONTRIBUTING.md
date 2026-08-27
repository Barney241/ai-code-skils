# Contributing

Contributions welcome — new skills, better rules, adapters for tools not covered yet.

## The bar for a rule

Every rule follows `ASSERT` → `VERIFY` → `DO NOT FLAG`. A pull request that adds a rule without the
second and third parts will be asked for them, because a rule missing them produces confident
nonsense and trains people to ignore the skill.

- **ASSERT** — what must be true, stated so it can be false.
- **VERIFY** — the command that establishes it, and what its output means.
- **DO NOT FLAG** — the case where firing this rule would be wrong.

**Run every VERIFY command once against a real repository and check the count is plausible.** The
worst rule is not a wrong one; it is one whose command silently matches nothing and therefore
reports clean forever. Two real examples are in
[`skills/review/README.md`](skills/review/README.md).

## Portable vs. project-specific

If a rule depends on a specific linter, ORM, framework, directory layout, or price list, it belongs
in that skill's `project.template.md`, not in the shared rules. The template is what makes the rest
portable.

## Evidence over assertion

Claims in a skill should be traceable. "This is a measured failure site" needs a number behind it,
or it should be softened to "this is a common failure site". Where a number came from a private
codebase, generalise the shape and drop the figure rather than shipping something unverifiable.

## Adding a skill

See [`skills/README.md`](skills/README.md). In short: `skills/<name>/SKILL.md` with `name` and
`description` frontmatter, universal rules in `rules/`, a row in the root README table. No manifest
to update.

## Style

- Markdown, wrapped around 100 columns.
- No badge decoration, no emoji section markers.
- Write for someone who will disagree with you. State the carve-out before they find it.

## Attribution

If an idea comes from another project or paper, cite it in that skill's README under Sources. The
existing entries are the model.
