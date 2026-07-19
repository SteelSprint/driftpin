# Observation 0012 — 2026-07-19-073xxx (atomic-minimax)

Date: 2026-07-19
Runs:
- `/workspaces/drift/eval/runs/atomic-minimax-r0` (bad-link)
- `/workspaces/drift/eval/runs/atomic-minimax-r1` (code-refactor)
- `/workspaces/drift/eval/runs/atomic-minimax-r2` (drift-detection)
- `/workspaces/drift/eval/runs/atomic-minimax-r3` (level1)
- `/workspaces/drift/eval/runs/atomic-minimax-r4` (level2)
- `/workspaces/drift/eval/runs/atomic-minimax-r5` (level3)

Subject: `minimax-coding-plan/MiniMax-M3`. Judge: skipped (`--skip-judge` flag). Each subject produced a self-debrief and a clean `drift todo` post-run state.

## Known issues

`--skip-judge` was added to the eval driver mid-run to dodge the long judge phase (full judge run aborted twice previously on the 1-hour wall-clock budget). This run captures subject signal only — there are no `report.md` files because no judge wrote them, and no cross-run synthesis was generated. Findings below are *direct observations from the subjects' self-debriefs*, not judge-corroborated. Treat as exploratory signal.

Methodology caveat: a second eval-batch (atomic-closures-1) ran earlier with the same prompts against `zai-coding-plan/glm-5.2`. Some themes below appear in both cohorts and likely reflect the *prompts/UX* rather than the *model*. Cross-cohort comparison is partial; themes marked [both] appear in both. Where helpful, atomic-closures-1 findings are referenced inline.

## Convergent findings

| Theme | Runs | Priority |
|---|---|---|
| `drift skill` is sufficient cold-start documentation | r0, r1, r2, r3, r4, r5 | [High] |
| Closure model (hash, drift events, decision tree) lands clearly | r1, r2, r3, r4, r5 | [High] |
| `[SEED] / [citer]` labels make drift directionality legible | r1, r5 | [Medium] |
| Marker-placement guidance during refactors removes a real judgment call | r1, r4 | [Medium] |
| `drift list --verbose` and `drift show <id>` are the primary investigation tools; absence of side-by-side "context" annotation limits their power | r0 | [Medium] |
| `--dry-run` (exit 3) is a well-received safety net | r0, r1, r2 | [Medium] |
| `drift` binary not on `$PATH` in fixture workspaces | r4 | [Low] |

## Divergent findings

