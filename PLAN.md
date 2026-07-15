# Plan

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  CLI (cli.go)                                        │
│  drift init | drift todo | drift reset <m>:<s>      │
│  drift link <m>:<s>                                  │
├──────────────────────────────────────────────────────┤
│  Orchestrator                                        │
│  load pin → scan → reconcile → build ctx → core     │
│  → save (reset only)                                 │
├────────────────────────┬─────────────────────────────┤
│  PinStore              │  Scanner                    │
│  read/write drift.pin  │  walk fs, discover specs +  │
│  (XML codec)           │  markers, hash content,     │
│  ✓ done                │  produce ScanResult         │
├────────────────────────┴─────────────────────────────┤
│  Core (core.go)  ✓ done                              │
│  pure, stateless                                     │
│  EvaluateState(ctx) → EvaluatedState                 │
└──────────────────────────────────────────────────────┘
```

## Decisions

| Decision | Choice |
|---|---|
| drift.pin format | XML (stdlib `encoding/xml`, zero deps) |
| Hash function | SHA1 hex-encoded |
| Missing drift.pin | `drift init` required first |
| CLI output | Match DOCUMENTATION.md exactly |
| Test doubles | Hand-written fakes |
| File location | Project root |
| Testing | Red/green, exhaustive arity, clamped validations |
| Build approach | Walking skeleton / steel cable, end-to-end per iteration |
| Spec files | `*.pin.xml` with `<spec id="...">` elements |
| Markers | `#F <shortcode>` comment lines in code files |
| Marker-to-spec links | Separate shortcodes, links declared in drift.pin via CLI |
| Marker hashing | Next 10 lines from marker line (configurable in future) |
| Missing from disk | Error if spec/marker in drift.pin but not found by scanner |

## File structure

```
core.go              # done - pure algorithm
core_test.go         # done - exhaustive tests
pin_file.go          # done - PinState, PinStore, XML read/write
pin_file_test.go     # done - exhaustive arity, round-trip, error cases
scanner.go           # Scanner interface, FileScanner (discovery + hashing)
scanner_test.go      # exhaustive arity, clamped validations
orchestrator.go      # Orchestrator: Init(), Todo(), Reset(), Link()
orchestrator_test.go # exhaustive arity with fakes, reconciliation tests
cli.go               # CLI dispatch + output formatting
cli_test.go          # E2E steel cable tests
cmd/drift/main.go    # Entry point
Makefile             # make build
```

## XML format for drift.pin

```xml
<drift>
  <specs>
    <spec id="validate_input" hash="S98YH3T2T32..." filepath="main.pin.xml" line="37"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="JHIO34YU..." filepath="src/main.go" line="15"/>
  </markers>
  <links>
    <link specId="validate_input" markerId="4hy7fh3h"/>
  </links>
  <resolutions>
    <resolution specId="validate_input" markerId="4hy7fh3h" currentSpecHash="..." currentMarkerHash="..."/>
  </resolutions>
</drift>
```

## Scanner interface

```go
type ScanResult struct {
    Specs   []Spec   // ID, Filepath, LineNumber, Hash (current)
    Markers []Marker // ID, Filepath, LineNumber, Hash (current)
}

type Scanner interface {
    Scan() (ScanResult, error)
}
```

The orchestrator extracts `Scan` (hash maps) from `ScanResult` internally. `Scan` stays as-is for the core algorithm.

### Spec discovery

- Walk the project directory for `*.pin.xml` files
- Parse XML, extract `<spec id="...">` elements
- SHA1 hash each spec element's inner content
- Record: ID, filepath, line number, hash

### Marker discovery

- Walk the project directory for code files (`.go`, `.py`, `.js`, `.ts`, etc.)
- Find lines matching `#F <shortcode>` pattern
- SHA1 hash the next 10 lines from the marker line
- Record: shortcode (ID), filepath, line number, hash
- (Future: configurable line count per marker declaration)

## Orchestrator reconciliation

On `drift todo` / `drift reset`:
1. Load `PinState` from `drift.pin`
2. Get `ScanResult` from scanner
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
| `drift todo` | Scans fs → reconciles → runs core → outputs todos |
| `drift reset <marker>:<spec>` | Scans fs → reconciles → runs core reset → saves |
| `drift link <marker>:<spec>` | Validates + adds link to drift.pin |

## Steel cable iterations

### Steel cable 1: `drift init` → `drift todo` → "No changes detected." ✓ DONE

PinStore, Scanner (empty), Orchestrator, CLI dispatch, output formatting.

### Steel cable 2: `drift reset` on empty → error ✓ DONE (covered by steel cable 1 tests)

### Steel cables 3-4: Scanner discovery + link command + drift detection

**Test plan (red first, then green):**

`scanner_test.go` — exhaustive arity:
- Empty project → empty ScanResult
- 1 spec file with 1 spec element → correct ID/filepath/line/hash
- 1 spec file with many spec elements
- Many spec files
- 1 code file with 1 `#F` marker → correct shortcode/filepath/line/hash
- 1 code file with many markers
- Many code files
- Mixed: specs + markers across multiple files
- Spec element missing `id` attribute → error
- Duplicate spec IDs across files → error
- Duplicate marker shortcodes across files → error
- Hash is SHA1 (deterministic across runs)
- 10-line marker hashing (exactly 10 lines, fewer if EOF)

`orchestrator_test.go` — new reconciliation tests:
- Empty pin + empty scan → no drift
- Empty pin + discovered specs/markers → specs/markers added with baseline=current, no drift
- Pin with specs + scan with same hashes → no drift
- Pin with specs + scan with changed hash → drift detected
- Pin with spec not in scan → error
- Pin with marker not in scan → error
- New spec in scan not in pin → added, no drift
- Link added via orchestrator → link in saved PinState

`cli_test.go` — E2E:
- init → create spec file → create code with marker → todo (no links → no drift)
- → link → todo (linked, no drift → "No changes detected.")
- → modify code → todo (1 drift todo)
- → reset → todo (no drift)
- → link nonexistent marker/spec → error
- → link duplicate → error

**Execution order:**
1. Red: `scanner_test.go` (scanner arity tests)
2. Red: Update `orchestrator_test.go` (reconciliation + link tests)
3. Red: Update `cli_test.go` (E2E link + drift tests)
4. Green: Update `scanner.go` (implement discovery + hashing)
5. Green: Update `orchestrator.go` (reconciliation + `Link()`)
6. Green: Update `cli.go` (link command)
7. Green: Update fakes in test files
8. Run all tests → verify green
9. Manual testing instructions

### Steel cable 5: `drift reset m1:s1` → edge resolved ✓ (covered by orchestrator tests)

### Steel cable 6+: Many-to-many topologies

Partial resolution (1×2, 2×1, 2×2, 3×3), progressive collapse, matrix state tracking.
