# Plan

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  CLI (cli.go)                                                 │
│  drift init                                                   │
│  drift todo                                                    │
│  drift reset <marker> <module.spec>                           │
│  drift link <marker> <module.spec>                            │
├──────────────────────────────────────────────────────────────┤
│  Orchestrator                                                 │
│  load pin → scan → reconcile → build ctx → core              │
│  → save (reset only)                                          │
├─────────────────────────┬────────────────────────────────────┤
│  PinStore               │  Scanner                           │
│  read/write drift.pin   │  follow main.pin.xml imports →     │
│  (XML codec)            │  discover specs (module-qualified)  │
│                         │  walk dir tree → discover markers   │
│                         │  hash content → produce ScanResult  │
├─────────────────────────┴────────────────────────────────────┤
│  Core (core.go)  ✓ done                                      │
│  pure, stateless                                            │
│  EvaluateState(ctx) → EvaluatedState                        │
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
- `<ref>` elements are not parsed by the scanner in this phase. They are part of spec content and get hashed as-is. Ref-based drift is a future steel cable.

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
| Spec struct | `Spec.ID` = full qualified string. `Spec.Module` = module name. |
| drift.pin storage | Module not stored separately — derived from qualified ID. |
| Refs | `<ref spec="module.specid">text</ref>` — inline in prose, hashed as content. Not parsed for drift yet. |
| Markers | `#F <shortcode>` comment lines in code files. Shortcodes are bare (not module-qualified). |
| Marker-to-spec links | `drift link <marker> <module.spec>` — space-separated. |
| drift.ignore | Applies to marker discovery only (code files). Spec discovery is via imports. |
| Marker hashing | Next 10 lines from marker line |
| Missing from disk | Error if spec/marker in drift.pin but not found by scanner |

## File structure

```
cmd/drift/main.go        # entry point (imports cli)
core/
  core.go                # ✓ done - pure algorithm (no changes needed)
  core_test.go           # ✓ done - no changes (uses bare string IDs)
  core.pin.xml           # migrate - <specs> → <module name="core">
scanner/
  scanner.go             # rewrite - import graph instead of dir walk for specs
  scanner_test.go        # rewrite - module format, import graph, cycle detection
  scanner.pin.xml        # migrate - <specs> → <module name="scanner">
pinstore/
  pin_file.go            # update - Spec gains Module field, specXML unchanged
  pin_file_test.go       # update - qualified spec IDs
  pinstore.pin.xml       # migrate - <specs> → <module name="pinstore">
orchestrator/
  orchestrator.go        # minor - find main.pin.xml, pass to scanner
  orchestrator_test.go   # update - qualified spec IDs
  orchestrator.pin.xml   # migrate - <specs> → <module name="orch">
cli/
  cli.go                 # update - new link/reset syntax
  cli_test.go            # update - new syntax + module-format spec files
  cli.pin.xml            # migrate - <specs> → <module name="cli">
internal/testutil/       # shared test helpers (NewSpec, AssertNoError, etc.)
  testutil.go
  fixtures.go            # markerLine() — excluded from drift scan
main.pin.xml             # new - entry point importing all project modules
drift.pin                # rebuilt after migration
drift.ignore             # excludes internal/testutil/fixtures.go, examples/
examples/                # ✓ done - reference examples for all 4 structures
```

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

### Spec discovery (new)

1. Look for `main.pin.xml` in the working directory. Error if not found.
2. Parse root element: `<main>` or `<module name="...">`.
3. If `<main>` with direct `<spec>` children: implicit module name `"main"`.
4. Follow `<import path="...">` relative to the importing file.
5. Track visiting stack (by absolute path) for cycle detection.
6. Track loaded files (by absolute path) for dedup.
7. Track module names (by string) for duplicate detection.
8. Each spec: `Module` = module name, `ID` = `Module + "." + localID`.
9. Hash: SHA1 of trimmed inner content (including any `<ref>` elements).

### Marker discovery (unchanged)

1. Walk working directory tree for code files (`.go`, `.py`, `.js`, etc.).
2. Apply `drift.ignore` patterns.
3. Find lines matching `#F <shortcode>` pattern.
4. SHA1 hash the next 10 lines from the marker line.
5. Record: shortcode (bare ID), filepath, line number, hash.

## Orchestrator reconciliation

On `drift todo` / `drift reset`:
1. Load `PinState` from `drift.pin`
2. Get `ScanResult` from scanner (specs via import graph, markers via dir walk)
3. **Reconcile specs**: for each in ScanResult:
   - In PinState → keep baseline hash, update filepath/line if changed
   - NOT in PinState → new, baseline = current hash (no drift)
   - In PinState but NOT in ScanResult → error: "spec X in drift.pin but not found on disk"
