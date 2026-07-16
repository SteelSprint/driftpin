# Plan — Diff Verification (Phase 7)

> Source: Observation 0007 — Phase 6 range model eval (6 runs).
> Top convergent finding (4/6 runs): `drift todo` reports *that* something changed but never *what*. The verify step — the one step the tool explicitly asks the agent to perform before resetting — has zero tooling support.

## Goal

Close the detect→**verify**→resolve loop. When `drift todo` reports drift, the agent runs `drift diff <marker> <spec>` and sees a unified diff of both the spec and the marker content against their baselines — instead of re-reading whole regions and guessing against a SHA1.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  CLI (cli.go)                                                 │
│  drift todo    → summary + one-line hint per item             │
│  drift diff <marker|spec>         (NEW)                       │
│  drift diff <marker> <spec>       (NEW, edge-scoped)          │
│  drift show / link / unlink / reset / list / init / skill     │
├──────────────────────────────────────────────────────────────┤
│  Orchestrator                                                 │
│  Link  → pin.Save + BaselineStore.Write (both sides)          │
│  Reset → pin.Save + BaselineStore.Write (new baseline hashes) │
│  Diff  → read-only: load state, read baselines by hash,       │
│          compute current content, return both sides           │
├───────────────────────────────┬──────────────────────────────┤
│  PinStore                     │  BaselineStore (NEW)          │
│  .driftpin/state.xml          │  .driftpin/baselines/<hash>    │
│  (lean hash/link index)       │  (raw content, content-       │
│                               │   addressed, self-verifying)  │
├───────────────────────────────┴──────────────────────────────┤
│  internal/diff (NEW)                                          │
│  UnifiedDiff(old, new string) string                          │
│  LCS-based, ~120 lines, zero external deps                   │
└──────────────────────────────────────────────────────────────┘
```

## Directory layout

```
<project root>/
  .driftpin/
    state.xml              # formerly drift.pin — specs/markers/links/resolutions
    baselines/
      <sha1hex>            # raw baseline content; sha1(file)==filename
      ...                  # dedup'd (identical content → one file)
  drift.ignore             # stays at root (user-facing)
  main.pin.xml, *.pin.xml  # stay at root (entry point + spec files)
```

**Naming rationale:**
- `.driftpin/state.xml` — descriptive; the directory says "drift," `.xml` signals format.
- `.driftpin/baselines/<hash>` — flat, content-addressed. The filename IS the integrity check. Dedup is automatic. No escaping/pathing issues (hex is filesystem-safe).

**Content-addressing falls out for free:** `state.xml` already stores `hash` per spec/marker. That hash IS the baseline content's address (`sha1(baseline_content) == hash`). No new field needed — the hash does double duty as drift-detection key AND content address.

## Locked decisions

| Decision | Choice | Rationale |
|---|---|---|
| Baseline storage location | `.driftpin/baselines/<hash>` (flat, content-addressed) | Self-verifying; dedup'd; drift.pin stays lean; no pathing/escaping issues |
| Diff algorithm | In-house LCS (~120 lines, new `internal/diff` package) | Preserves zero-external-deps invariant; inputs are tiny so O(n·m) is fine |
| `drift todo` integration | Summary + one-line hint per item | Keeps `todo` scannable for multi-edge drift; no context blowup |
| `snapshot` / `gc` commands | **Skip** | Baselines written on `link`/`reset`; orphaned files harmless (tiny, dedup'd). Add `gc` later if needed |
| `drift.pin` → `.driftpin/state.xml` | Clean break, migrate manually | Pre-release, no external users. No backward-compat shim. |

## Baseline lifecycle

| Operation | What happens to baselines |
|---|---|
| `drift link <m> <s>` | Write `.driftpin/baselines/<spec.hash>` = spec content, `<marker.hash>` = marker content (skip if file exists — dedup) |
| `drift reset <m> <s>` | Baseline collapses → new hashes → write new baseline files for new hashes. Old files become unreferenced (harmless). |
| `drift reset <id>` (orphan) | Entity removed from state.xml. Its baseline file becomes unreferenced (harmless). |
| `drift diff <m> <s>` | Read state.xml → baseline hashes → read `.driftpin/baselines/<hash>` → baseline content. Compute current content (reuse `readSpecContent`/`readMarkerContent`). LCS diff. Read-only. |

## Implementation steps

### Step 1: `internal/diff` package (new)

~120 lines. Zero external deps.

- `func UnifiedDiff(old, new string) string` — LCS-based, emits unified-diff hunk format (`@@ -a,b +c,d @@`, ` `/`-`/`+` prefixes).
- Standard LCS DP table (O(n·m), fine for tiny inputs — specs: 1-5 lines, markers: 1-50 lines).
- Edge cases:
  - Empty old → all `+` lines (handles deletion drift: shows what was removed)
  - Empty new → all `-` lines
  - Identical → empty string (caller prints "No changes — in sync.")
  - Both empty → empty string
- No context-line count config (keep it simple; default 3 lines context).

**Tests:**
- Identity (no changes → empty)
- Insert (new lines added)
- Delete (lines removed)
- Modify (lines changed)
- Replace (block replaced)
- Multi-hunk (disjoint changes)
- Single-line
- Both-empty
- Deletion-drift case (old has content, new is empty)

### Step 2: `pinstore` restructure

- `NewFilePinStore(dir)` → targets `.driftpin/state.xml`.
- `Save` creates `.driftpin/` if missing.
- `drift init` → creates `.driftpin/` + `.driftpin/state.xml` + `.driftpin/baselines/`.
- XML struct unchanged (just the path moves).
- Update existing pinstore tests to use new path.

### Step 3: `pinstore/baselines.go` (new)

```go
type BaselineStore struct { dir string }  // .driftpin/baselines/

