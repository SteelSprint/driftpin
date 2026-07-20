# Drift Skill — LLM Agent Guide

Drift is a spec-drift detection tool. Specs describe behavior in plain English. Markers wrap the code that implements each spec. When the code, the spec, or the citation graph changes, drift derives **closures** so the agent can verify alignment before resolving.

## Mental model

- **Specs** — `*.drift.xml` files with `<spec id="localid">` elements under `<main>` or `<module name="...">` roots.
- **Markers** — `// D! id=<shortcode> range-start` and `// D! id=<shortcode> range-end` comment pairs in code files, wrapping the lines that implement a spec.
- **Edges** — `marker → spec` (link, user-declared via `drift link`) or `spec → spec` (ref, auto-parsed from `<ref>` tags).
- **Closures** — per-seed drift sets. The unit of review. Each closure has an 8-character hash.

Specs and markers are symmetric nodes in a directed citation graph. Both can drift. Drift propagates along the citer chain (cited → citer), transitive to fixpoint. Markers cannot be cited, so drift through a marker stops there — the single retained asymmetry.

## The workflow (always follow this)

1. **`drift todo`** — see which closures drifted (each with an 8-character hash).
2. **`drift diff <hash>`** — review every node in the closure. `drift diff --all` reviews every closure in one pass.
3. **For each closure:** decide whether the *code* is wrong (fix the code), the *spec* is wrong (update the spec), or the *citation* is wrong (fix the `<ref>` target).
4. **`drift reset <hash>`** — resolve ONE closure at a time, only after reviewing it.

**NEVER batch-reset.** There is no `drift reset --all`. This friction is the point — blind reset defeats the tool.

**`drift todo` exit 1 means unfinished work.** Exit 0 requires both (a) all markers linked and (b) no closures derived.

## Writing specs

A spec is plain English inside an XML element:

```xml
<module name="auth">
<spec id="login">The login endpoint must reject empty username or password with HTTP 400.</spec>
</module>
```

