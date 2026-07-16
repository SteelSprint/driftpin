# Plan

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  CLI (cli.go)                                                 │
│  drift init                                                   │
│  drift todo                                                    │
│  drift list                                                   │
│  drift link <marker> <module.spec>                            │
│  drift unlink <marker> <module.spec>                          │
│  drift reset <marker> <module.spec>                           │
│  drift reset <id>           (orphan cleanup)                  │
│  drift help / drift skill                                     │
├──────────────────────────────────────────────────────────────┤
│  Orchestrator                                                 │
│  load pin → scan → reconcile → build ctx → core              │
│  → save (reset/link/unlink only)                              │
├─────────────────────────┬────────────────────────────────────┤
│  PinStore               │  Scanner                           │
│  read/write drift.pin   │  follow main.pin.xml imports →     │
│  (XML codec)            │  discover specs (module-qualified)  │
│                         │  validate spec/marker ID format     │
│                         │  walk dir tree → discover markers   │
│                         │  hash content → produce ScanResult  │
├─────────────────────────┴────────────────────────────────────┤
│  Core (core.go)                                               │
│  pure, stateless                                            │
│  EvaluateState(ctx) → EvaluatedState                        │
│  - drift detection (including deletion = drift)              │
│  - collapse (prune deleted nodes after resolution)            │
└──────────────────────────────────────────────────────────────┘
```

## Module system

Specs live in `.pin.xml` files. Each file is a module. Files import each other forming a DAG. The scanner starts at `main.pin.xml` (the entry point) and follows imports transitively to discover specs.

### File format

Entry point (`main.pin.xml`) — pure manifest, no specs:
```xml
<main>
  <import path="./core.pin.xml" />
  <import path="./utils.pin.xml" />
</main>
```

Entry point with direct specs (implicit `"main"` module):
```xml
<main>
  <spec id="validate_input">Input MUST be validated.</spec>
</main>
```

Module file:
```xml
<module name="core">
  <import path="./utils.pin.xml" />

  <spec id="validate">
    Validation MUST reject duplicates. Validation
    <ref spec="utils.hash">hashes</ref> each spec's content.
  </spec>
</module>
```

### Rules

- Each `.pin.xml` has exactly one root: `<main>` or `<module name="...">`.
- `<main>` is the entry point. Found by convention in the working directory.
- `<main>` with direct `<spec>` children gets implicit module name `"main"`.
- `<import path="..."/>` resolves relative to the file containing the import.
- Both `<main>` and `<module>` use `<import>`. One keyword, one operation.
- Imports trigger file loading. Transitive: A imports B, B imports C → all loaded.
- Explicit visibility: a module can only `<ref>` specs from modules it directly imports.
- Diamond imports: same file loaded once, deduplicated by absolute path.
- Duplicate module names across the graph: error.
- Cycles: hard error with trace (`main → subA → subB → subA`).
- Spec IDs are per-module. Referenced as `module.specid` (dot-qualified).
- **Spec local IDs must not contain a dot.** Dots are reserved for module qualification. The scanner rejects spec IDs containing dots.
- **Marker shortcodes must not contain a dot.** Dots are reserved for spec ID qualification. The scanner rejects marker IDs containing dots.
- `<ref>` elements are not parsed by the scanner in this phase. They are part of spec content and get hashed as-is. Ref-based drift is a future steel cable.

### ID format invariants

These two invariants enable unambiguous disambiguation in CLI commands:
- **Spec IDs contain exactly one dot**: `module.localId`. The local `id` attribute in `<spec>` must not contain a dot.
- **Marker IDs contain no dots**: bare shortcodes only.

This allows `drift reset <id>` (single-arg orphan cleanup) to determine: dot → spec, no dot → marker.

### Monorepo / multiworkspace

Each workspace (directory with `main.pin.xml`) is an independent drift context:

```
services/auth/
  main.pin.xml       ← entry point
  drift.pin          ← auth's baselines, links, resolutions
  drift.ignore       ← (optional) auth's marker scan exclusions

services/payments/
  main.pin.xml
  drift.pin          ← payments' baselines, links, resolutions
  drift.ignore

shared/
  common.pin.xml     ← no drift.pin, no drift.ignore — just a spec source