func (b *BaselineStore) Write(hash, content string) error
  // Writes .driftpin/baselines/<hash> if absent (dedup).
  // Asserts sha1(content) == hash defensively (mismatch → error).

func (b *BaselineStore) Read(hash string) (string, bool)
  // Reads .driftpin/baselines/<hash>. Returns false if file missing.

func (b *BaselineStore) Delete(hash string) error
  // Removes a single baseline file. (Used by orphan cleanup if needed later.)
```

**Tests:**
- Write + read round-trip
- Dedup (write same hash twice → one file, no error)
- Missing file → Read returns `false`
- Integrity mismatch (content hash != filename → Write errors)
- Delete removes file

### Step 4: Orchestrator wiring

- `Link`: after `pin.Save`, compute current spec+marker content (reuse scanner's content extraction), `BaselineStore.Write(hash, content)` for both.
- `Reset`: after `pin.Save` (baseline collapse), write new baseline files for new hashes.
- New `Diff(markerID, specID string) (DiffResult, error)`:
  - Loads state, scans current content (for hashes + content).
  - Reads baselines via `BaselineStore.Read(baselineHash)`.
  - Returns struct with spec baseline/current + marker baseline/current (or "missing" flags).
  - **Pure/read-only** — does not save (safe to call anytime).

```go
type DiffResult struct {
    Spec   DiffSide
    Marker DiffSide
}

type DiffSide struct {
    ID         string
    Filepath   string
    Lines      string  // "start-end" for markers, "" for specs
    BaselineHash string
    CurrentHash string
    Baseline   string  // content; empty if no snapshot
    Current    string  // content
    HasBaseline bool    // false if snapshot missing
    Deleted    bool    // true if entity deleted (current empty)
}
```

**Tests:**
- `Link` writes baseline files for both spec and marker
- `Reset` updates baseline files (new hashes)
- `Diff` returns correct baseline + current content
- `Diff` with missing baseline (HasBaseline=false)
- `Diff` with deleted spec (Deleted=true, Current empty)
- `Diff` with deleted marker

### Step 5: `drift diff` command (cli)

- **`drift diff <marker|spec>`** (one-arg): dot detection reuses `show`'s logic. Auto-expands: shows that entity's diff + all linked counterparts' diffs.
- **`drift diff <marker> <spec>`** (two-arg): scoped to one edge — shows spec diff then marker diff, separated by `---`.

**Output format:**
```
Spec: main.div (main.pin.xml)
Baseline: c8a4827d...   Current: bb82d02a...

--- baseline
+++ current
@@ -1 +1 @@
-div divides a by b and returns an error for division by zero
+div divides a by b and returns an error for division by zero or negative zero

---

Marker: div_func (main.go:22-32)
Baseline: 1d10564d...   Current: bb82d02a...

--- baseline
+++ current
@@ -3,3 +3,5 @@
 	if b == 0 {
 		return 0, fmt.Errorf("division by zero")
 	}
