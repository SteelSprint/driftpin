# Three-Mode Output Layer + Command Dispatcher

Status: planned, awaiting implementation.
Scope: refactor `cli.Run` into a dispatcher-driven pipeline producing typed `Result`s, rendered by one of three `Presenter` implementations (Plain / Color / JSON). Add `--json`, `--no-color`, `--color={auto,always,never}` as global flags on every command. Collapse three sources of command metadata into a single Registry.

This plan is drift-tracked. Implementation phases add markers linking code to the new specs in `cli/output/`.

---

## 1. Surface

Every command — `init`, `todo`, `list`, `show`, `diff`, `link`, `unlink`, `reset`, `help`, `skill`, `version` — accepts three global flags:

| Flag | Effect |
|---|---|
| `--json` | Output a single JSON object per command (schema per command). |
| `--no-color` | Force Plain output. |
| `--color={auto,always,never}` | Explicit color control. `auto` = default. |

Default mode = **Color** when stdout is a TTY and `NO_COLOR` is unset; otherwise Plain. Piped/redirected output is always Plain.

`version` moves out of the `main.go` pre-check (`cmd/drift/main.go:14`) into the dispatch pipeline as `VersionCommand`, uniform with every other command.

---

## 2. Architecture

```
cmd/drift/main.go
    │  selectOutput(args, stdout, env) → (Options, Presenter)
    │  RunWithRender(args, ".", opts, presenter) → (string, int)
    │  fmt.Println(output)
    ▼
cli.RunWithRender
    │  1. stripGlobalFlags(args)         → (--json/--no-color/--color= removed)
    │  2. uniform pre-checks:            help_flag, unknown_flag_rejection
    │  3. dispatcher.Lookup(args[0])     → Command
    │  4. ctx := CommandContext{Args, Dir, Orch}
    │  5. result, code := recoverWrap(cmd, ctx)   ← enforces ErrorResult on panic
    │  6. return presenter.Render(result), code
    ▼
cli/commands/*.go           ← one struct per command, implements Command interface
cli/output/                 ← Result types, Presenter interface, 3 presenter impls
cli/registry.go             ← map[string]Command — single source of truth
cli/help_template.go        ← drift help generator (replaces help.txt)
cli/fragments.go            ← shared prose constants used by all presenters + help
```

Three strict layers, one direction of dependency. Commands produce data, never strings. Presenters own all presentation.

---

## 3. Locked decisions

| Decision | Choice |
|---|---|
| Output modes | `default` (color) / `--no-color` (plain) / `--json` |
| Command→formatter boundary | Design Y — `RunWithRender` returns typed `Result`; `main.go` picks presenter and calls `presenter.Render(result)` |
| Presenter count | 3 visitor implementations (Plain, Color, JSON), each with one method per command |
| `Run(args, dir) (string, int)` | Unchanged signature; thin wrapper over new `RunWithRender(args, dir, opts, presenter)` — keeps ~50 test sites green |
| Color strategy | Independent from raw Result data; shares prose via `cli/fragments.go` constants. NOT a wrapper over Plain |
| Guardrail invariant | `stripANSI(ColorPresenter.X(r)) == PlainPresenter.X(r)` for every Result `r` and method `X`, enforced by battery test |
| Plain fidelity | Byte-identical to today — keeps existing `cli.drift.xml` specs and CLI tests valid through Landings 1–2 |
| Dispatcher | W1 — `Command` interface, one struct per command in `cli/commands/`; `Registry` collapses `help.txt` + `subcommandHelpTexts` + `recognizedFlags` into one source |
| `version` command | Moves into dispatch as `VersionCommand` → `VersionResult` |
| Flag precedence | `--json` > `--no-color` > `--color=never` > `NO_COLOR` > non-TTY > default (Color). `--color=always` overrides NO_COLOR and non-TTY |
| Help/skill under `--json` | Pass-through as `{"text": "..."}` |
| Execution | Landings 1–2 (behavior-identical structural refactor) ship before any new mode. Then JSON (3), Color (4), spec hygiene (5) |
| Zero-dependency | Preserved — ANSI codes, TTY detection, JSON via Go stdlib only |

---

## 4. Spec organization (multi-level, drift-tracked)

