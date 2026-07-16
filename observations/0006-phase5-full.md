# Observation 0006 — phase5-full

Date: Thu Jul 16 2026
Runs:
- `/workspaces/filament/eval/runs/phase5-full-0`
- `/workspaces/filament/eval/runs/phase5-full-1`
- `/workspaces/filament/eval/runs/phase5-full-2`
- `/workspaces/filament/eval/runs/phase5-full-3`
- `/workspaces/filament/eval/runs/phase5-full-4`
- `/workspaces/filament/eval/runs/phase5-full-5`

## Known issues

- **Run 5 (phase5-full-5) — incomplete debrief artifact.** The subject hit the harness's 40-step cap and never produced the required `self-debrief.md`; the final assistant message contained a capable retrospective but the file was absent. The judge scored the run's driftpin usage as valid (all tool commands correct, workspace in sync), but downgraded the universal "self-debrief quality" criterion to FAIL. Because no subject-authored debrief exists, run 5's UX findings below are drawn solely from the judge's transcript observations rather than the subject's self-report; treat them as somewhat lower-confidence than findings from runs 0–4. The run is **not excluded** from convergence — only the debrief-derived signal is.
- **Run 3 (phase5-full-3) — positional cascade drift is a tool property, not a run compromise.** Adding a `math` import shifted the 10-line hash windows of unrelated `sub_func`/`mul_func` markers, producing spurious drift the subject had to reset. This is a reproducible tooling defect surfaced by the run; it is recorded as a convergent finding, not a taint.
- **No sandbox escapes, tainted runs, or methodology problems** were identified in any of the six runs. All workspaces verified independently by the judge at evaluation time.

## Convergent findings

| Theme | Runs | Priority |
|---|---|---|
| Add `drift diff` / rich `todo` output showing *what* changed (old vs. new hash/content), not just *that* it drifted | 0, 1, 2, 3, 4 | High |
| Show spec description text inline in `drift list` / `drift links --verbose` (IDs+paths alone cannot reveal consistent-but-wrong links) | 0, 1, 4, 5 | High |
| Add a read-only `drift inspect` / `drift show` / `drift hash` command to print a marker's hashed region + linked spec text without parsing `drift.pin` by hand | 0, 1, 2, 4 | Medium |
| Fix or document `line="0"` for specs in `drift.pin` and `drift list` (reads as a bug; undermines placement verification) | 0, 3, 4 | High |
| Document edge cases & hash-window semantics in `drift skill` (deleted markers, removed specs, duplicate IDs, marker-line inclusion, adjacent markers, `line=` auto-update) | 0, 2, 3, 4 | Medium |
| Add `--json` output to `drift todo` / `drift list` / `drift link` for reliable LLM-agent consumption | 0, 1, 2 | Medium |
| Add `drift check` / `drift verify` (assert all linked, no orphans, in sync, non-zero exit on failure) for CI / agent gating | 0, 4 | Medium |
| Clarify spec-ID vs. marker-ID qualification convention (`main.palindrome` vs. `palindrome_func`; local id has no dot inside `<module>`) | 0, 1, 4 | Medium |
| Add `drift reset --all` / bulk resolve for multi-edge refactors (subjects reset drift edge-by-edge) | 2, 3 | Medium |
| Improve `drift reset` UX: emit a confirmation line on success and/or clarify "collapses baselines" semantics in `skill` (currently silent + ambiguous) | 2, 3 | Medium |
| Replace/augment fixed 10-line hash window with a semantic region (function body / next decl boundary) — positional cascade drift (run 3) and marker-coarseness false-"in sync" (run 5) both stem from this | 3, 5 | High |

## Divergent findings

