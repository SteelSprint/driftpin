<p align="center">
  <img src="Drift%20Headline%20Image.png" alt="Drift - Hard-link your LLM spec documents to their implemented source code" width="800" style="max-width: 100%; height: auto;" />
</p>

# Drift

**Drift** links your requirements to the exact code that makes them real. Specs cite each other via `<ref>` tags, so the spec graph is tracked too — editing a foundational spec surfaces drift on every spec transitively connected to it. When the code, the requirements, or the citations change, the tool tells you exactly what is affected — not "something in this file," but which lines, which function, which rule. One rule can point to many places in the code, so you can trace any requirement to every spot that carries it out. `drift todo` tells you what fell out of sync. `drift diff` shows you what changed. `drift show` walks you through every piece of code behind a rule. This lets AI agents check their own work against the rules before saying "done" — not just "the tests passed," but "every rule still matches its code."

## Zero dependencies

Single static binary. No runtime, no libraries, no config files — just one executable you can drop anywhere.

## Language agnostic

Drift works with any programming language — and any text file. Specs are plain XML; markers are comment lines (`// D! id=... range-start` / `// D! id=... range-end`) that work in any comment style — `//`, `#`, `--`, `/* */`. The scanner detects text files by extension blocklist (skips known binary formats) plus a null-byte content sample, so any text file of any extension is scanned. If you can write a comment in it, drift can track it.

## Three output modes

Every command supports three output modes via global flags:

- **Color** (default in a terminal) — themed ANSI output with syntax highlighting on code content. 12 built-in themes including Solarized, Gruvbox, Nord, and Dracula. Set your theme with `drift config theme gruvbox`.
- **Plain** (default when piped) — clean text with no escape codes. Safe for pipelines, redirects, and CI logs. Automatically selected when stdout is not a TTY.
- **JSON** (`--json`) — structured JSON for programmatic consumption. LLM agents should use `--json` to parse output reliably.

```bash
drift todo --json               # structured JSON for LLM consumption
drift todo --color=always       # force color even when piped
drift config theme nord         # set theme preference
```

JSON mode never emits ANSI codes — it's always plain structured data.

## Install

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/SteelSprint/Drift/main/scripts/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/SteelSprint/Drift/main/scripts/install.ps1 | iex
```

Or pin a version:

```bash
# macOS / Linux
DRIFT_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/SteelSprint/Drift/main/scripts/install.sh | bash

# Windows
$env:DRIFT_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/SteelSprint/Drift/main/scripts/install.ps1 | iex
```

Installs to `~/.local/bin/drift` (macOS/Linux) or `%USERPROFILE%\.local\bin\drift.exe` (Windows); override with `DESTDIR`. Add it to your `PATH` if needed. To build from source instead: `make build` (or `go build -o drift ./cmd/drift`).

## Quickstart

1. Install drift (see [Install](#install) above).
2. Give the binary to your LLM agent and tell it to run `drift skill` — it will learn everything it needs to use the tool.

That's it. The tool is self-documenting. `drift skill` prints a complete guide that teaches the agent how to write specs, place markers, link them, check for drift, see what changed, and resolve it. You don't need to read docs — your agent will.

## Example: An AI agent changed your code — did it stay true to the rules?

You have a TODO app in Python. You wrote a rule: *"The title must not be empty."* You asked an AI agent to add a feature. It changed the code — but it also snuck in a new rule you didn't ask for. Drift catches this.

**Step 1 — Check for drift.** `drift todo` scans your rules and your code. If anything fell out of sync, it tells you exactly where:

```bash
$ drift todo

1 marker has unchecked changes.

1. [TODO] Edge between marker "add_func" in "app.py:5" and spec term
   "main.add_todo" in "main.drift.xml:0". The marker has changed but not
   the spec term. Check whether the changed code still complies with
   the spec term and make any modifications necessary. Once you are
   satisfied, run `drift reset add_func main.add_todo` to mark this
   todo item as complete.
  → Run 'drift diff add_func main.add_todo' to see what changed.
```

Something changed. The tool doesn't just say "file changed" — it says **which rule** is affected and **which code** implements it.

**Step 2 — See what changed.** The hint above says to run `drift diff`. This shows you a side-by-side comparison of the code before and after the change:

```bash
$ drift diff add_func main.add_todo

