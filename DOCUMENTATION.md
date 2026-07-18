# Documentation

## Mental model

When specs and their implementations change, `drift` informs your LLM. This gives your LLM a chance to fix any drifts as they happen. `drift` is packaged as a CLI that is intended mainly for LLMs to use.

## Self-discovery

The binary is self-describing:

- `drift help` — command reference with marker syntax, spec file format, and examples
- `drift skill` — comprehensive guide covering the full workflow, module/import system, marker hashing, refs, and CLI reference. Designed for LLM agents to pipe into their context.

## How it works

`drift` asks you to save your specs in `*.drift.xml` files. Each file contains `<spec id="...">` elements that describe individual spec terms. `drift` then scans your code files for `D! id=<shortcode>` markers — short unique IDs placed in comments above the code that implements each spec term. Specs and markers form a many-to-many graph — a single spec term can be enforced by several markers, and a single marker can refer to several spec terms. Each connection between two nodes is called an **edge**. When a change occurs on either side of an edge, `drift todo` surfaces one todo item per affected edge, with filepaths and line numbers where your LLM should check for drifts in specification.

There are two kinds of edges, unified in storage:

- **marker → spec** (the "link" edge): created by `drift link <marker> <module.spec>`. The marker stores the edge to the spec it implements.
- **spec → spec** (the "ref" edge): auto-parsed from `<ref spec="module.localid">label</ref>` tags inside spec content. The source spec stores the edge to the cited spec.