- **Run 0** — Suggested `drift auto-link` for single-module projects (link by ID similarity when shortcode matches a local spec). No other run raised this; likely low-value for multi-module repos.
- **Run 0** — Asked for explicit documentation of marker-scanner behavior inside string literals / block comments; no other run hit this.
- **Run 1** — Flagged that `drift.pin` stores **absolute** spec paths (an apparent `/tmp/tmp…/main.pin.xml` leftover was observed initially, then normalized after `unlink`/`link`). Recommend relativizing paths for portability across machines/CI. Unique to this run.
- **Run 1** — Proposed a `drift validate` semantic-plausibility pass (keyword/name-overlap heuristic between spec text and marker code) to catch consistent-but-wrong links. Related to run 5's `drift lint` but framed as link-validation rather than placement.
- **Run 1** — Minor debrief inaccuracy: claimed the spec path was a `/tmp/tmp…` leftover while the final `drift.pin` held the correct path; subject did not re-check the file after the fix.
- **Run 2** — `drift reset` emits **nothing on success**, forcing a redundant `drift todo` round-trip. Confirmed independently by the judge. (Folded into the convergent "reset UX" theme but the silent-stdout detail is run-2-specific.)
- **Run 2** — Suggested a `drift verify` hook that gates `reset` behind a build/test check; judged low priority and risks over-coupling to language toolchains.
- **Run 3** — Sharp observation that the task's "no other markers affected" success criterion is in **tension** with the tool's positional drift notion — unrelated markers *do* get flagged. Unique framing.
- **Run 3** — Asked for `--dry-run` on `drift reset` (preview which baselines will change). Not raised elsewhere.
- **Run 3** — Suggested `drift check`/`drift status` as an alias for `todo` ("status" is more conventional for git-fluent users).
- **Run 4** — Adjacent-marker placement (`thread_safety`@line 22, `shorten_type`@line 23) felt "fragile"; subject unsure whether overlapping 10-line windows are independent (they were). Unique edge case.
- **Run 4** — Asked for per-subcommand `--help` (e.g. `drift link --help`); currently only top-level help exists.
- **Run 4** — Suggested a self-contained `drift demo` / `drift init --demo` that creates specs, links markers, mutates one, and shows drift — to teach the detection cycle in <1 min. Notably, runs 0 and 4 both **never exercised drift/reset organically** (setup-only tasks), making this a divergent-but-valid concern.
- **Run 4** — Requested `drift --version` / changelog output and concrete `drift.ignore` examples.
- **Run 5** — Markers placed coarsely (e.g. `draw_phase` above the deal block, `hand_eval` on a type alias, `betting` on `playRound` not `bettingRound`); three real edits to betting/draw logic produced false "in sync" because they fell outside any marker's 10-line window. Strongest single-run evidence that the fixed window + permissive placement silently misses drift.
- **Run 5** — Proposed `drift lint` placement check (warn if no `func`/`class`/`def` line lies within the hashed window, or if marker is far from the next declaration).
- **Run 5** — Suggested `drift init` emit a 2-line marker-placement hint on stdout (the strategic implication of the 10-line window is easy to miss).
- **Run 5** — Normalize help-flag handling (`--help`/`-h`/`help`/no-arg as strict aliases; the subject burned 3 exploratory calls).
- **Run 5** — Detect duplicate/no-op `drift link` and emit "already linked" (subject re-linked `ai_player`).
- **Run 5** — `drift todo --strict` or a probe that re-evaluates synced edges, to surface the drift→reset loop to subjects who only ever see "in sync".

## Prioritized recommendations (consolidated)

