<p align="center">
  <img src="Drift%20Headline%20Image.png" alt="Drift - Hard-link your LLM spec documents to their implemented source code" width="800" style="max-width: 100%; height: auto;" />
</p>

# Drift

**Drift** links your requirements to the exact code that makes them real. When the code or the requirements change, the tool tells you exactly what is affected — not "something in this file," but which lines, which function, which rule. One rule can point to many places in the code, so you can trace any requirement to every spot that carries it out. `drift todo` tells you what fell out of sync. `drift diff` shows you what changed. `drift show` walks you through every piece of code behind a rule. This lets AI agents check their own work against the rules before saying "done" — not just "the tests passed," but "every rule still matches its code."

## Quickstart

1. `make build`
2. Give the binary to your LLM agent and tell it to run `./drift skill` — it will learn everything it needs to use the tool.

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

## Anatomy

- **Specs** — `*.drift.xml` files containing `<spec id="...">` elements under `<main>` or `<module name="...">` roots
- **Markers** — `// D! id=<shortcode> range-start` and `// D! id=<shortcode> range-end` comment lines in code files, wrapping the code that implements a spec
- **`.drift/`** — state directory at project root containing `state.xml` (baseline hashes, links, resolution state) and `baselines/` (content-addressed baseline snapshots). Tool-managed — do not edit by hand. Commit to git.
- **CLI** — `drift init`, `drift todo`, `drift list`, `drift show`, `drift diff`, `drift link`, `drift unlink`, `drift reset`, `drift help`, `drift skill`

See [DOCUMENTATION.md](DOCUMENTATION.md) for the full documentation.