4. Same for markers
5. Build `Scan` from ScanResult hash maps
6. Build `CoreAlgorithmContext` with reconciled specs/markers + links/resolution from PinState
7. Run core

## CLI commands

| Command | What it does |
|---|---|
| `drift init` | Creates empty drift.pin |
| `drift todo` | Scans imports + dir → reconciles → runs core → outputs todos |
| `drift reset <marker> <module.spec>` | Scans → reconciles → runs core reset → saves |
| `drift link <marker> <module.spec>` | Validates + adds link to drift.pin |

## Spec migration

Project's own spec files migrate from `<specs>` to `<module>` format:

| File | Module name | Old ID → New ID |
|---|---|---|
| `core.pin.xml` | `core` | `core_validate` → `core.validate` |
| `cli.pin.xml` | `cli` | `cli_dispatch` → `cli.dispatch` |
| `scanner.pin.xml` | `scanner` | `scanner_spec_discovery` → `scanner.spec_discovery` |
| `orchestrator.pin.xml` | `orch` | `orch_init` → `orch.init` |
| `pinstore.pin.xml` | `pinstore` | `pin_load` → `pinstore.load` |

`main.pin.xml` (new): entry point importing all 5 modules.

`#F` markers in `.go` files: no change (shortcodes stay bare).
`drift.pin`: rebuilt via `drift init` + re-link all markers with qualified spec IDs.

## Steel cable iterations

### Steel cables 1-6: ✓ DONE

Core algorithm, pin store, scanner (dir-walk), orchestrator, CLI, many-to-many topologies. All 190+ tests pass, vet clean.

### Steel cable 7: Module/import system ← CURRENT

**Test plan (red first, then green):**

`scanner_test.go` — rewrite:
- `main.pin.xml` with direct specs (implicit "main" module) → specs have `main.specid`
- `main.pin.xml` importing one module → specs from module with `module.specid`
- `main.pin.xml` importing multiple modules → all specs discovered
- Transitive imports: main → A → B → all loaded, specs from all three
- Explicit visibility: module refs spec from unimported module → error
- Diamond imports: A and B both import C, main imports A and B → C loaded once
- Duplicate module names → error
- Cycle: main → A → B → A → error with trace
- Missing `main.pin.xml` → error
- Import path not found → error
- Spec missing `id` attribute → error
- Duplicate spec IDs within same module → error
- Same spec ID in different modules → OK (per-file scoping)
- Marker discovery tests: unchanged (dir walk)
- `drift.ignore` applies to markers only (not specs)

`scanner_test.go` — marker tests stay:
- 1 code file with 1 marker → correct shortcode/filepath/line/hash
- Hash is SHA1 (deterministic)
- 10-line marker hashing window
- `drift.ignore` excludes files from marker scan

`cli_test.go` — update:
- All spec files use `<module>`/`<main>` format
- `drift link m1 core.validate` (new syntax)
- `drift reset m1 core.validate` (new syntax)
- E2E: init → create main.pin.xml with module → create code with marker → todo → link → drift → reset

`orchestrator_test.go` — update:
- Qualified spec IDs in all test fixtures
- Reconciliation with module-qualified IDs

`pin_file_test.go` — update:
- Qualified spec IDs in round-trip tests

**Execution order:**
1. Red: `scanner_test.go` (import graph tests)
2. Red: `cli_test.go` (new syntax)
3. Red: `orchestrator_test.go` (qualified IDs)
4. Red: `pin_file_test.go` (qualified IDs)
5. Green: `scanner.go` (import graph parser + cycle detection)
6. Green: `cli.go` (new link/reset syntax)
7. Green: `orchestrator.go` (find main.pin.xml)
8. Green: `pin_file.go` (Spec.Module field)
9. Migrate project specs (5 .pin.xml files + new main.pin.xml)
10. Rebuild drift.pin (drift init + re-link)
11. Run all tests → verify green
12. `go vet` clean
13. `drift todo` passes

### Steel cable 8: Ref-based drift (future)

Parse `<ref>` elements in spec content. Implement dual-hash model:
- Self hash: hash of spec content excluding resolved refs
- Composite hash: hash including resolved ref content
- Markers link to composite hash
- Drift output distinguishes "you changed this" vs "a dependency changed"

### Steel cable 9: AST (future)

Replace flat prose specs with structured AST nodes. Each node hashable independently. Markers link to specific AST nodes, not whole specs. See design discussion notes.