```

Running `drift todo` in `services/auth/` uses `services/auth/main.pin.xml` and `services/auth/drift.pin`. Specs can be imported from outside the workspace (`../../shared/common.pin.xml`), but markers are only scanned within the workspace directory tree.

## Decisions

| Decision | Choice |
|---|---|
| drift.pin format | XML (stdlib `encoding/xml`, zero deps) |
| Hash function | SHA1 hex-encoded |
| Missing drift.pin | `drift init` required first |
| CLI output | Match DOCUMENTATION.md exactly |
| Test doubles | Hand-written fakes |
| Testing | Red/green, exhaustive arity, clamped validations |
| Build approach | Walking skeleton / steel cable, end-to-end per iteration |
| Spec files | `*.pin.xml` with `<module>` or `<main>` root |
| Module declaration | `<module name="...">` — one per file |
| Entry point | `<main>` — found by convention in cwd |
| Import syntax | `<import path="..."/>` — relative to importing file |
| Spec IDs | Module-qualified: `module.specid` (e.g., `core.validate`) |
| Spec local ID format | Must not contain a dot (scanner rejects) |
| Spec struct | `Spec.ID` = full qualified string. `Spec.Module` = module name. |
| drift.pin storage | Module not stored separately — derived from qualified ID. |
| Refs | `<ref spec="module.specid">text</ref>` — inline in prose, hashed as content. Not parsed for drift yet. |
| Markers | `D! id=<shortcode>` comment lines in code files. Shortcodes are bare (not module-qualified). |
| Marker ID format | Must not contain a dot (scanner rejects) |
| Marker-to-spec links | `drift link <marker> <module.spec>` — space-separated. |
| drift.ignore | Applies to marker discovery only (code files). Spec discovery is via imports. |
| Marker hashing | Next 10 lines from marker line |
| Deleted spec/marker | Treated as drift (not error). Sentinel hash `""` in scan. Surfaces as todo. Pruned after `drift reset`. |
| Orphan (deleted, no links) | Shows as `[deleted]` in `drift list`. Cleaned via `drift reset <id>` (single-arg). |
| Stale entry handling | No hard errors. Reconciler keeps stale entries. Deletion flows through normal drift→reset workflow. |

## XML format for drift.pin

```xml
<drift>
  <specs>
    <spec id="core.validate" hash="afd4321ea69c..." filepath="core.pin.xml" line="0"/>
  </specs>
  <markers>
    <marker id="cval" hash="7dc34f7516f4..." filepath="core.go" line="108"/>
  </markers>
  <links>
    <link specId="core.validate" markerId="cval"/>
  </links>
  <resolutions>
    <resolution specId="core.validate" markerId="cval" currentSpecHash="..." currentMarkerHash="..."/>
  </resolutions>
</drift>
```

Spec IDs are stored as fully qualified strings (`core.validate`). Module is derivable by splitting on the first dot — not stored as a separate attribute.

## Scanner interface

```go
type ScanResult struct {
    Specs   []Spec   // ID is qualified ("core.validate"), Module is "core"
    Markers []Marker // ID is bare shortcode ("cval")
}

