# Plan: Rename Driftpin → Drift

Breaking rename of the project's Go module, on-disk state directory, spec file extension, internal identifiers, container config, and docs from "Driftpin" / "driftpin" / ".driftpin" / "*.pin.xml" / "filament" to "Drift" / "drift" / ".drift" / "*.drift.xml" / "drift".

Pre-release, no external users. No backward-compat shim.

---

## Decisions (locked)

- **Scope:** Rename the project name AND purge "pin" from filenames/identifiers. `*.pin.xml` → `*.drift.xml`, `pinstore` → `statestore`, `PinState` → `State`, etc.
- **State dir:** `.driftpin/` → `.drift/` (clean break, no read-fallback).
- **History:** Leave `eval/runs/**` and `observations/0001–0007*.md` frozen. Add a top-of-file rename note to each frozen historical doc. For `eval/runs/`, a single `eval/runs/RENAME_NOTE.md` at its root.
- **Planning docs:** DELETE `PLAN.md`, `PLAN-UPDATED.md`, `PLAN-DIFF_VERIFICATION.md`. They are replaced by this file.
- **DOCUMENTATION.md:** Fix stale `drift.pin` → `.drift/state.xml` references as part of this pass.
- **filament → drift:** Full pass over `.devcontainer/*`, `.opencode/skills/write-code.md`, `.gitignore`. Host repo dir stays; config self-heals on next container rebuild.
- **Go package naming:** `statestore` (not `driftstore` — avoids redundancy with module `drift`).
- **`pinned` / `pinnedByID`:** → `baselined` / `baselinedByID` (semantic: "has a stored baseline hash"; "drifted" would mean "changed", which is wrong).
- **State-store field/local:** field `pin` → `stateStore` (Option B); local `state` unchanged. Avoids ambiguity with the existing `baselines` store field and with method-local `state` shadowing.
- **Marker shortcodes** (`pload`/`psave`/`pbase` etc.): skip — cosmetic, low value.
- **Verify-then-resolve loop** (replaces blind regeneration): manually check each drifted edge with `drift diff` / `drift show` before running `drift reset`.

---

## Identifier mapping (Go)

| Old | New | Notes |
|---|---|---|
| `module driftpin` | `module drift` | go.mod + 38 import strings across 17 .go files |
| `pinstore/` package | `statestore/` | dir rename + 6 importers + package decls |
| `PinState` | `State` | |
| `PinStore` | `StateStore` | interface |
| `FilePinStore` | `FileStateStore` | |
| `NewFilePinStore` | `NewFileStateStore` | |
| `ErrPinNotFound` | `ErrStateNotFound` | + update message `.driftpin/` → `.drift/` |
| `pinFileXML` (pinstore) | `stateFileXML` | parses state.xml |
| `pinFileXML` (scanner) | `specFileXML` | parses *.drift.xml — distinct name from pinstore's to avoid two same-named types |
| `pinFile` (scanner/content.go) | `specFile` | |
| field/local `pin` (StateStore-typed) | `stateStore` | `o.stateStore.Load()`; method-local `state` unchanged |
| `pinned` / `pinnedByID` | `baselined` / `baselinedByID` | orchestrator reconcile funcs |
| `pinPath` (tests) | `stateFilePath` | avoids collision with `statePath()` method |
| `initMainPinXML` | `initMainDriftXML` | + `//go:embed` directive + file rename |
| `writeMainPin` | `writeMainDrift` | test helper |
| `fakePinStore` | `fakeStateStore` | test fake |
| `AssertPinStateEquals` | `AssertStateEquals` | testutil |
| `EvaluatedStateToPinState` | `EvaluatedToState` | testutil |

Test function / subtest names containing `Pin` or `pin` (e.g. `TestFilePinStoreRoundTrip`, `init_creates_drift_pin`, `spec_in_pin_*`, `marker_in_pin_*`, `pin_load_error`) are renamed in lockstep.

---

## File / directory mapping

| Old | New |
|---|---|
| `*.pin.xml` | `*.drift.xml` (25 project files + 3 eval fixtures) |
| `main.pin.xml` | `main.drift.xml` (scanner.go:84,86 hardcode + cli.go:185) |
| `.driftpin/` | `.drift/` (prod: cli.go:37,46,48 + statestore/pin_file.go:42,46; 16 test sites; 3 eval fixtures; drift.ignore) |
| `<import path="./x/y.pin.xml"/>` | `.drift.xml` inside every spec file |
| `cmd/drift/` | unchanged (already correct) |
| git branch `driftpin` | delete (currently on `main`) |

---

## Spec text edits (self-coverage .drift.xml)

In `cli/cli.drift.xml`, `orchestrator/orchestrator.drift.xml`, `statestore/statestore.drift.xml`, `core/core.drift.xml`:

- `driftpin` → `drift`
- `.driftpin/` → `.drift/`
- `drift.pin` → `.drift/state.xml`
- `pinned specs/markers` → `baselined specs/markers`

These are the tool's own spec coverage — editing spec text changes baseline hashes, which is exactly what the verify-then-resolve loop (below) reconciles.

---

## User-facing docs

