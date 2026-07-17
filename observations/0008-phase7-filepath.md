# Observation 0007 — phase7-filepath

Date: Fri Jul 17 2026
Runs: `phase7-filepath` (single run in this batch)

## Known issues

- **Batch size = 1.** This batch contains only one subject run (`phase7-filepath`). Convergence analysis is therefore weak by construction: any theme present in the run is, by definition, present in "all runs." The convergent table below should be read as "themes confirmed in the single run," not as multi-run validation. Recommendations are still carried forward because they are independently verifiable against the tool itself.
- **No harness issues, sandbox escapes, or tainted runs were reported.** The subject transcript shows 32 tool calls, all completing on the first attempt with no retries or errors; `go build ./...` and `go run -race` both passed clean. The run is considered valid for analysis.
- **One self-debrief inaccuracy (not a taint).** The subject's `self-debrief.md` over-generalized that "`drift link --help` or `drift todo --help` doesn't provide subcommand-specific help." The judge verified that `drift link --help` *does* print help; only `drift todo --help` silently runs the command. This is a subject-side observation error, not a tool/harness compromise, so the run is retained — but the underlying tool inconsistency (inconsistent `--help` handling) is itself a real finding and is carried into recommendations.

## Convergent findings

| Theme | Runs | Priority |
|---|---|---|
| Inconsistent `--help`/`-h` handling across subcommands (`link --help` works; `todo --help` runs the command) | phase7-filepath | High |
| No structured `--json` output for LLM-agent consumption of `todo`/`list` | phase7-filepath | High |
| `drift todo` is silent about unlinked markers (common forgotten-link failure mode) | phase7-filepath | High |
| Unknown long-flags silently parsed as positional args (`drift diff --all` → `--all` treated as marker ID); root cause shared with the `--help` inconsistency | phase7-filepath | Medium |
| Cold-start discoverability leans heavily on `drift skill`; top-level `drift help` is "command-focused" with no quick-start workflow block | phase7-filepath | Medium |
| No unified `drift status` command combining `list` (state) + `todo` (drift); agents pay two commands' token cost for a periodic check | phase7-filepath | Medium |
| No `drift link`-time validation of marker/spec IDs (dots, `module.spec` format), leading to silently unresolvable edges | phase7-filepath | Medium |
| `drift skill` lacks a worked end-to-end drift→diff→reset transcript; resolution workflow not tangible before first `reset` | phase7-filepath | Medium |
| No `drift diff --all` audit view (currently per-edge only; `--all` mis-parses today) | phase7-filepath | Low |
| `.drift/state.xml` schema undocumented ("do not edit" with no schema sketch) | phase7-filepath | Low |
| Cosmetic `line="0"` emitted for every spec entry in `state.xml` (looks like a bug at a glance) | phase7-filepath | Low |

## Divergent findings

(Run-specific observations from `phase7-filepath` that did not have other runs to corroborate. With a single-run batch these are necessarily run-specific; they are recorded here for traceability rather than as converged evidence.)

- **phase7-filepath** — The subject achieved textbook-clean drift usage on the first attempt: `drift --help` → `drift skill` → TodoWrite plan mirroring init→specs→markers→link→todo → 6 links all `[synced]`, `drift todo` exit 0. This suggests the current `drift skill` guide is sufficient for a capable agent to cold-start correctly, but it does not prove the same for less capable models.
- **phase7-filepath** — Specs were authored *before* code and markers were placed at semantically meaningful boundaries (each public method, the constructor, the `main.go` demo). The single deliberate unlinked marker (`generate_code`, wrapping the internal `generateCode` helper) was an implementation detail with no spec counterpart — a defensible choice the subject disclosed honestly in its debrief.
- **phase7-filepath** — The subject invoked drift as `../drift` from inside `urlshortener/` and framed drift's directory-sensitivity (`.drift/` discovered from cwd upward) as a "pitfall" rather than articulating the discovery invariant. This is a documentation/discovery nuance, not a tool bug: the behavior is correct but not clearly communicated as an invariant.
- **phase7-filepath** — `drift skill` (139 lines) was comprehensive enough that the subject never needed external docs and reached correct usage by the second tool call. The cold-use weakness was confined to subcommand-help consistency (see convergent theme above), not to overall onboarding depth.
- **phase7-filepath** — `drift diff --all` produced `no linked edges found for "--all"`, confirming that long-flags are not lexed as flags but consumed positionally. This is the same root cause as the `drift todo --help` misbehavior and would be fixed by a single flag-parsing pass before subcommand dispatch.