type Scanner interface {
    Scan() (ScanResult, error)
}
```

### Spec discovery

1. Look for `main.pin.xml` in the working directory. Error if not found.
2. Parse root element: `<main>` or `<module name="...">`.
3. If `<main>` with direct `<spec>` children: implicit module name `"main"`.
4. Follow `<import path="...">` relative to the importing file.
5. Track visiting stack (by absolute path) for cycle detection.
6. Track loaded files (by absolute path) for dedup.
7. Track module names (by string) for duplicate detection.
8. Each spec: `Module` = module name, `ID` = `Module + "." + localID`.
9. **Validate**: local ID must not contain a dot. Error if it does.
10. Hash: SHA1 of trimmed inner content (including any `<ref>` elements).

### Marker discovery

1. Walk working directory tree for code files (`.go`, `.py`, `.js`, etc.).
2. Apply `drift.ignore` patterns.
3. Find lines matching `D! id=<shortcode>` pattern.
4. **Validate**: shortcode must not contain a dot. Error if it does.
5. SHA1 hash the next 10 lines from the marker line.
6. Record: shortcode (bare ID), filepath, line number, hash.

## Orchestrator reconciliation

On `drift todo` / `drift reset`:
1. Load `PinState` from `drift.pin`
2. Get `ScanResult` from scanner (specs via import graph, markers via dir walk)
3. **Reconcile specs**: for each in ScanResult:
   - In PinState → keep baseline hash, update filepath/line if changed
   - NOT in PinState → new, baseline = current hash (no drift)
   - In PinState but NOT in ScanResult → **keep in reconciled list** (stale entry, baseline preserved). No error. Will be treated as drift via sentinel hash.
4. Same for markers
5. Build `Scan` from ScanResult hash maps. For stale specs/markers (in reconciled list but not in scan), add sentinel hash `""`.
6. Build `CoreAlgorithmContext` with reconciled specs/markers + links/resolution from PinState
7. Run core

## Deletion = drift model

When a spec or marker is deleted from disk but still referenced in drift.pin:

### With links (common case)
1. Reconciler keeps the stale entry with baseline hash preserved
2. `buildScan` adds sentinel hash `""` for the stale entry
3. `computeTodoList`: `scan.SpecHashes[link.SpecID] == ""` → `specChanged = true`, `SpecDeleted = true`
4. `drift todo` shows drift with deletion-specific message: "The spec term has been deleted from disk. If this was intentional, run `drift reset <marker> <spec>` to acknowledge the removal."
5. `drift reset <marker> <spec>` resolves the edge
6. `collapseResolvedNodes`: when scan hash is `""`, **delete the node** from the map (instead of updating baseline). Also remove all its links.
7. `pin.Save`: pruned spec/marker/links are no longer in drift.pin → clean state

### Without links (orphan case)
1. Reconciler keeps the stale entry
2. `buildScan` adds sentinel hash `""`
3. No links → `computeTodoList` generates no todos for it
4. `drift todo` does not mention it (no edge to drift)
5. `drift list` shows it with `[deleted]` tag
6. User runs `drift reset <id>` (single-arg) to clean up
7. Orchestrator removes the entry from pin state + saves

### `drift reset <id>` (single-arg orphan cleanup)

- `drift reset <id>` where `id` contains a dot → look up as spec
- `drift reset <id>` where `id` has no dot → look up as marker
- If found and stale (not on disk, no links) → remove from drift.pin
- If found but still on disk → error: `"%q is still on disk; nothing to remove"`
- If found but has links → error: `"%q still has N links; resolve them first with drift reset <marker> <spec>"`
- If not found → error: `"no spec/marker %q found in drift.pin"`

## CLI commands

| Command | What it does |
|---|---|
| `drift init` | Creates empty drift.pin + starter main.pin.xml |
| `drift todo` | Scans → reconciles → runs core → outputs todos (read-only) |
| `drift list` | Shows all specs, markers, links, sync state (read-only) |
| `drift link <marker> <module.spec>` | Validates + adds link to drift.pin |
| `drift unlink <marker> <module.spec>` | Removes link + resolution from drift.pin |
| `drift reset <marker> <module.spec>` | Resolves a drifted edge, collapses baselines |
| `drift reset <id>` | Removes an orphaned (deleted, no links) spec/marker from drift.pin |
| `drift help` | Prints command reference |
| `drift skill` | Prints comprehensive guide for LLM agents |

## File structure

```
cmd/drift/main.go        # entry point (imports cli)
core/
  core.go                # pure algorithm (drift detection, collapse, deletion handling)
  core_test.go           # exhaustive tests
  core.pin.xml           # specs for validate, todo, reset, collapse, edge, scan, deleted_drift
scanner/
  scanner.go             # import graph + marker discovery + ID format validation
  scanner_test.go        # module format, import graph, cycle detection, ID format
  scanner.pin.xml        # specs for discovery, hashing, duplicates, ID format
pinstore/
  pin_file.go            # XML codec for drift.pin
  pin_file_test.go       # round-trip tests
  pinstore.pin.xml       # specs for load, save, not_found
orchestrator/
  orchestrator.go        # reconcile, init, todo, reset, link, unlink
  orchestrator_test.go   # fakes-based tests
  orchestrator.pin.xml   # specs for init, todo, reset, link, unlink, reconcile
