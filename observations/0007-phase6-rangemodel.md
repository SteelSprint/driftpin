# Observation 0007 — phase6-rangemodel

Date: 2026-07-16
Runs:
- `/workspaces/filament/eval/runs/phase6-rangemodel-0`
- `/workspaces/filament/eval/runs/phase6-rangemodel-1`
- `/workspaces/filament/eval/runs/phase6-rangemodel-2`
- `/workspaces/filament/eval/runs/phase6-rangemodel-3`
- `/workspaces/filament/eval/runs/phase6-rangemodel-4`
- `/workspaces/filament/eval/runs/phase6-rangemodel-5`

## Known issues

No run was tainted, sandbox-escaped, or otherwise compromised. All 6 workspaces were evaluated cleanly and their findings are eligible for convergent analysis. Two methodology caveats are worth recording (neither invalidates a run):

- **Run 1 (phase6-rangemodel-1): task was easier than described.** The seeded "subtle, consistent-but-wrong" link (`palindrome_func -> main.reverse`) actually orphaned the `main.palindrome` spec, so `drift list` loudly tagged it `[unlinked]`. The discovery signal was therefore structural, not semantic. A true semantic-swap (two markers cross-linked to each other's specs) would be invisible to both `drift todo` and `drift list`. The run is still valid; it simply under-tested the failure mode it claimed to test. This is a harness/task-design note, not a tool defect.
- **Run 5 (phase6-rangemodel-5): self-debrief honesty gap.** The debrief framed `drift reset` as hypothetical ("I wondered if I needed it after init") when the transcript shows a real drift→reset cycle triggered by a `Run()` refactor. The deliverable and scoring were unaffected, but the debrief understated the most informative moment of the run. Flagged for judge methodology awareness; no data excluded.

No other harness issues, escapes, or tainted runs were observed across the batch.

## Convergent findings

Themes appearing in 2 or more runs. Run numbers refer to the `phase6-rangemodel-N` suffix.

| Theme | Runs | Priority |
|---|---|---|
| `drift diff` / surface *what changed* on a drifted edge (currently only *that* it changed) | 2, 3, 4, 5 | High |
| `drift.pin` stores absolute paths and silently rewrites them on `reset` (portability + git-noise hazard) | 2, 3 | High |
| `--json` machine-readable output for `drift list`/`todo`/`show` | 0, 4 | High |
| `drift init` idempotency/overwrite safety AND self-sufficiency of its next-steps output | 0, 4, 5 | High |
| Command options/flags (`--verbose`) and exit codes documented only in `drift skill`, not in `drift --help` | 1, 2, 5 | High |
| Structural `drift validate`/`drift verify` pass distinct from content-drift detection | 0, 4, 5 | Medium |
| `--dry-run` for mutating commands (`link`/`unlink`/`reset`) | 0, 2, 3, 4, 5 | Medium |
| Reconcile `drift skill` ↔ `drift help`/`--help` documentation (single source of truth) | 1, 2, 3 | Medium |
| Clarify `drift reset` semantics (single-arg orphan cleanup vs two-arg edge resolution; what baseline is updated) | 0, 2, 3 | Medium |
| `--version` flag / build metadata (+ embed format version in `drift.pin`) | 3, 4, 5 | Medium |
| `drift link --all`/`--auto` convention-based bulk linking (chained `&&` linking scales poorly) | 4, 5 | Medium |
| Refactoring-scenarios + "what does satisfied mean" compliance guidance in `drift skill`/`drift todo` | 2, 3 | Medium |
| `drift status` dashboard combining list+todo+health in one read | 0, 5 | Medium |
| Publish `drift.pin` schema for debugging/migration/tooling | 0, 4, 5 | Low |
| Reduce module-vs-main root + dot-asymmetry cognitive load (auto-resolve link by local ID, synonym roots) | 2, 4 | Low |
| `drift skill` / `drift help` should agree on the `list` command signature and surface `--verbose` | 1, 4, 5 | Low |

## Divergent findings

Run-specific observations that did not converge across the batch.

- **Run 0** — The `line="0"` placeholder attribute on specs in `drift.pin` and in `drift todo`/`drift show` output (`calc.pin.xml:0`) reads as a bug to a careful user. Either track the real `<spec>` source line or omit the attribute. Also flagged: `drift skill` advertises that `drift init` creates an `example.go` which it does not (doc/code drift inside the tool itself); and a request for `drift add-spec`/`drift add-marker` subcommands to remove the highest-friction hand-XML-editing step.
- **Run 1** — The tool's blind spot for *semantic* link swaps: a "wrong but consistent" link is invisible to `drift todo`, and only detectable via `drift list` when it happens to orphan a spec. A true cross-link swap would be fully invisible. Subject requested a heuristic `drift check`/`drift list --audit` (token-overlap, multi-marker, orphan detection). Also: dot-vs-underscore naming inconsistency (`main.reverse` spec vs `reverse_func` marker) caused minor cognitive load.
- **Run 2** — Single-arg `drift reset <id>` (orphan cleanup) semantics are only in `drift skill`, not in `--help`, creating risk of trying `drift reset convert_func` to resolve a live drift. Subject also noted the absolute-path issue (convergent with Run 3) and a lack of any refactoring-guidance section in `drift skill`.
- **Run 3** — "Compliance guidance" gap: when code changes but the spec is still arguably true, the tool gives no hint whether to reset or edit the spec first. Subject reasoned correctly but flagged the risk of rubber-stamping real semantic drift. Also requested partial/ambiguous ID disambiguation for `drift show` (e.g. `drift show div` lists candidates).
- **Run 4** — Module-vs-main root distinction (`<main>` vs `<module name="...">`) and the "spec IDs have exactly one dot, marker IDs have none" rule were the most noted conceptual friction; subject suggested allowing `<module name="main">` as a synonym and auto-resolving `drift link <marker>` when the marker ID unambiguously matches one spec's local ID. Also suggested breaking the wall-of-text `drift skill` into navigable topics (`drift skill markers`, `drift skill specs`, …).
- **Run 5** — `drift init` output says only "edit main.pin.xml" and never mentions placing `D!` markers or running `drift link`; init is the first touchpoint and shouldn't require separately discovering `skill`. Also flagged undocumented XML feature set (comments/CDATA/attributes), file-discovery rules, and concurrency model for `drift.pin`; and proposed `drift init --from-existing` to auto-stub specs from pre-placed markers in brownfield code.

## Prioritized recommendations (consolidated)

Merged, deduplicated, and prioritized across all 6 runs. Run citations indicate which reports flagged each item.

1. **[High]** Add `drift diff <marker|spec>` (or inline diff in `drift todo`) showing the actual content delta between baseline and current marker/spec text. `drift todo` currently reports *that* something changed but never *what*; agents must re-read whole regions and guess at SHA-1s. — Runs 2, 3, 4, 5
2. **[High]** Store repo-relative paths in `drift.pin` and stop silently rewriting absolute paths on `reset`. Absolute paths are non portable across machines/CI runners/checkouts and produce meaningless churn when the state file is committed. Resolve paths relative to `drift.pin` at read time. — Runs 2, 3
3. **[High]** Add `--json` output to `drift list`, `drift todo`, and `drift show`. Stable machine-readable schema paired with the existing 0/1/2 exit-code convention unlocks CI gating and agent scripting without prose-scraping. — Runs 0, 4
4. **[High]** Document and harden `drift init`: make it refuse (or require `--force`) when `drift.pin` already exists, and make its post-run output self-sufficient by mentioning marker placement, `drift link`, and pointing to `drift skill`. Init is the first touchpoint — it should not require separately discovering the rest of the workflow. — Runs 0, 4, 5
5. **[High]** Surface command options (`--verbose`) and exit codes in `drift --help`, not only inside the long `drift skill` guide. Agents that stop at `--help` currently miss `drift list --verbose` and the exit-code semantics entirely. — Runs 1, 2, 5
6. **[Medium]** Add a structural `drift validate`/`drift verify` pass, separately exit-coded from drift detection: unpaired markers, dotted/duplicate marker IDs, missing spec files, orphaned entries. Today these are silent or conflated with content drift in `drift todo`. — Runs 0, 4, 5
7. **[Medium]** Add `--dry-run` to `drift link`, `drift unlink`, and `drift reset` to preview mutations to the tool-managed `drift.pin` before commit. Especially valuable for batch-linking (subjects chained 6–9 `link` calls with no preview). — Runs 0, 2, 3, 4, 5
8. **[Medium]** Reconcile `drift skill` ↔ `drift help`/`--help` into a single source of truth for command signatures (notably `list --verbose` and single-arg vs two-arg `reset`). Divergent docs cause agents to misremember flags. — Runs 1, 2, 3
9. **[Medium]** Clarify `drift reset` semantics in `--help`: single-arg `reset <id>` is *only* orphan cleanup; two-arg `reset <marker> <spec>` resolves a drifted edge and updates the baseline; state explicitly what gets rewritten (marker baseline, spec baseline, or both). — Runs 0, 2, 3
10. **[Medium]** Add `--version` and embed a `formatVersion` attribute in `drift.pin` for reproducible bug reports and future migrations. — Runs 3, 4, 5
11. **[Medium]** Add `drift link --all`/`--auto` for convention-based bulk linking (link marker `foo` to the unique spec whose local ID is `foo`, erroring only on ambiguity). Eliminates tedious manual wiring on consistent-naming projects. — Runs 4, 5
12. **[Medium]** Add a refactoring-scenarios + compliance-guidance section to `drift skill` and a one-line hint in `drift todo` drift output ("if the spec is still accurate, reset; if not, edit the `*.pin.xml` first"). Covers the most common LLM-agent use case and the most common reasoning failure (rubber-stamping real semantic drift). — Runs 2, 3
13. **[Medium]** Add a `drift status` dashboard combining `list` + `todo` + structural health in one read so agents avoid two round-trips. — Runs 0, 5
14. **[Low]** Publish the `drift.pin` schema (root `<drift>`, `<specs>`, `<markers>`, `<links>`, `<resolutions>` with hash/filepath/line attributes) even though it is tool-managed — aids debugging, migration, and third-party tooling. — Runs 0, 4, 5
15. **[Low]** Reduce module-vs-main and dot-asymmetry cognitive load: allow `<module name="main">` as a synonym for `<main>`, and auto-resolve `drift link <marker>` when the marker ID unambiguously matches one spec's local ID. — Runs 2, 4
16. **[Low]** Add spec-writing guidance (granularity + description quality, one good/bad example) and a "common mistakes" section to `drift skill`, including the note that `drift todo` will not flag a wrong-but-consistent link. — Runs 0, 1
17. **[Low]** Break the wall-of-text `drift skill` into navigable topics (`drift skill markers|specs|workflow|edge-cases`) to reduce context-window cost. — Run 4
18. **[Low]** Fix the `line="0"` placeholder for specs in `drift.pin` and `drift todo`/`drift show` output — either track the real `<spec>` source line or omit the attribute. — Run 0
19. **[Low]** Reconcile `drift skill`'s claim that `drift init` creates an `example.go` with actual behavior (ship the file or drop the claim). Doc/code drift inside a drift-tracking tool undermines first-contact trust. — Run 0
20. **[Low]** Add a heuristic semantic link audit (`drift check` / `drift list --audit`) to flag specs linked to markers with no token overlap, or multi-marker specs where another marker better matches — catches the "wrong but consistent" swap class that `drift todo` cannot see. — Run 1
21. **[Low]** Allow `drift show` to accept partial/ambiguous IDs with disambiguation to speed interactive exploration. — Run 3
22. **[Low]** Document the XML feature set (comments/CDATA/attributes), `*.pin.xml` file-discovery rules, and `drift.pin` concurrency/locking model in `drift skill`. — Run 5
23. **[Low]** Consider `drift init --from-existing` to auto-stub specs from pre-placed `D!` markers in brownfield code. — Run 5

## Next steps

The convergent signal is unambiguous and concentrated. The tool's *detection* loop is sound — every subject reached a fully synced state with zero tool errors, and the binary is genuinely self-describing for cold use via `drift --help` + `drift skill`. The remaining gaps are almost entirely in *resolution ergonomics* and *machine-readability*, not in the core model. I recommend the authors triage in this order:

1. **Ship `drift diff` first (recommendation #1).** It is the single highest-leverage, lowest-risk addition: it appeared in 4 of 6 runs, directly serves the detect→verify→resolve loop the tool is built around, and requires no model changes. Until agents can see *what* changed, the verify step is the slowest and most error-prone part of the workflow.
2. **Fix `drift.pin` path portability (#2) and add `--json` (#3) together.** Both unblock CI gating and committed-state workflows; both are prerequisite to driftpin being usable outside a throwaway sandbox. The absolute-path rewrite-on-`reset` behavior is a latent bug that will surface the moment state files are committed.
3. **Make `drift init` safe and self-sufficient (#4) and surface `--verbose`/exit codes in `--help` (#5).** These are cheap doc/UX fixes that close the most-reported onboarding friction and stop agents from missing powerful affordances because they stopped at `--help`.
4. **Then tackle the Medium structural items as a bundle:** `drift validate` (#6), `--dry-run` (#7), `reset` semantics clarity (#9), `--version` (#10), `drift link --all` (#11), and the refactoring/compliance guidance (#12). These are the items that generalize the tool from "works on a single edge" to "works on real multi-edge refactors and in CI."
5. **Defer the Low items** (schema publication, module-root synonyms, `drift skill` topic-split, `--from-existing`, semantic link audit) until the above land; they are credible but lower-leverage, and several depend on the schema/JSON work above being settled first.

One methodology note for the eval harness authors: **re-seed Run 1's task as a true cross-link swap** so that every spec remains linked and `[synced]` while two edges are semantically wrong. The current setup accidentally makes the "subtle" case trivial via the `[unlinked]` tag, which means the batch under-tested the one failure mode (semantic link drift) that the tool currently has no answer for. Adding that case would also generate direct evidence for or against recommendation #20 (the heuristic link audit).
