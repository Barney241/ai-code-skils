# Calibrating the verifier

An all-KEEP run proves nothing on its own. A rubber stamp and a working verifier produce identical
output when every finding happens to be sound — and the only way to tell them apart is to hand it
something that *should* be deleted.

Do this whenever the verifier prompt changes, and periodically otherwise.

## The probe

Mix **two fabricated findings** into a set of real ones and run the verifier normally. It must
delete both, on the right ground, naming the line.

**Ground-A probe** — a plausible claim about code that is nowhere in the diff. Invent a call:

> path `dispatcher.go:71`
> anchor `if err := t.Validate(ctx); err != nil { return err }`
> claim: The new `Validate` call runs inside the per-item loop and returns on the first error, so
> one malformed item aborts dispatch for every remaining item.

Nothing named `Validate` appears in the diff. Correct verdict: `DELETE-GROUND-A`.

**Ground-B probe** — a claim one added line flatly contradicts:

> path `metrics.go:70`
> anchor `m.skipped.WithLabelValues(trigger, table, op).Inc()`
> claim: `observeSkip` dereferences `m.skipped` with no nil-receiver guard, unlike `observe`
> directly above it, so any runner constructed without a registry panics.

The diff shows `observeSkip` opening with `if m == nil { return }`. Correct verdict:
`DELETE-GROUND-B`, citing that line.

## Reading the result

| Outcome | Meaning |
|---|---|
| Both probes deleted on the right grounds, all real findings kept | Working. |
| A probe survives | Not filtering. The prompt is decorative — tighten the proof requirement. |
| A real finding deleted alongside the probes | Over-filtering, and this is the more expensive failure. Re-read the asymmetry paragraph; something in the prompt is licensing deletion-by-doubt. |
| A deletion with no line named | Invalid. The requirement to cite a line is what makes a deletion falsifiable — enforce it. |

## A worked result

Run against a real 6-file change, three real findings plus the two probes above:

```
G1: KEEP
G2: DELETE-GROUND-A — no `t.Validate(ctx)` line exists anywhere in the diff
G3: KEEP
G4: DELETE-GROUND-B — `+	if m == nil {`
G5: KEEP
```

Both probes caught, all three real findings kept. The same verifier, on the unmixed real set, had
returned 7 of 7 KEEP — which on its own would have been indistinguishable from a stamp.

Worth asking the verifier afterwards whether any finding tempted it toward a forbidden deletion,
and why. That answer is data about the prompt, not about the findings; one run reported being
tempted to delete an unverifiable-from-the-diff claim, which is exactly the instinct the asymmetry
paragraph exists to override.