## Prioritized recommendations (consolidated)

1. **[High]** Make `--help`/`-h` a first-class flag parsed before subcommand dispatch for *every* subcommand. Verified inconsistency: `drift link --help` prints help, `drift todo --help` silently runs the command. This single fix also resolves part of the subject's debrief over-generalization. — phase7-filepath
2. **[High]** Warn on unlinked markers in `drift todo` (one-line summary, unchanged exit code), e.g. "1 unlinked marker found — run `drift list` to review." Catches forgotten-link mistakes without penalizing intentional unlinked helpers. — phase7-filepath
3. **[High]** Add `--json` output mode to `drift todo` and `drift list` (structured `specs[]`, `markers[]`, `links[]`, `drift[]`). LLM agents are the stated target audience; parsing "6 specs, 7 markers, 6 links in sync" with regex is fragile. — phase7-filepath
4. **[Medium]** Reject unknown long-flags instead of treating them as positional args. Verified: `drift diff --all` → `no linked edges found for "--all"`; `drift todo --help` ignored. A general "unknown flag: --all" error prevents silent misinterpretation and is the shared root cause behind recommendation #1. — phase7-filepath
5. **[Medium]** Add a `drift status` unified command combining `list` (what exists) with `todo` (what drifted). One command is cheaper in prompt tokens and round-trips for agent periodic checks. — phase7-filepath
6. **[Medium]** Add a 5-line quick-start workflow block (`init → write specs → place markers → link → todo`) to top-level `drift help` so simple tasks can skip `drift skill` and lower cold-start cost. — phase7-filepath
7. **[Medium]** Validate marker/spec IDs at `drift link` time: reject marker IDs containing dots, validate `module.spec` format on the spec side, emit a clear early error. Prevents confusing downstream state with silently unresolvable edges. — phase7-filepath
8. **[Medium]** Add a worked drift→diff→reset example with realistic output to `drift skill`. Makes the resolution cycle tangible and reduces agent hesitation before first `reset`. — phase7-filepath
9. **[Low]** Add `drift diff --all` to show every edge (synced + drifted) for auditing (currently per-edge only; today's `--all` mis-parses per #4). — phase7-filepath
10. **[Low]** Document the `.drift/state.xml` schema in `drift skill` (specs/markers/links/resolutions) or mark it explicitly opaque, so advanced users can debug/trust the tool even if editing remains discouraged. — phase7-filepath
11. **[Low]** Stop emitting `line="0"` for spec entries in `state.xml`; omit the attribute for specs or use a sentinel like `line="-1"` to avoid looking like a bug. — phase7-filepath

## Next steps

The single run in this batch produced a clean, high-confidence signal despite the batch-size limitation, because the subject exercised drift end-to-end and the judge independently verified each tool-behavior claim against the live binary. The highest-leverage action for the tool authors is to **fix flag parsing once, holistically**: implementing recommendation #4 (reject unknown long-flags via a pre-dispatch flag pass) will simultaneously resolve #1 (uniform `--help`), unblock #9 (`drift diff --all` as a real flag rather than a positional), and remove the root cause that tripped even this capable subject into an over-generalized debrief claim. That is a small, well-scoped change with outsized UX impact and should be the first item triaged.

After flag parsing, the next two priorities are the agent-facing output work: **`--json` for `todo`/`list` (#3)** and the **unlinked-marker warning in `todo` (#2)**. Both directly serve drift's stated LLM-agent audience and are independently verifiable. The Medium-tier documentation additions (#6 quick-start block, #8 worked reset example) are cheap to author and should be bundled into the next `drift skill` revision.

To strengthen future convergence evidence, the authors should re-run this filepath/URL-shortener task (and ideally a second, larger task) with **2–3 subjects per batch** so that the themes above can be corroborated across runs rather than resting on a single observation. The current batch is sufficient to justify the High-priority fixes but not to rank-order the Medium/Low items with confidence.