| Run | Finding |
|---|---|
| r0 (bad-link) | Wrong-but-consistent link (`palindrome_func → main.reverse`) is invisible to `drift todo`. Suggested: a `drift check`/`drift verify` for semantic-link sanity (already in PLAN.md as deferred #7). |
| r0 (bad-link) | `--help` lists `drift show <marker\|spec>` but doesn't exemplify its output shape. |
| r1 (code-refactor) | "reset" verb is initially ambiguous for cold users (suggests file revert; actually means "accept scan as new baseline"). Suggested: short help clarification or `accept`/`sync-baseline` alias. |
| r1 (code-refactor) | A worked end-to-end cosmetic-refactor recipe in `skill` would be welcome; the pieces are all present, but no single example walks it through. |
| r1 (code-refactor) | When extracting helpers, future edits *inside unmarked helpers* won't drift the marker that wraps the public entry point. Subject flagged this as a potential blind spot. Could warn in `diff` or suggest dedicated specs. |
| r2 (drift-detection) | Spec wording precision (e.g. "any non-empty length" vs. "up to 50 chars") is left entirely to the subject's judgment. Drift doesn't help author the new spec text. |
| r2 (drift-detection) | Whether the spec needed updating was initially ambiguous to MiniMax-M3. It reasoned correctly ("returns an error for division by zero is still true — `-0 == 0`"), but the decision tree guidance could be more explicit about this case. |
| r2 (drift-detection) | "characters vs bytes vs runes" Go-semantics wrinkle. Subject used `len(name)` (bytes) rather than `utf8.RuneCountInString`. Drift correctly reported drift either way, but the task wording was ambiguous. |
| r3 (level1) | MiniMax-M3 explicitly thanked the decision tree. "The decision tree was in `drift skill` rather than `--help`. For an LLM that may only skim `--help`, surfacing the most likely decision inline would speed up use." |
| r4 (level2) | `drift` not on `$PATH` — requires `./drift` prefix. Minor, but reproducible across cohorts. The fixture workspace should include a `.envrc` or wrapper, or the task prompt should call this out. |
| r4 (level2) | `gofmt` reformatting the `import` block changed line numbers in markers and produced extra drift events. Drift correctly reported this; subject resolved via review-and-reset. Cosmetic-only diffs being reportable as drift is by design, but worth noting. |
| r5 (level3) | "Predicting which side is the seed" was a learning curve — the closure lists *both* the spec and marker as seeds when both have `NODE_CHANGED` events. The seed terminology is slightly ambiguous when the closure absorbs multiple events. |
| r5 (level3) | `<ref>` hash-stripping mentioned in skill but not exercised in this cohort (no spec citations). MiniMax-M3 noted it as a subtle invariant worth keeping in mind. |

## MiniMax-M3 model observations (cross-cutting)

Distinct characteristics of MiniMax-M3 vs. the prior `zai/glm-5.2` cohort (atomic-closures-1):

- **Faster subject wall-clock.** All 6 subjects completed within the `timeout=30m0s` budget with substantial slack (transcripts 37–106 lines; the calibration single-subject run completed in <1 minute, and the full 6-subject parallel batch finished without abort). Compare to `zai/glm-5.2` atomic-closures-1 which also completed but required 1+ hour total.
- **More terse self-debriefs.** Subjects wrote concise but complete reports (37–106 transcript lines including the model output). The same task against `zai/glm-5.2` produced longer transcripts (50–80 self-debrief lines plus surrounding reasoning). Subjective signal: MiniMax-M3 reports what it did, `zai/glm-5.2` reports more of its reasoning process.
- **Same successful task completion.** All 6 MiniMax-M3 subjects completed their tasks correctly and reached clean `drift todo` state. No subjects got stuck.

## Prioritized recommendations (consolidated)

1. **[High]** Surface the decision tree at `--help` level, not just `drift skill`. Three cohorts converged on this: subjects who skimmed `--help` would benefit from a one-line "NODE-CHANGED on marker → if code still implements spec, reset; otherwise fix code, then reset."
2. **[High]** Document the `state.xml` schema and the role of line numbers in `--help` or a follow-up `drift schema` command. Currently buried in `skill`; subjects had to read `state.xml` directly.
3. **[Medium]** Implement `drift check`/`drift verify` for semantic-link sanity (already deferred from PLAN.md as #7). The bad-link task is a clean forcing function.
4. **[Medium]** Add `drift reset --help` text that explicitly states "accept reviewed current content as the new baseline; does not revert source files." Or rename to `accept`/`sync-baseline` and keep `reset` as alias.
5. **[Medium]** When a marked function has changed from inline implementation to calls into unmarked helpers, warn in `diff` and suggest a larger marker range or dedicated specs for the helpers.
6. **[Low]** Add a "what changed inline" hint to `drift todo` listing the hash delta (currently present in the event detail lines; could be promoted for LLM scripting).
7. **[Low]** Fixture workspace: include `.envrc` or wrapper so `drift` is on `$PATH` in eval subjects' working dirs.

## Next steps

- Re-run the same atomic cohort against `zai/glm-5.2` with `--skip-judge` for direct cross-model comparison (the atomic-closures-1 run was judge-included and 1 hour total — different conditions).
- Re-enable judges for the atomic cohort once a faster model is found or eval is restructured. For signal quality, judge self-evaluation against a different model is preferable to no judge.
- Implement `drift check`/`drift verify` (PLAN.md #7) — the bad-link task is a clean motivation.
- Move decision tree + state.xml schema from `skill` to `--help` (recommendation #1, #2 above).
