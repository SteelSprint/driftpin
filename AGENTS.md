# Drift — Agent Guide

Drift is a spec-drift detection tool for LLM coding agents. Specs describe behavior; markers wrap the code that implements each spec. Specs also cite each other via `<ref>` tags — those citations are tracked too, so editing a spec surfaces drift on every spec transitively connected to it. When any side changes, `drift todo` surfaces the drift so the agent can verify alignment before resolving.

## Spec discipline workflow (MUST follow)

1. **`drift todo`** — check what drifted (specs, markers, refs, or unlinked)
2. **`drift diff --all`** — review every broken edge's changes in one pass
3. **For each edge:** decide whether the *code* is wrong (fix the code), the *spec* is wrong (update the spec), or the *citation* is wrong (fix the `<ref>` target)
4. **`drift reset <from> <to>`** — resolve ONE edge at a time, only after reviewing it

**NEVER batch-reset.** There is no `drift reset --all`. This friction is the point — blind reset defeats the tool.

**`drift todo` exit 1 means unfinished work.** Exit 0 requires both (a) all markers linked and (b) all edges in sync. Unlinked markers are actionable drift.

## Critical rules

- **Specs are the source of truth.** When a spec and code disagree, first decide which is correct. If the code is right, update the spec *then* reset. If the spec is right, fix the code *then* reset.
- **Spec IDs have exactly one dot** (module separator): `main.bootstrap`, `orch.link`. Marker shortcodes have no dot. Never put a dot in a `<spec id="...">` local ID.
- **Markers wrap the implementation region** with `// D! id=<shortcode> range-start` and `// D! id=<shortcode> range-end`. The scanner hashes the lines between the markers.
- **Refs (`<ref spec="module.localid">label</ref>`) declare spec-spec edges.** The scanner parses them from spec content; they are stored in `state.xml` as baseline edges. Direction records who-cited-whom (used for cycle detection); drift propagation is rhizomatic (undirected). Renaming a referenced spec ID does NOT invalidate the referrer's hash — refs are stripped from spec content before hashing.
- **No directed cycles among spec-spec edges.** `$1 → $2 → $1` is rejected by validation. The scanner reports all cycles in one pass.
- **Commit `.drift/state.xml` and `.drift/baselines/` to git.** They are shared baselines, not local artifacts. Do NOT commit `.drift/user-settings.xml` or `.drift/state.lock` (both gitignored).
- **State file locking is built in.** Concurrent `drift link`/`unlink`/`reset` calls are safe — flock (Unix) or LockFileEx (Windows) serializes Load→Save. Safe to batch these in parallel tool calls.

## Build / test / lint

```sh
make build                              # build + drift gate (preferred)
go build -o drift ./cmd/drift           # build only, skip gate
go test -race -count=1 ./...            # full suite with race detector
GOOS=windows go build -o /dev/null ./statestore/   # verify Windows compiles
```

- Module path is `drift`, Go 1.26.
- One external dependency: `golang.org/x/sys` (for cross-platform file locking in `statestore/`). Do not add dependencies without strong justification.
- The race test (`cli/race_test.go`) runs on every `go test ./...` — it is a regression guard for concurrent state mutations, not optional.
- `make build` runs `./drift todo` as a spec-drift gate. The build fails if any drift is detected. On each successful rebuild the prior binary is backed up to `bak/drift-<UTC-timestamp>` (gitignored). Roll back with `cp bak/drift-<ts> drift`.

## Repo layout

```
cmd/drift/       # main() entry point
cli/             # CLI dispatch, command structs, output layer (Plain/Color/JSON)
  commands/      # one struct per subcommand (init, todo, link, reset, …)
  output/        # presenters, themes, tokenizer, user settings
core/            # core algorithm (evaluated state, scan, reconcile)
scanner/         # file scanner — specs and refs from *.drift.xml, markers from code
statestore/      # FileStateStore (state.xml), BaselineStore, file locking
orchestrator/    # wires scanner + statestore + core; mutating methods hold lock
eval/            # eval harness (subjects an LLM to a drift fixture, judges result)
internal/        # diff, testutil
business/        # product spec hierarchy (goals → modules → intent → impl)
model.drift.xml  # CONCEPTUAL SPEC — the rhizomatic edge model (above all impls)
```

## Specs in this repo