Three sibling modules, one per level. Each lives in `cli/output/`. All imported from `main.drift.xml`. They cross-reference each other in prose using fully-qualified IDs (`module.spec`).

```
main.drift.xml                       (adds 3 imports)
  └── cli/output/
        output_intent.drift.xml      module: output_intent      Level 1 — intent & outcomes (WHY)
        output.drift.xml             module: output             Level 2 — behaviors (WHAT)
        output_impl.drift.xml        module: output_impl        Level 3 — code units (WHERE/HOW)
  └── cli/cli.drift.xml              updates in place           Levels 2 & 3 mixed (existing)
```

**Level discipline:**
- **L1 (`output_intent.*`)** — Outcomes and commitments. No code, no flags, no JSON shapes. Readable by a product stakeholder.
- **L2 (`output.*`)** — Observable behaviors. Modes, flag precedence, palettes, JSON schemas, invariants. Readable by an integration tester writing behavior tests.
- **L3 (`output_impl.*`)** — Code-unit organization. Package layout, interface signatures, file responsibilities. Readable by a maintainer navigating the codebase.

Cross-refs flow downward: L1 → L2 → L3. L2 never cites L3 for behavior; L3 cites L2 for "implementation of".

### Spec inventory

**`output_intent` (Level 1) — 11 specs**

| ID | One-liner |
|---|---|
| `audiences` | Three distinct consumers (interactive human, piping human, software); one format can't serve all. |
| `interactive_default` | Default in a terminal is color — drift must be legible at a glance. |
| `machine_readable` | LLMs/scripts need structured JSON, not regex over human prose. |
| `pipe_safe` | Piped/redirected output must never contain ANSI — enforced by TTY detection, not opt-in. |
| `user_override` | `--no-color`/`--json`/`--color=always` let users override auto-selection. |
| `explicit_contract` | Every mode of every command is specified; no "implementation-defined" outputs. |
| `error_uniformity` | Errors are first-class output with defined shape in every mode, never bare strings. |
| `metadata_single_source` | Command metadata lives in one place (Registry), not three. |
| `uniform_dispatch` | Every command flows through the same pipeline; no command bypasses it (incl. `version`). |
| `zero_dependency_preserved` | Output layer adds no deps; ANSI/TTY/JSON all stdlib. |
| `self_hosting_preserved` | Output layer is drift-tracked like everything else; `drift todo` clean before commit. |

**`output` (Level 2) — 20 specs**

| Group | Specs |
|---|---|
| Mode selection (6) | `modes`, `color_default`, `tty_detection`, `global_flags`, `global_flag_stripping`, `flag_precedence` |
| Plain (1) | `plain_mode` |
| Color (3) | `color_mode`, `color_palette`, `guardrail_property` |
| JSON (3) | `json_mode`, `json_schema`, `json_field_ordering` |
| Result & errors (3) | `result_types`, `error_result_invariant`, `error_hint_convention` |
| Dispatch (4) | `dispatch_pipeline`, `command_interface`, `uniform_prechecks`, `version_in_dispatch` |

**`output_impl` (Level 3) — 9 specs**

| ID | One-liner |
|---|---|
| `package_layout` | New `cli/output/` + `cli/commands/` packages; `cli/command.go`, `registry.go`, `help_template.go`, `fragments.go`. |
| `presenter_interface` | Presenter interface in `cli/output/presenter.go`; one method per Result type; three implementations. |
| `result_types` | Sealed Result interface in `cli/output/result.go`; ~10 concrete types wrapping orchestrator/core data. |
| `plain_presenter` | `cli/output/plain.go` — byte-identical migration of today's `format*` functions. |
| `color_presenter` | `cli/output/color.go` — independent impl from Result data; uses `fragments.go`; ANSI per palette. |
| `json_presenter` | `cli/output/json.go` — `encoding/json` with struct-driven field ordering; per-command schema. |
| `command_interface_impl` | `cli/command.go` — `Command`/`CommandContext`/`CommandMeta`/`recoverWrap` signatures. |
| `registry` | `cli/registry.go` — `var Registry = map[string]Command{...}`; derived helpers; replaces `help.txt` + two maps. |
| `help_generator` | `cli/help_template.go` — `GenerateHelp(registry)`; static prose from `fragments.go`; deletes `help.txt`. |
| `fragments` | `cli/fragments.go` — shared prose constants (tags, statuses, templates, helpers). |
| `guardrail_test` | `cli/output/color_test.go` — battery asserting `stripANSI(Color.X(r)) == Plain.X(r)`. |