cli/
  cli.go                 # command dispatch + formatters
  cli_test.go            # end-to-end CLI tests
  cli.pin.xml            # specs for dispatch, help, skill, format, reset, link, unlink, list
  help.txt               # embedded help text
  skill.md               # embedded comprehensive guide
  init_main.pin.xml      # embedded starter template
internal/testutil/       # shared test helpers
  testutil.go
  fixtures.go            # markerLine() — excluded from drift scan
main.pin.xml             # entry point importing all project modules
drift.pin                # rebuilt after each phase
drift.ignore             # excludes testutil/fixtures.go, examples/, eval/
eval/                    # LLM-as-judge eval pipeline
  main.go                # parallel battery runner with --runs flag
  pipeline.go            # stage/subject/judge/surface/synthesize
  agents/                # eval-subject.md, eval-judge.md
  prompts/               # task prompts + fixture directories
    drift-detection.md   # prompt: detect drift after code change
    drift-detection/     # fixture: pre-made calculator + drift.pin
    bad-link.md          # prompt: find and fix wrong link
    bad-link/            # fixture: pre-made project with wrong link + drift.pin
    code-refactor.md     # prompt: refactor triggers drift, resolve
    code-refactor/       # fixture: pre-made temp converter + drift.pin
    apply-existing.md    # prompt: add specs to existing code
    library.md           # prompt: greenfield library
    small-cli.md         # prompt: greenfield CLI
