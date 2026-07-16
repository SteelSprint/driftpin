Driftpin is a spec-drift detection tool designed for LLM coding agents. It tracks the relationship between specification terms (specs) and the code that implements them (markers). When either side changes, `drift todo` surfaces the drift so the agent can verify alignment.

# Quick Start

```
drift init            # Initialize: creates drift.pin + a starter main.pin.xml + example.go
drift help            # Show command reference
drift skill           # Print this guide (pipe to a file or read into context)
```

# Workflow

1. **Initialize**: `drift init` — creates `drift.pin` (state file) and `main.pin.xml` (spec entry point template). Edit `main.pin.xml` to add your specs.

2. **Write specs**: Edit `*.pin.xml` files. Each file has a root `<module name="...">` (or `<main>` for the entry point). Specs are `<spec id="...">description</spec>` elements — they must be **direct children** of the root element, not nested inside a `<specs>` wrapper.

3. **Place markers**: Add `// D! id=<markerid>` comment lines in your code, directly above the code that implements a spec. The marker IDs are short unique strings you choose.

4. **Link markers to specs**: `drift link <marker> <module.spec>` — connects a marker to a spec. Spec IDs are module-qualified (e.g. `core.validate`).

5. **Check for drift**: `drift todo` — scans specs and markers, compares hashes against baselines, and reports any drift as a todo list.

6. **Resolve drift**: After verifying that code and specs are still aligned, run `drift reset <marker> <module.spec>` to mark the edge as resolved and collapse baselines.

# Spec Files

Specs live in `*.pin.xml` files. The entry point is `main.pin.xml` in the project root.

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
  <spec id="validate">Input must be validated before processing</spec>
  <spec id="parse">Parse input tokens into structured data</spec>
</module>
```

Spec IDs are qualified as `<module>.<specId>`. Specs in `main.pin.xml` use the `main.` prefix (e.g. `main.bootstrap`). Imports are relative to the importing file. Diamond imports are deduplicated by absolute path. Cycles are detected and reported with a trace.

**ID format invariants:** The local `id` attribute in a `<spec>` element must NOT contain a dot — dots are reserved for module qualification (e.g. `module.specid`). Marker shortcodes must NOT contain a dot either. This ensures every spec ID has exactly one dot (separating module from local ID) and marker IDs have none, enabling unambiguous disambiguation in CLI commands like `drift reset <id>`.

# Markers

Markers are comment lines in code files matching the pattern `D! id=<shortcode>`. They go on the line **above** the code they annotate. The scanner hashes the 10 lines following the marker line.

```go
// D! id=validate_input
func handleRequest() {
    validateInput()
}
```

Supported file extensions: `.go`, `.py`, `.js`, `.ts`, `.jsx`, `.tsx`, `.java`, `.c`, `.cpp`, `.h`, `.hpp`, `.rs`, `.rb`, `.php`, `.swift`, `.kt`, `.cs`, `.scala`, `.sh`, `.bash`, `.lua`, `.dart`, `.vue`, `.svelte`.

The marker pattern is a regex: `D!\s+id=(\S+)`. It can appear in any comment style (`//`, `#`, `--`, `/* */`, etc.).

# CLI Commands

| Command | Description |
|---|---|
| `drift init` | Create `drift.pin` and `main.pin.xml` template. |
| `drift todo` | Scan specs and markers, report drift. Does not modify `drift.pin`. |
| `drift list` | Show all specs, markers, links, and sync state. Read-only. |
| `drift link <marker> <module.spec>` | Connect a marker to a spec. Both must exist on disk. |
| `drift unlink <marker> <module.spec>` | Remove a link between a marker and a spec. Also clears resolution state for that edge. |
| `drift reset <marker> <module.spec>` | Mark a drifted edge as resolved. Collapses baselines when all edges for a node are resolved. |
| `drift help` | Show command reference with examples. |
| `drift skill` | Print this guide (for LLM agents learning the tool). |

# How Drift Detection Works

`drift` SHA1-hashes spec content (the text inside `<spec>` elements) and marker content (the 10 lines following the marker line). These hashes are stored as baselines in `drift.pin`. On each `drift todo`, current hashes are compared against baselines:

- **No drift**: All hashes match → "No changes detected. N specs, M markers, K links in sync."
- **Marker changed**: The code near a marker was modified. Check if it still matches the spec.
- **Spec changed**: The spec text was modified. Check if the code still implements it.
- **Both changed**: Both sides changed. Verify alignment on both sides.

Drift is per-edge (one marker ↔ one spec). If 1 spec is linked to 3 markers and the spec changes, that's 3 todo items. `drift reset <marker> <module.spec>` resolves one edge. When all edges for a node are resolved, the baseline collapses to the current hash.

# drift.pin

`drift.pin` is an XML state file at the project root. It stores baseline hashes, links, and resolution state. It is tool-managed — do not edit it by hand. Commit it to git.

# drift.ignore

A `.gitignore`-style file at the project root. Patterns exclude files/directories from marker scanning. Directory patterns end with `/`. Comments start with `#`.