1. **[High]** Replace or augment the fixed 10-line hash window with a semantic region (hash to end of enclosing function/block, or `// D! id=… fn=name` anchoring) — flagged by runs 3 (positional cascade drift) and 5 (marker-coarseness false "in sync" on real edits). This is the tool's largest correctness gap: it can silently miss the very drift it exists to catch.
2. **[High]** Add `drift diff <marker|spec>` (or a verbose `drift todo`) showing old-vs-new hash and the changed 10-line content / spec text — runs 0, 1, 2, 3, 4. Highest-frequency request; directly serves the core "verify alignment" loop.
3. **[High]** Show spec description text inline in `drift list` (or `drift list --verbose` / `drift links --verbose`) — runs 0, 1, 4, 5. Closes the consistent-but-wrong-link blind spot that hash-based `todo` cannot see.
4. **[High]** Fix or document the `line="0"` reporting for specs in `drift.pin` and `drift list` — runs 0, 3, 4. Either record the real `<spec>` element line or drop the `:0` suffix for specs.
5. **[High]** Add a `drift lint` / placement-check that warns when a marker's hashed window contains no `func`/`class`/`def` line or sits far from the next declaration — runs 1 (validate framing), 5 (placement framing). Pushes cold-start agents toward correct placement.
6. **[Medium]** Add a read-only `drift inspect` / `drift show` / `drift hash` command printing a marker's hashed region + linked spec text + link state — runs 0, 1, 2, 4. Aids debugging without hand-parsing `drift.pin`.
7. **[Medium]** Add `--json` output to `drift todo`, `drift list`, `drift link` for reliable LLM-agent parsing — runs 0, 1, 2.
8. **[Medium]** Add `drift check` / `drift verify` (all-linked, no-orphans, in-sync, non-zero exit on failure) for CI / agent gating — runs 0, 4.
9. **[Medium]** Add `drift reset --all` / bulk resolve for multi-edge refactors — runs 2, 3.
10. **[Medium]** Improve `drift reset` UX: emit a `✓ Resolved…` confirmation line on success and clarify "collapses baselines" semantics in `drift skill` — runs 2, 3.
11. **[Medium]** Document edge cases & hash-window semantics in `drift skill` (deleted markers, removed specs, duplicate IDs, marker-line inclusion, adjacent markers, `line=` auto-update, what "collapses baselines" means) — runs 0, 2, 3, 4.
12. **[Medium]** Clarify spec-ID vs. marker-ID qualification convention in `skill`/`help` ("inside `<module name="X">` write `id="local"`; the CLI uses `X.local`") — runs 0, 1, 4.
13. **[Medium]** Add `--dry-run` to `drift reset` (preview which baselines change) — run 3.
14. **[Low]** Add per-subcommand `--help` (e.g. `drift link --help`) — run 4.
15. **[Low]** Normalize help-flag handling (`--help`/`-h`/`help`/no-arg as strict aliases) — run 5.
16. **[Low]** Detect duplicate / no-op `drift link` and emit "already linked" — run 5.
17. **[Low]** Have `drift init` emit a 2-line marker-placement hint on stdout — run 5.
18. **[Low]** Provide a self-contained `drift demo` / `drift init --demo` to teach the drift→reset cycle in <1 min (runs 0 and 4 never exercised drift/reset organically) — run 4.
19. **[Low]** Add `drift --version` / changelog output — run 4.
20. **[Low]** Clarify `drift.ignore` with concrete examples (patterns, negation, comments) — run 4.
21. **[Low]** Consider `drift auto-link` for single-module projects — run 0.
22. **[Low]** Add example workflows (modify → todo → reset transcript) to `drift skill` — run 3.
23. **[Low]** Consider `drift validate` semantic-plausibility pass (token overlap between spec and marker code) as an opt-in complement to `drift lint` — run 1.
24. **[Low]** Normalize/relativize stored spec/marker paths in `drift.pin` and document the resolution rule — run 1.

## Next steps

The consolidated evidence points to one overriding issue: **driftpin's drift detection is not robust to marker placement, and the tool gives agents little help placing markers well or understanding *what* drifted.** Two of six runs (3 and 5) demonstrated the tool either over-reporting drift (positional cascade from an import shift) or silently **missing** real drift (markers placed tens of lines from the implementing logic, with three material edits producing "in sync"). Both failure modes share a root cause: the fixed 10-line positional hash window.

The tool authors should, in priority order:

1. **Rework the hashing model** (recommendation #1) so a marker anchors to a semantic region — either to the end of the enclosing function/declaration, or via an explicit `fn=`/`scope=` annotation. This is the single change that would have prevented the most serious observed defect (run 5's false "in sync" on real betting/draw edits).
2. **Add `drift diff` and inline spec text in `drift list`** (#2, #3). Together these turn the tool from a binary "drifted / not drifted" signal into a reviewable change summary, which is what every subject ultimately needed and several had to reconstruct by hand from `drift.pin`.
3. **Add a placement/lint check** (#5) so coarse placement is flagged at link time rather than discovered (or, worse, never discovered) after a refactor.
4. **Fix the `line="0"` spec reporting** (#4) — small, but it was independently flagged in three runs and reads as a bug to first-time users.
5. **Round out agent ergonomics**: `--json`, `drift check`, `drift reset --all` + confirmation line, and an "edge cases" section in `drift skill`. These are individually low-effort but were each raised by 2–3 runs and would materially smooth the agent loop.

A useful follow-up evaluation would deliberately vary (a) marker placement quality and (b) edit distance from the marker, to measure how often the current 10-line window produces false "in sync" under realistic cold-start agent behavior — the present batch hints this is common but only run 5 produced direct evidence. Additionally, two of the six tasks (runs 0 and 4) were setup-only and never exercised drift/reset; future task designs should force the modify→drift→reset cycle so the detection half of the value proposition is always observed.