+	if b == 0 && math.Signbit(b) {
+		return 0, fmt.Errorf("division by negative zero")
+	}
```

**Cases:**
- In-sync → "No changes — in sync." (exit 0)
- No baseline snapshot → "No baseline snapshot for <id> (hash <hash>)." (exit 0, informational)
- Deleted entity → current side empty, diff shows all-removed lines
- Both sides changed → two-arg form shows both diffs, clearly separated

**Tests:**
- One-arg (marker): shows spec diff + marker diff
- One-arg (spec): shows spec diff + linked marker diffs
- Two-arg: edge-scoped
- In-sync: "No changes — in sync." (exit 0)
- No baseline snapshot: informational message (exit 0)
- Deletion drift: all-removed diff
- Format assertions (hunk headers, `+`/`-` prefixes, `---` separator)

### Step 6: `drift todo` hint line

After each todo item, append:
```
  → Run 'drift diff <marker> <spec>' to see what changed.
```

Keeps `todo` scannable — one extra line per item, not full diffs.

**Test:** hint line present in todo output when drift exists; absent when clean.

### Step 7: Manual migration (filament project + 3 eval fixtures)

For each project:
1. Create `.driftpin/` directory
2. Move `drift.pin` → `.driftpin/state.xml`
3. Backfill `.driftpin/baselines/<hash>` for every spec/marker from current content (all in-sync, so baseline == current content)
4. Delete old `drift.pin` at root

**Projects to migrate:**
- `/workspaces/filament/` (the filament project itself)
- `/workspaces/filament/eval/prompts/bad-link/`
- `/workspaces/filament/eval/prompts/code-refactor/`
- `/workspaces/filament/eval/prompts/drift-detection/`

### Step 8: Specs + docs

- New spec `cli.diff_command` (in cli.pin.xml)
- New markers: `cdiff` (diff command impl), `chint` (todo hint line)
- Update specs: `cli.format_todo` (hint line), `cli.dispatch`, `cli.help`, `cli.skill`
- `drift skill`: add `diff` to CLI table, document `.driftpin/` directory layout, add "Diffs" section
- `drift help`: add `diff` line
- Rebuild `.driftpin/state.xml` + baselines for filament project after all changes

### Step 9: Tests + vet + gofmt

Run full suite: `go test ./...`, `go vet ./...`, `gofmt -l .`

## Edge cases handled

| Case | Behavior |
|---|---|
| In-sync (no drift) | "No changes — in sync." (exit 0) |
| No baseline snapshot (pre-migration) | "No baseline snapshot for <id> (hash <hash>)." (exit 0, informational) |
| Spec deleted | Current side empty → diff shows all baseline lines as removed |
| Marker deleted | Same as spec deleted |
| Both sides changed | Two-arg form shows both diffs, separated by `---` |
| Multi-edge (1 spec → 3 markers) | One-arg `diff <spec>` shows spec diff once + each linked marker's diff |
| Identical content shared by entities | Dedup'd automatically (one baseline file per unique hash) |
| Baseline integrity mismatch | `Write` errors if `sha1(content) != hash` |

## What this does NOT include (deferred)

- `drift snapshot` command (one-shot backfill) — baselines written naturally on `link`/`reset`
- `drift gc` command (delete unreferenced baseline files) — orphaned files harmless, add later
- `--json` output — separate concern (obs 0007 recommendation #3)
- Inline diffs in `drift todo` — kept as hint line (could add `--diff` flag later)
- Backward-compat read path for `drift.pin` at root — clean break

## Effort estimate

| Component | LOC (impl) | LOC (tests) |
|---|---|---|
| `internal/diff` (LCS) | ~120 | ~100 |
| `pinstore` restructure | ~20 (path changes) | ~30 (update existing) |
| `pinstore/baselines.go` | ~80 | ~60 |
| orchestrator wiring | ~100 | ~80 |
| cli `diff` command | ~120 | ~80 |
| cli `todo` hint | ~10 | ~15 |
| migration + spec/doc updates | — | — |
| **Total** | **~450** | **~365** |

## Verification

After implementation:
1. All tests pass: `go test ./...`
2. Vet clean: `go vet ./...`
3. Gofmt clean: `gofmt -l .`
4. `drift diff` works on the filament project itself (drift a marker, see the diff)
5. Re-run eval battery → observation 0008 to verify the verify step is now tooled