**Updates to existing `cli/cli.drift.xml` — 9 specs touched**

| Spec | Change |
|---|---|
| `dispatch` | Note `version` now dispatched; add global_flag_stripping as third uniform pre-check. |
| `unknown_flag_rejection` | Note `--json`/`--no-color`/`--color=` stripped before this check; never rejected. |
| `format_todo` | Cross-ref: Plain-specific; Color via `output.color_palette`; JSON via `output.json_schema`. |
| `reset_format`, `link_format`, `unlink_format`, `list_format` | Same cross-ref clause as `format_todo`. |
| `show_command`, `diff_command` | Same cross-ref clause. |
| `help` | Note help text generated from Registry (`output_impl.help_generator`); byte-identical to current. |

**New spec in `cli/cli.drift.xml` — 1 added**

| Spec | Content |
|---|---|
| `version_command` | `drift version` returns VersionResult; plain = `"drift version <X>"`, JSON = `{"version":"<X>"}`, exit 0. |

### Sample spec text (one per level, representative)

**L1 — `output_intent.pipe_safe`:**
> When drift output is piped, redirected, or consumed by another process, color codes MUST NOT appear in the output. ANSI escape sequences corrupt pipelines, files, and structured parsers. This is non-negotiable: pipe-safety is enforced by automatic TTY detection, not by user opt-in. A user should never have to remember `--no-color` when running `drift todo | grep marker` — drift detects the non-TTY stdout and emits plain. See `output.tty_detection` for the mechanism and `output.flag_precedence` for the override precedence.

**L2 — `output.guardrail_property`:**
> For every Result `r` and every Presenter method `X`, `stripANSI(ColorPresenter.X(r))` MUST equal `PlainPresenter.X(r)` byte-for-byte. This is the correctness invariant that permits Color to be implemented independently from Plain (see `output.color_mode`) rather than as a wrapper. It is enforced by a battery test in `cli/output/color_test.go` covering: synced case, drifted marker, drifted spec, both-changed, deleted spec, deleted marker, no-baseline, multi-edge expanded, `--all` with multiple edges, empty state, unlinked warning, and every OkResult/ErrorResult/TextResult/VersionResult shape. If Color and Plain ever diverge in layout (not just ANSI), this test fails. See `output_impl.guardrail_test` for the test contract.

**L3 — `output_impl.registry`:**
> `cli/registry.go` exports `var Registry = map[string]Command{...}` mapping command names to Command instances (one entry per command in `cli/commands/`). Provides derived helpers `subcommandHelp(name string) (string, bool)` returning `cmd.Meta().Usage`, and `recognizedFlagsFor(cmd string) map[string]bool` built from `cmd.Meta().Flags`. The Registry is the SINGLE source of truth for command metadata; `cli/help.txt`, `subcommandHelpTexts` (cli/cli.go:197), and `recognizedFlags` (cli/cli.go:216) are deleted. See `output_intent.metadata_single_source` for the commitment and `output_impl.command_interface_impl` for the Command interface shape.

---

## 5. Result types & Presenter interface

```go
// cli/output/result.go
type Result interface{ isResult() }

type TodoResult         struct{ State core.EvaluatedState }
type ListResult         struct{ State core.EvaluatedState; Verbose bool }
type ShowResult         struct{ IsSpec bool; Spec *core.Spec; Marker *core.Marker; Linked []LinkedEntity; Contents map[string]string }
type DiffEdgeResult     struct{ Result orchestrator.DiffResult }
type DiffExpandedResult struct{ ID string; Edges []orchestrator.DiffResult }
type DiffAllResult      struct{ State core.EvaluatedState; Edges []orchestrator.DiffResult }
type OkResult           struct{ Command, Message string }
type ErrorResult        struct{ Command, Message, Hint string; Exit int }
type TextResult         struct{ Text string }   // help/skill passthrough
type VersionResult      struct{ Version string }
```