Both kinds share a single `<edges>` section in `state.xml`, run through one reconciliation pass, and propagate drift symmetrically (rhizomatic — see [Rhizomatic propagation](#rhizomatic-propagation) below). The stored direction is preserved for cycle detection (the directed spec-spec graph must be acyclic); drift propagation is undirected.

`drift` hashes spec content (canonicalized — refs stripped, label text preserved) and marker content (lines between `range-start`/`range-end` with other marker declarations blanked) using SHA1. Those hashes are stored inside `.drift/state.xml`, which should be committed to git. This is an XML file (version=3) that contains the hashes of specs and markers, the edges between them, and a temporary resolution state area for partial todo-list resolutions. The algorithm manages this file itself — the user should refrain from touching `.drift/state.xml` manually.

### Spec files

Specs are defined in `*.drift.xml` files. The entry point is `main.drift.xml` in the project root.

**main.drift.xml** (entry point — can be pure manifest or have direct specs):
```xml
<main>
  <import path="./core/core.drift.xml"/>
  <spec id="bootstrap">Initialize the project and load all modules</spec>
</main>
```

**Module files** (e.g. `core/core.drift.xml`):
```xml
<module name="core">
  <spec id="validate_input">input must be validated before processing. See <ref spec="main.bootstrap">bootstrap</ref>.</spec>
  <spec id="parse_input">Parse input tokens into structured data</spec>
</module>
```

Specs are **direct children** of the root element. Do not wrap them in a `<specs>` element — the scanner will reject this. Spec IDs are module-qualified: `<module>.<specId>`. Specs in `main.drift.xml` use the `main.` prefix (e.g. `main.bootstrap`).

The scanner walks the import graph starting from `main.drift.xml`. Imports are relative to the importing file. Diamond imports are deduplicated by absolute path. Duplicate module names cause an error. Cycles are detected and reported with a trace.

### Refs (spec-spec edges)

A `<ref spec="module.localid">label text</ref>` (or self-closing `<ref spec="module.localid" />`) inside a `<spec>` element declares a spec-spec edge: "this spec cites that spec." The scanner parses these and stores them as baseline edges alongside link edges.

Two key properties:

- **Canonical hashing**: refs are stripped from spec content before hashing. Only the label text is preserved. So renaming a referenced spec ID (and updating the `<ref spec="…">` to match) does NOT invalidate the referrer's hash — ID renames stay localized instead of cascading hash drift across the spec graph.
- **Rhizomatic propagation**: drift propagation treats the ref graph as undirected (see [Rhizomatic propagation](#rhizomatic-propagation) below). The stored direction records who-cited-whom (used for cycle detection); propagation is symmetric.

**No directed cycles among spec-spec edges.** `A → B → A` is rejected by validation. The scanner reports all cycles in one pass with the offending path. (Undirected cycles — paths that return to the start via a different edge — are fine and expected in any non-trivial spec graph.)

### Markers

Markers are `D! id=<shortcode> range-start` and `D! id=<shortcode> range-end` comment lines in any text file — drift is language-agnostic and scans all text files (binary files are skipped by extension blocklist and null-byte detection):

```go
// D! id=<your_id> range-start
func handleRequest() {
    validateInput()
}
// D! id=<your_id> range-end
```

The scanner finds these lines, records the shortcode, filepath, and line number, and SHA1-hashes the lines between range-start and range-end (exclusive of both marker lines, with nested marker declarations blanked). The marker pattern is a regex: `D!\s+id=([A-Za-z][A-Za-z0-9_]*)\s+(range-start|range-end)`. It can appear in any comment style (`//`, `#`, `--`, etc.).

### Link edges

Markers and specs have separate IDs — a marker's shortcode does not match a spec's ID. Link edges between them are declared in `.drift/state.xml` via the CLI:

```bash
$ drift link 4hy7fh3h core.validate_input
Linked marker "4hy7fh3h" to spec "core.validate_input"
```

This validates that both the marker and spec exist, then persists the edge. Edges can be many-to-many: one marker can link to multiple specs, and one spec can link to multiple markers.

## CLI commands

| Command | Description |
|---|---|
| `drift init` | Creates `.drift/state.xml` and a starter `main.drift.xml` template. Required before other commands. |
| `drift todo` | Scans the filesystem, reconciles with `.drift/state.xml`, and surfaces any drift as a todo list. Does not modify `.drift/state.xml`. |
| `drift link <marker> <module.spec>` | Declares a marker → spec edge. Validates both exist and the edge isn't a duplicate. Saves specs, markers, and the new edge to `.drift/state.xml`. |
| `drift unlink <marker> <module.spec>` | Removes a marker → spec edge. Also clears any resolution state for that pair. |
| `drift list` | Shows all specs, markers, edges, and sync state. Read-only. |
| `drift reset <marker> <module.spec>` | Marks a marker-spec edge as resolved (link-style reset; first arg has no dot, second has a dot). Saves updated state to `.drift/state.xml`. If all edges for a node are resolved, baselines collapse automatically. |
| `drift reset <spec> <spec>` | Marks a spec-spec edge as resolved (ref-style reset; both args have dots). First arg is the reviewed spec, second is the source spec. Also handles edge-add (baseline the new edge) and edge-remove (drop the baseline edge). |
| `drift reset <id>` | Removes an orphaned (deleted, no edges) spec/marker from `.drift/state.xml`. Dot in ID → spec; no dot → marker. |
| `drift help` | Prints command reference with marker syntax, spec file format, and examples. |
| `drift skill` | Prints a comprehensive guide for LLM agents covering the full workflow, module/import system, refs, and drift detection model. |

The reset dispatch is dots-based: marker shortcodes contain no dots; spec IDs contain exactly one. The CLI reads the dots and routes accordingly.

## Reconciliation

When `drift todo` or `drift reset` runs, the orchestrator:

1. Loads `.drift/state.xml` (baseline hashes, edges, resolution state)
2. Scans the filesystem (current specs, markers, spec-spec edges from `<ref>` tags, and their hashes)
3. **Reconciles** — for each discovered spec/marker:
   - If already in `.drift/state.xml` → keeps the baseline hash from state, updates filepath/line if changed
   - If new (not in pin) → baseline = current hash (no drift on first discovery)
   - If in pin but not found on disk → kept as deleted (sentinel empty-string scan hash)
4. Merges scanned spec-spec edges into baseline (link edges are user-curated and never appear in the scan)
5. Builds the scan and runs the core algorithm

This means the first `drift todo` after adding spec files or code markers discovers them and sets their baselines. On subsequent runs, changes are detected by comparing current hashes against these baselines.

## Example

Let's say you have a marker `4hy7fh3h` and a spec file that has `validate_input` in the `core` module.

First, initialize and discover:

```bash
$ drift init
Initialized .drift/ and main.drift.xml
Edit main.drift.xml to add your specs, then place D! id=<markerid> markers in your code.
Run `drift skill` for a comprehensive guide.

$ drift todo
No changes detected. 1 specs, 1 markers, 0 edges in sync.
```

The scanner discovers the spec and marker. Since they're new, baselines are set to current hashes — no drift. Link the marker to the spec:

```bash
$ drift link 4hy7fh3h core.validate_input
Linked marker "4hy7fh3h" to spec "core.validate_input"
```

Now let's say you modify some code:

```bash
$ drift todo

1 marker has unchecked changes.

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "core.validate_input" in "/workspaces/my-project/core/core.drift.xml:1". The From endpoint has changed but not the To endpoint. Please check whether the changed side still aligns with the other. Once you are satisfied, run `drift reset 4hy7fh3h core.validate_input` to mark this todo item as complete.
```

At this point `.drift/state.xml` is still unchanged — `drift todo` doesn't modify the file.

Then you mark the edge as resolved:

```bash
$ drift reset 4hy7fh3h core.validate_input
```

Since the marker has no more unchecked specs, and the spec has no more unchecked markers, the baselines collapse — `.drift/state.xml` is updated with the new hashes. The next `drift todo` will report "No changes detected: ..."

## Drift detection output

When there is no drift, `drift todo` distinguishes between empty and synced:

- **Empty** (no specs or markers registered): `Nothing to check: no specs or markers registered.`
- **Synced**: `No changes detected. N specs, M markers, K edges in sync.`

When there is drift, it prints summary counts bucketed by kind, then lists each todo item with:
- The edge (From and To endpoints)
- File locations with line numbers
- The drift kind and reason
- The `drift reset` command to run once resolved

### Drift kinds

| Kind | When | Example message |
|---|---|---|
| TodoEdgeDrift (direct) | A baseline edge (link or ref) where one or both endpoint hashes changed. | "Edge between marker X and spec term Y. The From endpoint has changed..." |
| TodoEdgeDrift (chain) | A spec transitively reachable through the ref graph from a hash-drifted spec. | "Spec X drifted because transitively-connected spec Y changed..." |
| TodoCascade | A marker linked to a chain-drifted spec. Derived — resolves when upstream chain drift resolves. | "Marker X drifted because transitively-connected spec Y changed..." |
| TodoEdgeAdded | A spec-spec edge appears in scan but not baseline (a new `<ref>` was declared). | "New edge declared: X → Y..." |
| TodoEdgeRemoved | A spec-spec edge was in baseline but not scan (a `<ref>` was deleted). | "Edge removed: X no longer points to Y..." |
| TodoBrokenEdge | A scan edge whose To endpoint does not exist (typo in a `<ref>` target). | "Spec X refs Y, but no node with that ID exists..." |

<a name="rhizomatic-propagation"></a>

## Rhizomatic propagation

The unified edge model treats drift propagation as **undirected** along the ref graph. When spec S changes:

1. Every baseline edge touching S emits TodoEdgeDrift (direct drift on the edge).
2. The core walks the undirected ref graph from S (BFS), visiting every transitively-connected spec N:
   - If no edge resolution covers the (N, S) pair → emit TodoEdgeDrift (chain drift) for N with SourceSpecID = S.
   - For each marker M linked to N → emit TodoCascade(M, source=S).
3. Resolving the upstream chain drift (`drift reset <N> <S>`) clears cascade todos for markers linked to N.

**Source-of-truth asymmetry**: marker hash-drift does NOT propagate (markers are leaf implementations; the spec might still be correct — only the code is suspect). Spec hash-drift DOES propagate rhizomatically (every transitively-connected spec's alignment is in question).

## Many-to-many relationships

Specs and markers form a many-to-many graph via edges. A single spec term can be enforced by several markers (e.g. the same rule applied in multiple places), and a single marker can refer to several spec terms (e.g. one block of code that satisfies multiple requirements). Because todos are edge-based, the number of todo items is the product of changed specs and their related markers.

### One spec term, many markers

A spec term like `core.auth_token_expiry` is often enforced in more than one place — say a middleware layer and a login handler. When the spec changes, every edge connecting it to a related marker produces its own todo item. Here there is 1 changed spec term and 2 related markers, so 1 × 2 = 2 todo items.

```bash
$ drift todo

1 spec item has unchecked changes.

1. [TODO] Edge between marker "a1b2c3d4" in "src/middleware/auth.go:42" and spec term "core.auth_token_expiry" in "specs/auth.drift.xml:24". The To endpoint has changed but not the From endpoint. Please check whether the new version is still reflected on the other side. Once you are satisfied, run `drift reset a1b2c3d4 core.auth_token_expiry` to mark this todo item as complete.

2. [TODO] Edge between marker "e5f6g7h8" in "src/api/handlers/login.go:88" and spec term "core.auth_token_expiry" in "specs/auth.drift.xml:24". The To endpoint has changed but not the From endpoint. Please check whether the new version is still reflected on the other side. Once you are satisfied, run `drift reset e5f6g7h8 core.auth_token_expiry` to mark this todo item as complete.
```

### One marker, many spec terms

Conversely, a single block of code can satisfy more than one spec term at once. For example, an upload handler might enforce both `core.validate_file_size` and `core.scan_for_malware`. When that marker changes, every edge connecting it to a related spec term produces its own todo item. Here there is 1 changed marker and 2 related spec terms, so 1 × 2 = 2 todo items.

```bash
$ drift todo

1 marker has unchecked changes.

1. [TODO] Edge between marker "k9l0m1n2" in "src/api/handlers/upload.go:115" and spec term "core.validate_file_size" in "specs/uploads.drift.xml:12". The From endpoint has changed but not the To endpoint. Please check whether the changed side still aligns with the other. Once you are satisfied, run `drift reset k9l0m1n2 core.validate_file_size` to mark this todo item as complete.

2. [TODO] Edge between marker "k9l0m1n2" in "src/api/handlers/upload.go:115" and spec term "core.scan_for_malware" in "specs/uploads.drift.xml:48". The From endpoint has changed but not the To endpoint. Please check whether the changed side still aligns with the other. Once you are satisfied, run `drift reset k9l0m1n2 core.scan_for_malware` to mark this todo item as complete.
```

## State file walkthrough

`.drift/state.xml` is an XML state file (version=3) inside the `.drift/` state directory. It stores baseline hashes, edges (both link-style and ref-style in a unified `<edges>` section), and resolution state. It is tool-managed — do not edit it by hand. Commit it to git.

### Clean state (no drift)

After `drift init`, `drift todo` (discovers specs/markers), and `drift link` for all edges, the `.drift/state.xml` looks like this — baselines match current content, no resolution entries:

```xml
<drift version="3">
  <specs>
    <spec id="core.validate_input" hash="S98YH3T2T32..." filepath="core/core.drift.xml" line="0"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="JHIO34YU..." filepath="src/main.go" line="15" endline="18"/>
  </markers>
  <edges>
    <edge from="4hy7fh3h" to="core.validate_input"/>
  </edges>
  <edgeResolutions>
  </edgeResolutions>
</drift>
```

The `<edges>` section unifies what used to be `<links>` and `<refs>` in earlier versions. Each edge has a `from` (the declaring endpoint — marker for link edges, source spec for ref edges) and a `to` (the referenced endpoint — spec in both cases).

### After drift detected

When a spec or marker changes, `drift todo` surfaces the edge. But `.drift/state.xml` is **not modified** by `drift todo` — the baselines stay as-is. Drift is detected by comparing current content hashes against the baselines in the file.

### After resolving an edge

```bash
$ drift reset 4hy7fh3h core.validate_input
```

Since the marker has no more unchecked specs, and the spec has no more unchecked markers, the baselines collapse — `.drift/state.xml` is updated with the new hashes, and resolution entries are pruned:

```xml
<drift version="3">
  <specs>
    <spec id="core.validate_input" hash="FGHJKNE..." filepath="core/core.drift.xml" line="0"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="0HGO24G4..." filepath="src/main.go" line="15" endline="18"/>
  </markers>
  <edges>
    <edge from="4hy7fh3h" to="core.validate_input"/>
  </edges>
  <edgeResolutions>
  </edgeResolutions>
</drift>
```

Back to clean state — baselines match current content, no resolution entries. The next `drift todo` will report "No changes detected. 1 specs, 1 markers, 1 edges in sync."

## drift.ignore

A `.gitignore`-style file at the project root. Patterns exclude files/directories from marker scanning. Directory patterns end with `/`. Comments start with `#`.

## Build gate

`make build` runs `drift todo` as a spec-drift gate before declaring the build complete. The build fails if any drift is detected. On each successful rebuild the prior `./drift` binary is backed up to `bak/drift-<UTC-timestamp>` (gitignored). Roll back with `cp bak/drift-<ts> drift`. To skip the gate and just compile, use `go build -o drift ./cmd/drift` directly.