Spec: main.add_todo (main.drift.xml)
Status: in sync

---

Marker: add_func (app.py:5-12)
Baseline: a1b2c3d4   Current: e5f6g7h8

--- baseline
+++ current
@@ -3,3 +3,5 @@
 def add_todo(title):
     if not title:
         raise ValueError("title must not be empty")
+    if len(title) < 3:
+        raise ValueError("title must be at least 3 characters")
     todos.append({"title": title})
```

The `+` lines are what the agent added. Your rule said "must not be empty." The agent added "must be at least 3 characters" — a new rule you never wrote. Now you can decide: is that a good addition? If so, update the rule. If not, remove it.

**Step 3 — Review the full picture.** Before deciding, look at the rule and the code side by side with `drift show`:

```bash
$ drift show main.add_todo

=== Spec: main.add_todo ===
File: main.drift.xml
Hash: a1b2c3d4

Add a new todo item. The title must not be empty.

=== Marker: add_func ===
File: app.py
Lines: 5-12
Hash: e5f6g7h8

def add_todo(title):
    if not title:
        raise ValueError("title must not be empty")
    if len(title) < 3:
        raise ValueError("title must be at least 3 characters")
    todos.append({"title": title})
```

Now you see both sides. The rule says one thing; the code does two. You decide the 3-character minimum is a good idea, so you update the rule to say *"The title must not be empty and must be at least 3 characters."* Then you run `drift reset add_func main.add_todo` to tell the tool: **I've checked this, accept the new version.**

That's the whole loop: **detect → see what changed → review → resolve.** The tool makes sure no AI-generated code sneaks past your rules unnoticed.

## Self-discovery

- `./drift help` — command reference with examples
- `./drift skill` — comprehensive guide for LLM agents (pipe to context)

## Development principles

<!-- D! id=selfhost range-start -->
Drift is self-hosting: it tracks its own specs and markers. `drift todo` must be clean before any commit — this is a hard gate, not a suggestion. The project is its own primary test case. If drift can't track itself correctly, it can't track anything. A bug that breaks `drift todo` on drift's own codebase blocks all other work until fixed.
<!-- D! id=selfhost range-end -->

<!-- D! id=testfirst range-start -->
Bugs are fixed test-first. Write the test that reproduces the bug, confirm it fails for the right reason, then fix the code and confirm the test passes. The failing test is proof you understand the bug before you touch the fix. Never fix a bug without first writing the test that reproduces it.
<!-- D! id=testfirst range-end -->

## Anatomy

- **Specs** — `*.drift.xml` files containing `<spec id="...">` elements under `<main>` or `<module name="...">` roots. Specs cite each other via `<ref spec="module.localid">label</ref>` tags inside spec content; refs are tracked as spec-spec edges and propagate drift rhizomatically.
- **Markers** — `// D! id=<shortcode> range-start` and `// D! id=<shortcode> range-end` comment lines in code files, wrapping the code that implements a spec. Marker-spec connections (link edges) are created via `drift link`.
- **Edges** — unified storage for both link edges (marker → spec) and ref edges (spec → spec). Stored in a single `<edges>` section in `.drift/state.xml`. Direction records who-declared-who (used for cycle detection); drift propagation is undirected (rhizomatic).
- **`.drift/`** — state directory at project root containing `state.xml` v3 (baseline hashes, unified edges, resolution state), `baselines/` (content-addressed baseline snapshots), optional `theme.xml` (project-level custom theme), and `user-settings.xml` (per-user theme preference, gitignored). Tool-managed — do not edit by hand. Commit to git (except user-settings.xml).
- **CLI** — `drift init`, `drift todo`, `drift list`, `drift show`, `drift diff`, `drift link`, `drift unlink`, `drift reset`, `drift config theme`, `drift help`, `drift skill`, `drift version`. `drift reset` dispatches on dots: `<marker> <spec>` for link edges, `<spec> <spec>` for ref edges. Global flags: `--json`, `--no-color`, `--color={auto,always,never}`.
- **Build gate** — `make build` runs `drift todo` before declaring the build complete. The build fails if any drift is detected. Prior binary backed up to `bak/drift-<UTC-timestamp>` on each successful rebuild (gitignored).

See [DOCUMENTATION.md](DOCUMENTATION.md) for the full documentation.