```go
// cli/output/presenter.go
type Presenter interface {
    Todo(TodoResult)         string
    List(ListResult)         string
    Show(ShowResult)         string
    DiffEdge(DiffEdgeResult) string
    DiffExpanded(DiffExpandedResult) string
    DiffAll(DiffAllResult)   string
    Ok(OkResult)             string
    Error(ErrorResult)       string
    Text(TextResult)         string
    Version(VersionResult)   string
}
```

Three implementations: `PlainPresenter`, `ColorPresenter`, `JSONPresenter`. `main.go` does a type switch on the returned `Result` and dispatches to the matching Presenter method.

---

## 6. JSON schema (per command)

```jsonc
// todo
{ "ok": true, "specs": N, "markers": N, "links": N,
  "todos": [{ "marker": "id", "spec": "mod.id",
              "markerLocation": "file:line", "specLocation": "file:line",
              "markerChanged": bool, "specChanged": bool,
              "markerDeleted": bool, "specDeleted": bool }],
  "unlinkedMarkers": N }

// list
{ "specs":   [{ "id": "...", "filepath": "...", "deleted": bool, "unlinked": bool, "text"?: "..." }],
  "markers": [{ "id": "...", "filepath": "...", "startLine": N, "endLine": N,
                "deleted": bool, "unlinked": bool, "preview"?: "..." }],
  "links":   [{ "marker": "...", "spec": "...", "status": "synced"|"drifted" }] }

// show
{ "kind": "spec"|"marker", "id": "...", "filepath": "...", "hash": "...",
  "lines"?: "start-end", "content": "...", "linked": [ {...entity...} ] }

// diff (single edge — matches orchestrator.DiffResult shape)
{ "spec":   { "id", "filepath", "lines", "baselineHash", "currentHash",
              "hasBaseline": bool, "deleted": bool,
              "baseline": "...", "current": "...", "patch": "..." },
  "marker": { ...same shape... } }

// diff expanded / --all
{ "id"?: "...", "edges": [ {...single-edge...} ] }

// init / link / unlink / reset
{ "ok": true, "command": "...", "message": "..." }

// help / skill
{ "text": "..." }

// version
{ "version": "..." }

// errors
{ "ok": false, "error": "...", "hint"?: "...", "exit": N }
```

---

## 7. Color palette

| Pattern | ANSI | Code |
|---|---|---|
| diff `+` lines | green | 32 |
| diff `-` lines | red | 31 |
| hunk `@@` headers | cyan | 36 |
| `[DRIFTED]`, `[deleted]`, error messages | red | 31 |
| `[TODO]`, `[unlinked]`, drift status lines | yellow | 33 |
| `[synced]`, `Status: in sync`, `No changes detected` | green | 32 |
| section headers `=== Spec/Marker: ... ===` | bold | 1 |
| `→ Run 'drift ...'` hints | cyan | 36 |

Bold = `\x1b[1m`; colors use `\x1b[Nm`; reset = `\x1b[0m`. Defined in `cli/output/ansi.go`.

---

## 8. Implementation phases (five landings)

Each landing is independently reviewable and revertable. Each ends with `go test ./...` green and `drift todo` clean.

### Landing 1 — Plain refactor (switch intact, behavior-identical)

- Create `cli/output/` package
- Define `Result` sealed interface + concrete types (incl. `VersionResult`)
- Define `Presenter` interface
- Extract `cli/fragments.go` with shared prose constants
- Implement `PlainPresenter` by migrating `format*` verbatim from `cli/cli.go`
- Add `RunWithRender(args, dir, opts, presenter) (string, int)`
- Convert `Run` into thin wrapper: `return RunWithRender(args, dir, DefaultOptions(), PlainPresenter{})`
- Convert every `return str, code` to typed `Result`; every `return err.Error(), N` to `ErrorResult{...}`
- Add `cli/output/plain_test.go` with golden assertions lifted from existing test cases

**Exit:** Plain byte-identical; existing CLI tests untouched.

### Landing 2 — Command interface + Registry collapse (behavior-identical)

