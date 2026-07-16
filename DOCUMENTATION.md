# Documentation

## Mental model

When specs and their implementations change, `drift` informs your LLM. This gives your LLM a chance to fix any drifts as they happen. `drift` is packaged as a CLI that is intended mainly for LLMs to use.

## Self-discovery

The binary is self-describing:

- `drift help` — command reference with marker syntax, spec file format, and examples
- `drift skill` — comprehensive guide covering the full workflow, module/import system, marker hashing, and CLI reference. Designed for LLM agents to pipe into their context.

## How it works

`drift` asks you to save your specs in `*.pin.xml` files. Each file contains `<spec id="...">` elements that describe individual spec terms. `drift` then scans your code files for `D! id=<shortcode>` markers — short unique IDs placed in comments above the code that implements each spec term. Specs and markers form a many-to-many graph — a single spec term can be enforced by several markers, and a single marker can refer to several spec terms. Each link between a spec term and a marker is called an **edge**. When a change occurs on either side of an edge, `drift todo` surfaces one todo item per affected edge, with filepaths and line numbers where your LLM should check for drifts in specification.

`drift` hashes spec terms as well as markers (SHA1 of content), and saves those hashes inside `drift.pin`, which should be committed to git. This is an XML file that contains the hashes of specs and markers, the links between them, and a temporary resolution state area for partial todo-list resolutions. The algorithm manages this file itself — the user should refrain from touching `drift.pin` manually.

### Spec files

Specs are defined in `*.pin.xml` files. The entry point is `main.pin.xml` in the project root.

**main.pin.xml** (entry point — can be pure manifest or have direct specs):
```xml
<main>
  <import path="./core/core.pin.xml"/>
  <spec id="bootstrap">Initialize the project and load all modules</spec>
</main>
```

**Module files** (e.g. `core/core.pin.xml`):
```xml
<module name="core">
  <spec id="validate_input">input must be validated before processing</spec>
  <spec id="parse_input">Parse input tokens into structured data</spec>
</module>
```

Specs are **direct children** of the root element. Do not wrap them in a `<specs>` element — the scanner will reject this. Spec IDs are module-qualified: `<module>.<specId>`. Specs in `main.pin.xml` use the `main.` prefix (e.g. `main.bootstrap`).

The scanner walks the import graph starting from `main.pin.xml`. Imports are relative to the importing file. Diamond imports are deduplicated by absolute path. Duplicate module names cause an error. Cycles are detected and reported with a trace.

### Markers

Markers are `D! id=<shortcode>` comment lines in code files (`.go`, `.py`, `.js`, `.ts`, `.java`, `.c`, `.rs`, etc.):

```go
// D! id=4hy7fh3h
func handleRequest() {
    validateInput()
}
```

The scanner finds these lines, records the shortcode, filepath, and line number, and SHA1-hashes the 10 lines following the marker line. The marker pattern is a regex: `D!\s+id=(\S+)`. It can appear in any comment style (`//`, `#`, `--`, etc.).

### Links

Markers and specs have separate IDs — a marker's shortcode does not match a spec's ID. Links between them are declared in `drift.pin` via the CLI:

```bash
$ drift link 4hy7fh3h core.validate_input
Linked marker "4hy7fh3h" to spec "core.validate_input"
```

This validates that both the marker and spec exist, then persists the link. Links can be many-to-many: one marker can link to multiple specs, and one spec can link to multiple markers.

## CLI commands

| Command | Description |
|---|---|
| `drift init` | Creates `drift.pin` and a starter `main.pin.xml` template. Required before other commands. |
| `drift todo` | Scans the filesystem, reconciles with `drift.pin`, and surfaces any drift as a todo list. Does not modify `drift.pin`. |
| `drift link <marker> <module.spec>` | Declares a link between a marker and a spec term. Validates both exist and the link isn't a duplicate. Saves specs, markers, and the new link to `drift.pin`. |
| `drift reset <marker> <module.spec>` | Marks a specific edge as resolved. Saves updated state to `drift.pin`. If all edges for a node are resolved, baselines collapse automatically. |
| `drift help` | Prints command reference with marker syntax, spec file format, and examples. |
| `drift skill` | Prints a comprehensive guide for LLM agents covering the full workflow, module/import system, and drift detection model. |

## Reconciliation

When `drift todo` or `drift reset` runs, the orchestrator:

1. Loads `drift.pin` (baseline hashes, links, resolution state)
2. Scans the filesystem (current specs, markers, and their hashes)
3. **Reconciles** — for each discovered spec/marker:
   - If already in `drift.pin` → keeps the baseline hash from the pin, updates filepath/line if changed
   - If new (not in pin) → baseline = current hash (no drift on first discovery)
   - If in pin but not found on disk → error
4. Builds the scan and runs the core algorithm

This means the first `drift todo` after adding spec files or code markers discovers them and sets their baselines. On subsequent runs, changes are detected by comparing current hashes against these baselines.

## Example

Let's say you have a marker `4hy7fh3h` and a spec file that has `validate_input` in the `core` module.

First, initialize and discover:

```bash
$ drift init
Initialized drift.pin and main.pin.xml
Edit main.pin.xml to add your specs, then place D! id=<markerid> markers in your code.
Run `drift skill` for a comprehensive guide.

$ drift todo
No drift: 1 specs, 1 markers, 0 links in sync.
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

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "core.validate_input" in "/workspaces/my-project/core/core.pin.xml:1". The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary. Once you are satisfied, run `drift reset 4hy7fh3h core.validate_input` to mark this todo item as complete.
```