- `README.md` — title, prose, path refs
- `DOCUMENTATION.md` — fix stale `drift.pin` → `.drift/state.xml` throughout + product name
- `cli/skill.md` — product name + all `.driftpin/`, `*.pin.xml` refs
- `cli/help.txt` — path refs
- `Makefile` — already builds `drift`; no change needed

Product "Drift" (capital D) vs concept "drift" (lowercase). Rephrase tight collisions where product and concept appear in the same sentence and capitalization alone is ambiguous.

---

## Eval harness (functional, not history)

- `eval/pipeline.go` — 9 prompt-string literals (lines 257, 268, 272, 282, 286 x2, 311, 317, 324, 513)
- `eval/README.md` — heading + 2 prose lines (1, 3, 35)
- `eval/agents/eval-judge.md` (lines 2, 27, 30), `eval/agents/eval-subject.md` (line 2)
- `eval/prompts/{bad-link,code-refactor,apply-existing,drift-detection}.md` — task prompts
- 3 eval fixtures: rename `.driftpin/` → `.drift/` and `main.pin.xml` → `main.drift.xml` + update import refs inside

---

## filament → drift pass

- `.devcontainer/devcontainer.json` — `"name": "filament"` → `"drift"`
- `.devcontainer/docker-compose.yml` — `container_name`, label, workspace mount path (host dir stays; config self-heals on next rebuild)
- `.devcontainer/attach.sh` — label + comment
- `.opencode/skills/write-code.md` — all `filament` → `drift` (also fixes the pre-existing broken `filament.spec.xml` reference → `drift.drift.xml` convention)
- `.gitignore` — drop `filament`; anchor `drift` → `/drift` (matches only the built binary, not `cmd/drift/` source — also fixes a pre-existing ripgrep-hidden-source bug)

---

## History (frozen, with rename note)

Add a top-of-file rename note to:
- `observations/0001-phase1-baseline.md` through `observations/0007-phase6-rangemodel.md`
- `eval/runs/RENAME_NOTE.md` (single file at `eval/runs/` root; do not touch the 34 run directories or `log.csv`)

Note text:

> This file/directory predates the Driftpin → Drift rename (breaking change). Historical references to "Driftpin" / "driftpin" / ".driftpin" / "*.pin.xml" / "filament" are retained as-is.

---

## Execution order

1. **Go mechanical** — module + 38 imports + `pinstore/` → `statestore/` dir + identifier rename (incl. the distinct `specFileXML` / `stateFileXML` split). `go build ./...` green.
2. **`.drift/` path literals** — prod (cli.go, pin_file.go) + 16 test sites + `drift.ignore`.
3. **`*.drift.xml` file renames** — 25 files + 3 eval fixtures + scanner hardcode + `<import>` refs + `//go:embed` + file rename for `init_main.pin.xml`.
4. **Self-coverage spec text edits** in the 4 affected `.drift.xml` files.
5. **User-facing docs** — README, DOCUMENTATION, skill.md, help.txt.
6. **Eval harness + fixtures** — pipeline.go prompts, eval/README, eval/agents, eval/prompts, 3 fixture dirs.
7. **filament → drift pass** — `.devcontainer/*`, `.opencode/skills/write-code.md`, `.gitignore`.
8. **History notes** — top-of-file headers + `eval/runs/RENAME_NOTE.md`.
9. **Verify-then-resolve loop** (see below).
10. **Delete git branch `driftpin`**.

---

## Verify-then-resolve loop (replaces blind regeneration)

The committed `.drift/state.xml` baseline hashes are stale after the spec-text edits (content changed → old hashes no longer match). The resolve loop updates them — but only after manual review confirms the rename preserved spec↔code alignment.

1. `make build`
2. `go build ./...` and `go test ./...` green
3. `rg -i "driftpin|\.pin\.xml|filament" --no-ignore -g '!eval/runs/**' -g '!observations/**'` returns empty (PLAN files are deleted, so excluded from this check)
4. `./drift todo` — enumerates every drifted edge (specs whose text changed + markers whose code changed)
5. **For each edge:**
   - `./drift diff <marker> <spec>` — side-by-side baseline vs current for both sides
   - `./drift show <id>` — full review
   - Confirm the rename preserved spec↔code alignment. For self-coverage specs, both sides changed (spec text + implementing code); confirm logic intact.
6. `./drift reset <marker> <spec>` per edge once verified — updates baseline hashes to new content. Without this, `state.xml` stays stale and `drift todo` reports perpetual drift.
7. Commit final `.drift/state.xml` + `.drift/baselines/`.

---

## Things deliberately out of scope

- Marker shortcodes `pload` / `psave` / `pbase` / `pnope` / `pspc` / `pmrk` / `pdone` — mnemonic "p" prefix. Cosmetic; renaming touches all `D!` markers + state.xml for low value. Skipped.
- Host repo directory rename (`/workspaces/filament` → `/workspaces/drift`) — host-level operation. Config self-heals on next container rebuild.
- `eval/runs/**` and `observations/0001–0007` content — frozen historical records (the latter records this very naming issue as a finding in `0003`).