The drift codebase is self-hosting on drift. Specs live in `*.drift.xml` files next to the code they describe:

- `model.drift.xml` — conceptual spec for the unified edge model (notation, axioms, algorithm)
- `cli/cli.drift.xml` — CLI command contracts
- `orchestrator/orchestrator.drift.xml` — orchestrator method contracts
- `cli/output/output.drift.xml` + `output_impl.drift.xml` — output layer (L1/L2/L3)
- `business/` — product-level goal hierarchy

Current state: 126 specs, 69 markers, 151 edges. `drift todo` should report clean on a resting tree.

## Editing code that drift tracks

When you change code inside a `// D! id=… range-start … range-end` region:

1. Run `drift todo` — the edge will show as drifted (TodoEdgeDrift)
2. Run `drift diff <marker> <module.spec>` — see the code delta
3. Read the linked spec and decide: does the spec still describe the new code?
4. If yes → `drift reset <marker> <module.spec>` (baseline collapses)
5. If no → update the spec text in the `*.drift.xml` file, then reset

When you change a spec's wording in a `*.drift.xml` file:

1. Run `drift todo` — the edge will show as drifted (spec side)
2. Read the linked marker region in the code
3. Decide: does the code still implement the new spec?
4. If yes → `drift reset`
5. If no → fix the code, then reset

If the spec you changed is referenced by other specs (via `<ref>`), every transitively-connected spec gets **chain drift** (TodoEdgeDrift with SourceSpecID set) and every marker linked to those specs gets **cascade drift** (TodoCascade). Cascade todos are derived — resolving the upstream chain drift clears them automatically.

## Adding new specs

1. Add `<spec id="localid">description</spec>` to the relevant `*.drift.xml` module file (local ID must NOT contain a dot)
2. Wrap the implementing code region with `// D! id=<shortcode> range-start` / `range-end`
3. `drift link <shortcode> <module.localid>`
4. `drift todo` — should report clean

## Citing other specs

1. Add `<ref spec="module.localid">label text</ref>` (or self-closing `<ref spec="module.localid" />`) inside a `<spec>` element's content. The label text is preserved in the canonical hash; the `<ref>` tag is stripped.
2. `drift todo` — first time, this surfaces as **TodoEdgeAdded** (a new spec-spec edge). Review and run `drift reset <your-spec> <cited-spec>` to baseline it.
3. Future changes to the *cited* spec will propagate rhizomatically: chain drift on every transitively-connected spec, cascade drift on every marker linked to them.

## Reset dispatch (dots are the discriminator)

| Args | Form | Example |
|---|---|---|
| `<id>` (1 arg) | orphan removal | `drift reset main.deleted_spec` |
| `<marker> <spec>` (no dot, dot) | link-edge reset | `drift reset cval core.validate` |
| `<spec> <spec>` (dot, dot) | spec-spec edge reset | `drift reset output.color_mode output.color_palette` |

Resolutions are keyed on the (from, to) pair; either direction covers the edge declaration.

## Eval harness

`eval/` runs an LLM ("subject") against a drift fixture workspace, then a judge LLM scores the result. Used to validate that agents can use drift correctly and that drift itself doesn't have UX footguns.

```sh
go run ./eval --battery --repeat 10 --subject <model> --judge <model>
```

Per-prompt overrides via `<name>-subject.md` and `<name>-judge.md` files alongside `<name>.md`. The `--repeat N` flag runs the same prompt N times in parallel for a statistical baseline.

## Output modes

Every command supports three output modes:

- **Plain** (default when piped) — stable text, no ANSI
- **Color** (default in TTY) — themed ANSI + syntax highlighting
- **JSON** (`--json`) — structured output for programmatic consumption

For scripting or LLM consumption, use `--json` or `--no-color`.

## Themes

`drift config theme <name>` sets a per-user preference (stored in `.drift/user-settings.xml`, not committed). 12 built-in themes. Project-level custom theme via `.drift/theme.xml` (committed, full override of all 18 elements).

## Quick reference

| Task | Command |
|---|---|
| What drifted? | `drift todo` |
| Show the diffs | `drift diff --all` |
| Resolve one edge | `drift reset <from> <to>` |
| List everything | `drift list --verbose` |
| Show one entity | `drift show <marker\|spec>` |
| Full guide | `drift skill` |
| Command reference | `drift help` |
| Structured output | `drift todo --json` |