At this point `drift.pin` is still unchanged — `drift todo` doesn't modify the file.

Then you mark the edge as resolved:

```bash
$ drift reset 4hy7fh3h core.validate_input
```

Since the marker has no more unchecked specs, and the spec has no more unchecked markers, the baselines collapse — `drift.pin` is updated with the new hashes. The next `drift todo` will report "No drift: ..."

## Drift detection output

When there is no drift, `drift todo` distinguishes between empty and synced:

- **Empty** (no specs or markers registered): `Nothing to check: no specs or markers registered.`
- **Synced**: `No drift: N specs, M markers, K links in sync.`

When there is drift, it prints the count of changed markers and specs, then lists each todo item with:
- The edge (marker ↔ spec)
- File locations with line numbers
- The drift reason (marker changed, spec changed, or both)
- The `drift reset` command to run once resolved

## Many-to-many relationships

Specs and markers form a many-to-many graph. A single spec term can be enforced by several markers (e.g. the same rule applied in multiple places), and a single marker can refer to several spec terms (e.g. one block of code that satisfies multiple requirements). Because todos are edge-based, the number of todo items is the product of changed specs and their related markers.

### One spec term, many markers

A spec term like `core.auth_token_expiry` is often enforced in more than one place — say a middleware layer and a login handler. When the spec changes, every edge connecting it to a related marker produces its own todo item. Here there is 1 changed spec term and 2 related markers, so 1 × 2 = 2 todo items.

```bash
$ drift todo

1 spec item has unchecked changes.

1. [TODO] Edge between marker "a1b2c3d4" in "src/middleware/auth.go:42" and spec term "core.auth_token_expiry" in "specs/auth.pin.xml:24". The spec term has changed but not the marker. Please check whether the new version of the spec term is still reflected in the code and make any modifications necessary. Once you are satisfied, run `drift reset a1b2c3d4 core.auth_token_expiry` to mark this todo item as complete.

2. [TODO] Edge between marker "e5f6g7h8" in "src/api/handlers/login.go:88" and spec term "core.auth_token_expiry" in "specs/auth.pin.xml:24". The spec term has changed but not the marker. Please check whether the new version of the spec term is still reflected in the code and make any modifications necessary. Once you are satisfied, run `drift reset e5f6g7h8 core.auth_token_expiry` to mark this todo item as complete.
```

### One marker, many spec terms

Conversely, a single block of code can satisfy more than one spec term at once. For example, an upload handler might enforce both `core.validate_file_size` and `core.scan_for_malware`. When that marker changes, every edge connecting it to a related spec term produces its own todo item. Here there is 1 changed marker and 2 related spec terms, so 1 × 2 = 2 todo items.

```bash
$ drift todo

1 marker has unchecked changes.

1. [TODO] Edge between marker "k9l0m1n2" in "src/api/handlers/upload.go:115" and spec term "core.validate_file_size" in "specs/uploads.pin.xml:12". The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary. Once you are satisfied, run `drift reset k9l0m1n2 core.validate_file_size` to mark this todo item as complete.

2. [TODO] Edge between marker "k9l0m1n2" in "src/api/handlers/upload.go:115" and spec term "core.scan_for_malware" in "specs/uploads.pin.xml:48". The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary. Once you are satisfied, run `drift reset k9l0m1n2 core.scan_for_malware` to mark this todo item as complete.
```

## Drift.pin walkthrough

`drift.pin` is an XML state file at the project root. It stores baseline hashes, links, and resolution state. It is tool-managed — do not edit it by hand. Commit it to git.

### Clean state (no drift)

After `drift init`, `drift todo` (discovers specs/markers), and `drift link` for all edges, the `drift.pin` looks like this — baselines match current content, no resolution entries:

```xml
<drift>
  <specs>
    <spec id="core.validate_input" hash="S98YH3T2T32..." filepath="core/core.pin.xml" line="0"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="JHIO34YU..." filepath="src/main.go" line="15"/>
  </markers>
  <links>
    <link specId="core.validate_input" markerId="4hy7fh3h"/>
  </links>
  <resolutions>
  </resolutions>
</drift>
```

### After drift detected

When a spec or marker changes, `drift todo` surfaces the edge. But `drift.pin` is **not modified** by `drift todo` — the baselines stay as-is. Drift is detected by comparing current content hashes against the baselines in the file.

### After resolving an edge

```bash
$ drift reset 4hy7fh3h core.validate_input
```

Since the marker has no more unchecked specs, and the spec has no more unchecked markers, the baselines collapse — `drift.pin` is updated with the new hashes, and resolution entries are pruned:

```xml
<drift>
  <specs>
    <spec id="core.validate_input" hash="FGHJKNE..." filepath="core/core.pin.xml" line="0"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="0HGO24G4..." filepath="src/main.go" line="15"/>
  </markers>
  <links>
    <link specId="core.validate_input" markerId="4hy7fh3h"/>
  </links>
  <resolutions>
  </resolutions>
</drift>
```

Back to clean state — baselines match current content, no resolution entries. The next `drift todo` will report "No drift: 1 specs, 1 markers, 1 links in sync."

## drift.ignore

A `.gitignore`-style file at the project root. Patterns exclude files/directories from marker scanning. Directory patterns end with `/`. Comments start with `#`.