- Define `Command`/`CommandContext`/`CommandMeta`/`recoverWrap` in `cli/command.go`
- Create `cli/commands/*.go` — one struct per command (`InitCommand`, `TodoCommand`, …, `VersionCommand`)
- Move each case body from `RunWithRender`'s switch into the corresponding command's `Run` method
- Each command's `Meta()` returns name/short/usage/flags (migrating `subcommandHelpTexts` content)
- Replace `subcommandHelpTexts` map with `subcommandHelp(name)` derived from Registry
- Replace `recognizedFlags` map with `recognizedFlagsFor(cmd)` derived from `cmd.Meta().Flags`
- Replace `cli/help.txt` with `cli/help_template.go` generator; static prose in `fragments.go`
- Move `version` from `main.go` pre-check into `VersionCommand` in dispatch
- Verify behavior-identical (existing tests still green)

**Exit:** metadata duplication eliminated; switch replaced by Registry lookup.

### Landing 3 — JSONPresenter + `--json`

- Implement `JSONPresenter` in `cli/output/json.go` per §6 schema
- Global flag pre-pass: `stripGlobalFlags(args)` for `--json` only at this stage
- `main.go`: parse `--json`, select `JSONPresenter` when set
- Add `cli/output/json_test.go`: parse via `encoding/json`; assert required keys per command; verify `patch` field matches `internal/diff.UnifiedDiff`; verify error shape has `ok:false` + `exit`

**Exit:** `--json` works on every command; plain/color paths unchanged.

### Landing 4 — ColorPresenter + TTY + remaining global flags

- Implement `ColorPresenter` independently in `cli/output/color.go` (uses `fragments.go`)
- `cli/output/ansi.go` — ANSI code constants + helper `Style(text string, codes ...string) string`
- `cli/output/tty.go` — `IsTerminal(f *os.File) bool` (via `os.File.Stat` + `os.ModeCharDevice`); `ColorEnabled(stdout, env, override) bool` implementing §3 precedence
- Add guardrail test in `cli/output/color_test.go`: `stripANSI(ColorPresenter.X(r)) == PlainPresenter.X(r)` battery
- Extend `stripGlobalFlags` to handle `--no-color`, `--color={auto,always,never}`
- `main.go`: `selectOutput` with full precedence — `--json` > `--no-color` > `--color=never` > `NO_COLOR` > non-TTY > Color
- Default mode flips from Plain to Color (TTY-gated)
- Update `unknown_flag_rejection` spec

**Exit:** default output becomes color; round-trip guardrail green for full battery.

### Landing 5 — Spec hygiene + markers

- Add the three new spec modules: `output_intent`, `output`, `output_impl`
- Update `main.drift.xml` to import them
- Update `cli/cli.drift.xml` specs per §4 (`dispatch`, `unknown_flag_rejection`, `format_*`, `show_command`, `diff_command`, `help`)
- Add new `cli.version_command` spec
- Place markers in new code files for each new spec
- `drift link` every new marker to its spec
- Run `drift todo` clean

**Exit:** `drift todo` clean; all new specs linked to implementing code.

---

## 9. Sizing

| Component | Rough LOC |
|---|---|
| Result types + Presenter interface | ~150 |
| PlainPresenter (migrated `format*`) | ~400 moved |
| ColorPresenter (independent impl) | ~450 |
| JSONPresenter | ~350 |
| ANSI + TTY helpers | ~80 |
| Command interface + dispatcher + recoverWrap | ~150 |
| Per-command files (×11) | ~600 |
| Registry + help_template + fragments | ~250 |
| Tests (plain/color/json/registry) | ~800 |
| Spec files (3 new modules + cli.drift.xml edits) | ~600 |
| **Total** | **~3,800 added/moved** |

---

## 10. Self-hosting

This plan follows `principles.self_hosting` (README:133): the output layer is itself drift-tracked. Specs land before code; code is linked to specs; `drift todo` is a hard commit gate throughout.

Adding ~40 new specs in three new modules changes `drift list` output (many new unlinked specs visible) but does NOT trigger `drift todo` drift (specs without markers don't produce todos). As landings ship, markers get placed and linked. By end of Landing 5, all specs are linked.

`principles.red_before_green` applies: bugs found during the refactor are fixed test-first.

`principles.zero_dependency_portable` is preserved: TTY detection, ANSI emission, and JSON serialization all use Go stdlib only. No `golang.org/x/term`, no `fatih/color`.

`principles.specs_are_truth` applies: if implementation and spec disagree during development, the spec is truth — fix the code, not the spec.