- Spec IDs are module-qualified: `auth.login`. The local ID (after the dot) must NOT contain a dot.
- Specs can be any length, any language, any notation. The scanner hashes the spec text (with `<ref>` tags stripped before hashing — renaming a referenced spec ID does NOT invalidate the referrer's hash).

## Placing markers

Markers wrap the code that implements a spec:

```go
// D! id=auth_login range-start
func login(w http.ResponseWriter, r *http.Request) {
    username := r.FormValue("username")
    password := r.FormValue("password")
    if username == "" || password == "" {
        http.Error(w, "missing credentials", http.StatusBadRequest)
        return
    }
    // ...
}
// D! id=auth_login range-end
```

- Marker IDs (shortcodes) contain NO dot.
- Markers can be nested and overlapping — the scanner blank inner-marker declarations before hashing, so they don't interfere.
- Marker syntax (`//`, `#`, `--`, etc.) works in any comment style across any text file.

### Marker placement (refactors)

<!-- D! id=skillmp range-start -->

When you extract helpers or move code across a marker boundary:

- **Keep the marker on the public entry-point function** that the spec describes. If a spec describes what `Validate` does, the marker wraps `Validate` — not the helpers it now calls.
- Helpers stay outside the marked range unless they warrant their own spec + marker.
- Nested markers are supported but produce noisier diffs and are not preferred — prefer one marker per spec.
- Drift is content-addressed. A marker reports changes to the bytes inside its range. Refactors that move code across a marker boundary without changing behavior still produce NODE_CHANGED events (intentional). There is no "this was just a refactor" annotation — the reviewer's job is to confirm the move didn't change semantics, then reset.

<!-- D! id=skillmp range-end -->

## Linking

`drift link <marker> <module.spec>` connects a marker to a spec. After linking, drift tracks the marker's hash and the spec's hash; if either changes, drift derives a closure.

```sh
drift link auth_login auth.login
```

## Citing other specs

Inside a `<spec>` element, add a ref to declare a spec-spec edge:

```xml
<spec id="login">The login endpoint must reject empty credentials.
See <ref spec="auth.hash_password">hash_password</ref> for the canonical hashing rule.</spec>
```

- Refs are stripped from spec content before hashing. Only the label text contributes to the hash.
- First time you add a ref, `drift todo` derives a closure with an `EDGE_ADDED` event. Review and `drift reset <hash>` to baseline it.
- Future changes to the *cited* spec propagate along the citer chain: every transitively-citing spec appears in the closure, plus every marker linked to those specs.

## Closure derivation

When something drifts, the closure algorithm runs:

1. **Seeds** — register drift events (NODE_CHANGED, NODE_ADDED, NODE_REMOVED, EDGE_ADDED, EDGE_REMOVED, EDGE_BROKEN). Each event has a seed = the citer-side party of the change.
2. **Closure per seed** — BFS from the seed over incoming edges (citer walk). Closure membership = seed + transitive citers. Marker seeds additionally include their outgoing-edge targets (the specs they link to).
3. **Hash** — first 8 hex chars of SHA1(sorted node IDs + sorted undirected edge keys). Stable across drift-state changes.
4. **Merge same-hash closures** — rare; produces one closure with combined events.

Closures are strictly disjoint across seeds. Two seeds produce two closures, even if they share non-seed citers.

## CLI command reference

| Command | Description |
|---|---|
| `drift init` | Initialize `.drift/` and a starter `main.drift.xml`. |
| `drift todo` | Derive closures; report drift. Exit 0 clean, 1 drift, 2 error. |
| `drift todo --json` | JSON output (closure hash, nodes, edges, events). |
| `drift list [--verbose]` | List specs, markers, edges, sync state. |
| `drift show <marker\|spec>` | Show entity content + linked counterparts. |
| `drift diff <hash>` | Show unified diffs for every node in the closure. |
| `drift diff --all` | Show diffs for all closures. |
| `drift link <marker> <module.spec>` | Create a link edge. |
| `drift unlink <marker> <module.spec>` | Remove a link edge. |
| `drift reset <hash>` | Resolve a closure. |
| `drift config theme <name>` | Set theme. |
| `drift skill` | Print this guide. |
| `drift help` | Print command reference. |
| `drift version` | Show version. |

Global flags: `--json`, `--no-color`, `--color={auto,always,never}`, `--help`.

## Drift events

Every closure contains one or more events:

| Event | Trigger |
|---|---|
| `NODE_CHANGED` | baseline node's hash differs from scan hash |
| `NODE_ADDED` | new node in scan, not in baseline |
| `NODE_REMOVED` | baseline node not in scan |
| `EDGE_ADDED` | new spec-spec edge in scan (link edges are user-curated) |
| `EDGE_REMOVED` | spec-spec edge in baseline but not scan |
| `EDGE_BROKEN` | scan edge whose To endpoint doesn't exist (typo, or target was deleted) |

## Closure properties

- **Identity**: 8-character hash of sorted node IDs + sorted undirected edge keys. Stable across drift-state changes within the same membership; changes only when nodes/edges are added or removed.
- **Ephemeral**: closures exist for the current `drift todo` run; not stored in state.xml.
- **Per-seed**: each closure has one seed (the citer-side party of the change). Reset syncs only the seed's events. Non-seed citers' state is untouched.
- **Strictly disjoint**: two seeds produce two closures, even when sharing non-seed citers. Resetting one closure never affects another.
- **Broken edges persist**: closures with broken-edge events survive reset (the broken edge event is a no-op on reset). The user must fix the scan to clear the broken edge.

## Decision tree

<!-- D! id=skilldt range-start -->

Drift is a deterministic signal — it reports that a hash changed but does not judge whether the change is semantically consistent with the spec. The decision tree is the rubric you (human or LLM) apply to decide what to fix before resetting.

For each event in the closure, ask the question in the right column and act:

| Event kind                | Question to ask                                                       | If yes                                   | If no                                              |
| ------------------------- | -------------------------------------------------------------------- | ---------------------------------------- | -------------------------------------------------- |
| `NODE_CHANGED` on marker  | Does the code still implement the spec?                              | Reset the closure                        | Fix the code so it does, then reset              |
| `NODE_CHANGED` on spec    | Does the spec still describe what the code does?                      | Reset the closure                        | Fix the spec text (or the code, whichever is wrong), then reset |
| `NODE_ADDED`              | Is this node supposed to be tracked here?                             | Reset the closure                        | Delete or relink the file/entity, then reset      |
| `NODE_REMOVED`            | Was this deletion intentional?                                       | Reset the closure                        | Restore the file, then reset                      |
| `EDGE_ADDED`              | Is the new ref intentional?                                          | Reset the closure                        | Remove the ref from the spec text, then reset     |
| `EDGE_REMOVED`            | Was the removal intentional?                                         | Reset the closure                        | Restore the ref in the spec text, then reset      |
| `EDGE_BROKEN`             | (always — drift can't auto-fix)                                       | Fix the scan: add the missing spec, or remove the ref | — |

When two events live in the same closure (e.g. spec text changed AND the linked marker hash changed), the closure's hash stays the same; reset syncs the seed's events, which includes both.

<!-- D! id=skilldt range-end -->

## Output modes

Every command supports three modes:

- **Plain** (default when piped) — stable text, no ANSI.
- **Color** (default in TTY) — themed ANSI + syntax highlighting.
- **JSON** (`--json`) — structured output. LLM agents should use this for reliable parsing.

## The `.drift/` directory

- `state.xml` — baseline (v4). Specs, markers, edges. No resolutions table. Commit to git.
- `baselines.bin` — gob-encoded packfile of content-addressed baseline snapshots. Commit to git.
- `theme.xml` — project-level custom theme. Commit to git.
- `user-settings.xml` — per-user theme preference. Do NOT commit (gitignored).
- `state.lock` — runtime lock acquired by `fileio.Begin` for the duration of each CLI invocation. Do NOT commit (gitignored).

## Why no bulk reset?

`drift reset <hash>` accepts exactly ONE closure per invocation. There is no `--all`, no glob, no multi-arg form. This friction is the point: a bulk reset would let an LLM blindly mark everything as reviewed without actually reviewing the changes. The intended workflow is todo → diff --all → reset one closure at a time.

## Edge cases

- **Unpaired markers**: a `range-start` without a matching `range-end` (or vice versa) is an error. The scanner reports all unpaired markers at once.
- **Nested/overlapping ranges**: supported. Inner marker declarations are blanked before hashing.
- **Deleted specs/markers**: kept in baseline with empty scan hash → NODE_REMOVED event. Reset removes from baseline.
- **Orphan specs/markers**: 1-node closures. Resolved with `drift reset <hash>` like any other closure.
- **Broken refs**: EDGE_BROKEN events. Closures containing only broken-edge events are refused on reset — fix the scan (add the missing spec or remove the ref).
- **`drift reset` semantics**: syncs the closure's seed events to baseline. Prints "Closure HASH resolved. Baseline updated." on success.

## Examples

```sh
# Initialize a new project.
drift init

# Place a marker in code, then link.
drift link auth_login auth.login

# Check what drifted.
drift todo

# Review a closure.
drift diff a3f7b2c1

# Resolve the closure.
drift reset a3f7b2c1

# Use JSON for programmatic consumption.
drift todo --json

# Set theme.
drift config theme gruvbox
```