observations/            # filed observation records (auto-numbered)
```

## Phase history

### Phase 1-3: ✓ DONE

- Core algorithm, pin store, scanner (import graph), orchestrator, CLI
- Many-to-many topologies, module/import system
- Self-describing binary (`drift help`, `drift skill`, `drift init`)
- `drift unlink`, `drift list`, per-subcommand `--help`
- `drift todo` wording: "No changes detected."
- All tests pass, vet clean, gofmt clean

### Phase 4: ✓ DONE

- Fixture-based eval cases (drift-detection, bad-link, code-refactor)
- Parallel eval pipeline with `--runs` flag
- Observation 0005 filed: 5 High-priority findings

### Phase 5: Stale entry handling + ID format invariants

**Problem**: Deleting a spec/marker from disk hard-fails all `drift todo`/`drift list` commands with no recovery path except hand-editing drift.pin.

**Solution**: Treat deletion as drift (not error). Deletion flows through the normal drift→reset workflow. Orphans (deleted, no links) are cleaned via `drift reset <id>`.

#### Implementation

1. **ID format validation** (scanner.go + scanner.pin.xml)
   - Spec local IDs must not contain dots → scanner rejects with error
   - Marker shortcodes must not contain dots → scanner rejects with error
   - New specs: `scanner.spec_id_format`, `scanner.marker_id_format`
   - New markers: `sidfmt`, `midfmt`
   - Update `drift skill`: document the dot invariant

2. **Stale entry handling** (orchestrator.go + core.go + cli.go)
   - `reconcileSpecs`/`reconcileMarkers`: stop hard-erroring, keep stale entries
   - `buildScan`: add sentinel `""` hash for stale entries
   - `Todo` struct: add `SpecDeleted bool`, `MarkerDeleted bool`
   - `computeTodoList`: set deleted flags when scan hash is `""`
   - `collapseResolvedNodes`: when scan hash is `""`, delete node + its links
   - `formatTodo`: deletion-specific message
   - `formatList`: `[deleted]` tag for stale entries
   - Updated specs: `orch.reconcile_specs`, `orch.reconcile_markers`, `core.scan_coverage`, `core.collapse`, `core.edge_unchecked`, `core.todo_action`, `cli.format_todo`, `cli.list_format`
   - New spec: `core.deleted_drift`, marker: `cdeld`

3. **`drift reset <id>` orphan cleanup** (orchestrator.go + cli.go)
   - Single-arg reset: dot → spec, no dot → marker
   - Remove orphan from pin state if stale (not on disk, no links)
   - Error if still on disk or has links
   - Updated specs: `cli.reset_format`, `cli.dispatch`, `orch.reset`
   - New spec: `cli.reset_orphan`, marker: `crorph`

4. **Promote `drift skill` in help** (help.txt)
   - Add "First time? Run `drift skill` for the full guide." at top
   - Updated spec: `cli.help`

5. **Fixtures: drop setup.sh** (eval/prompts/ + pipeline.go)
   - Remove `setup.sh` from all 3 fixtures
   - Commit pre-built `drift.pin` directly in each fixture directory
   - Add `go.mod` to fixtures that need it
   - Remove setup.sh execution code from `copyFixture` in pipeline.go

6. **Rebuild + verify**
   - Rebuild drift.pin with all new specs/markers/links
   - Run tests, vet, gofmt
   - `drift todo` must be clean

7. **Re-run evals**
   - Run 4 cases (apply-existing, bad-link, code-refactor, drift-detection)
   - File observation 0006

#### New/updated tests (~22)

| Test | What it verifies |
|---|---|
| Scanner: spec local ID with dot → error | `scanner.spec_id_format` |
| Scanner: marker ID with dot → error | `scanner.marker_id_format` |
| Scanner: spec local ID without dot → OK | No regression |
| Scanner: marker ID without dot → OK | No regression |
| Spec deleted with links → drift detected | Reconciler keeps stale, todo shows drift |
| Spec deleted with links → reset → pruned | After reset, spec + links gone |
| Marker deleted with links → drift detected | Same for markers |
| Marker deleted with links → reset → pruned | Same |
| Spec deleted no links → drift list shows [deleted] | Zombie visible |
| Spec deleted no links → drift reset <id> → pruned | Orphan cleanup |
| Marker deleted no links → drift reset <id> → pruned | Same |
| drift reset <id> on live spec → error | Can't remove something on disk |
| drift reset <id> on spec with links → error | Must resolve links first |
| drift reset <id> nonexistent → error | Clear error |
| drift todo shows deletion message | formatTodo message correct |
| CLI: drift reset with 1 arg | Correct dispatch |
| CLI: drift reset with 0 args → usage showing both forms | Usage error |
| CLI: drift list shows [deleted] tag | formatList correct |
| Existing reset tests still pass | No regression |
| Existing reconcile tests updated (stale no longer errors) | No regression |
| Existing scanner tests still pass | No regression |

#### Spec/marker ledger

| New specs | File |
|---|---|
| `scanner.spec_id_format` | scanner.pin.xml |
| `scanner.marker_id_format` | scanner.pin.xml |
| `core.deleted_drift` | core.pin.xml |
| `cli.reset_orphan` | cli.pin.xml |

| Updated specs | File |
|---|---|
| `cli.reset_format` | cli.pin.xml |
| `cli.dispatch` | cli.pin.xml |
| `cli.help` | cli.pin.xml |
| `cli.format_todo` | cli.pin.xml |
| `cli.list_format` | cli.pin.xml |
| `orch.reset` | orchestrator.pin.xml |
| `orch.reconcile_specs` | orchestrator.pin.xml |
| `orch.reconcile_markers` | orchestrator.pin.xml |
| `core.scan_coverage` | core.pin.xml |
| `core.collapse` | core.pin.xml |
| `core.edge_unchecked` | core.pin.xml |
| `core.todo_action` | core.pin.xml |

| New markers | File | Spec linked to |
|---|---|---|
| `sidfmt` | scanner.go | scanner.spec_id_format |
| `midfmt` | scanner.go | scanner.marker_id_format |
| `cdeld` | core.go | core.deleted_drift |
| `crorph` | cli.go | cli.reset_orphan |

#### Implementation order

1. ID format validation (scanner.go + scanner.pin.xml) — simplest, no dependencies
2. Stale entry handling (reconciler + buildScan + core) — the core behavior change
3. Orphan reset (orchestrator + cli) — depends on #2
4. Help/skill updates (help.txt, skill.md) — no dependencies
5. Fixtures (drop setup.sh, add drift.pin + go.mod) — independent
6. Rebuild drift.pin — after all code changes
7. Tests + vet + gofmt — verify
8. Commit
9. Re-run evals

## Future steel cables

### Steel cable 8: Ref-based drift

Parse `<ref>` elements in spec content. Implement dual-hash model:
- Self hash: hash of spec content excluding resolved refs
- Composite hash: hash including resolved ref content
- Markers link to composite hash
- Drift output distinguishes "you changed this" vs "a dependency changed"

### Steel cable 9: AST

Replace flat prose specs with structured AST nodes. Each node hashable independently. Markers link to specific AST nodes, not whole specs. See design discussion notes.
