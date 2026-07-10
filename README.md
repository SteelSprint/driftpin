# filament

<!-- #F id:po2f76jx tool.location -->
<!-- #F id:67qfxxk6 tool.language -->
<!-- #F id:k2m9rof3 tool.design -->
<!-- #F id:mlh14n33 versioning.amendments -->

Your spec.md files are going out of sync with your code, and you can't tell
which clauses are actually implemented. filament fixes this — it turns specs
from prose into enforced, traceable contracts.

## The problem

You write specs in markdown. Developers — or LLMs — implement them. Over
time, nobody can answer:

- Did the code follow the spec?
- Which code implements which clause?
- When the spec changed, was the code updated?
- When the code changed, was the spec updated?

```
  spec.md                      code
  ┌──────────┐                ┌──────────┐
  │ # Server │     ???        │ ListenAnd│
  │ port 8080│   no link      │ Serve:3000│
  └──────────┘                └──────────┘

  Silent drift. You accept it as inevitable. It's not.
```

## What filament does

filament turns specs from unstructured prose into enforced contracts:

- Specs become XML with clause ids — structured, not .md
- Markers (`#F`) in code trace to spec clauses — provenance
- A state file (`.filament`) tracks what's been reviewed
- When spec or code changes, drift is detected — nothing is silent

```
  spec.xml                     code
  ┌──────────┐                ┌──────────────┐
  │server.port│◄────#F───────│ // #F id:abc  │
  │"port 8080"│    marker     │ :8080        │
  └──────────┘                └──────────────┘
       │                              │
       ▼                              ▼
  ┌──────────────────────────────────────┐
  │          .filament (state)           │
  │  spec hash · content hash · reviewed │
  └──────────────────────────────────────┘

  Spec changed? → SPEC_DRIFT → review code
  Code changed? → SITE_DRIFT → review spec
  Both in sync? → ✓
```

## How it changes your workflow

```
  BEFORE                              AFTER
  ┌──────────┐                       ┌──────────┐
  │ Human    │ writes spec           │ Human    │ writes spec
  │ Human    │ checks code           │ LLM      │ places markers
  │ Human    │ updates spec          │ Filament │ detects drift
  │ Human    │ updates code          │ LLM      │ reviews + resolves
  │ Human    │ repeats manually      │ Human    │ reviews reports
  └──────────┘                       └──────────┘

  Manual. Unscalable.                Automated. Human at spec level.
  Specs drift silently.              Drift is caught and reviewed.
```

Your job shifts from checking code against specs to writing good specs and
reviewing drift reports. The LLM places markers, runs checks, reviews
drift, and resolves it — all prompted by you, all in prose.

## What filament believes

1. **Specs are the source of truth.** Code is an implementation.
2. **Every implementation site must trace to a spec clause.** Without a
   marker, there's no proof a clause is addressed.
3. **Changes in either direction must be reviewed.** Nothing is silent.
4. **Drift is a review signal, not a compliance checker.** filament flags
   that something changed; the review is for you or your LLM.

## Install

**macOS / Linux:**

```
curl -fsSL https://raw.githubusercontent.com/steelsprint/filament/main/scripts/install.sh | bash
```

**Windows (PowerShell):**

```
irm https://raw.githubusercontent.com/steelsprint/filament/main/scripts/install.ps1 | iex
```

Installs the latest release binary to `~/.local/bin` (macOS/Linux) or
`%USERPROFILE%\.filament\bin` (Windows) and tells you how to add it to
your PATH.

**Go users:**

```
go install github.com/steelsprint/filament@latest
```

**Manual:** download from [GitHub Releases](https://github.com/steelsprint/filament/releases).

## Quickstart

<!-- #F id:n0j7m2mk public_api.subcommands -->

Copy this prompt into your LLM:

> run `filament skill`. set up filament in my project and tell me what you find.

The LLM runs `filament skill` to learn the tool, reads your spec and
source files, creates the state file, generates markers for missing
clauses, tells you where to paste them, and checks for drift. You
review.

## Prompt library

Each prompt is copy-pasteable. Your LLM runs `filament skill` to learn
the tool's full capabilities.

### Check for drift

> check my project for drift and tell me what you find.

The LLM runs `filament check`, reads each finding, and reports what
changed and what to do about it.

### After updating the spec

> i updated the spec. check what drifted, review each one against
> the code, and clear it if the code still matches.

The LLM syncs spec hashes, checks for SPEC_DRIFT, reviews each
flagged site against the new spec wording, and resolves only what's
still aligned.

### After changing code

> i changed some code. check if anything drifted, review it against
> the spec, and clear what's still aligned.

The LLM checks for SITE_DRIFT, reads the spec clause(s) each marker
traces to, compares against the new content, and resolves only what's
still aligned.

### Add a new clause

> i want to add a new clause to the spec. the clause is <id> and
> the text is: "<text>". set it up and tell me where to put the
> marker.

The LLM edits the spec, syncs, generates a marker, tells you where
to paste it, registers it, and verifies.

### Full audit (PR review)

> do a full audit of this project's spec alignment. check everything,
> resolve any drift, and give me a summary.

The LLM runs skill, status, check, resolves all drift, and reports
what changed and why.

### CI gate

> i want filament to fail my CI build if there's drift.

The LLM shows the CI step: `filament check` (exit 0 = pass, exit 1
= fail).

## Command reference

<!-- #F id:7gvifkj2 public_api.file_walk -->
<!-- #F id:j094ghw4 self_hosting.test -->
<!-- #F id:zogz2vjr versioning.source -->

| Command | Description |
|---------|-------------|
| `check [paths]` | Verify all markers are in sync. Exit 1 on drift. |
| `status [paths]` | Show every marker and its state. Exit 0 always. |
| `init [paths]` | Create `.filament` state file. |
| `add <clauses>` | Print `#F` marker line with a new marker id. |
| `resolve --spec <ids>` | Clear spec drift for given markers. |
| `resolve --site <ids>` | Clear site drift for given markers. |
| `sync` | Refresh spec hashes from current spec XML. |
| `migrate [paths]` | Convert old `filament:hash` to `#F` markers. |
| `skill` | Print the full usage guide. |

| Flag | Description |
|------|-------------|
| `--spec=<path>` | Path to spec XML (default: `./filament.spec.xml`) |
| `--quiet` | Suppress tooltip preamble |

## Documentation

- [Tutorial](documentation/tutorial.md) — the mental model, with worked examples
- [Marker format](documentation/marker-format.md) — the `#F` directive syntax
- [State file](documentation/state-file.md) — `.filament` structure
- [Public API](documentation/public-api.md) — all commands with examples
- [For LLMs](documentation/for-llms.md) — how to read spec + code together

Run `filament skill` for the comprehensive built-in guide.

## Upgrading filament

When filament's own format changes, follow this bootstrap pattern:

```
1. Build the current filament binary
2. Update the spec to describe the new design
3. Update all hash comments to match the new spec
4. Old check passes → commit (checkpoint)
5. Implement the new code in one pass
6. Run filament migrate to convert markers
7. New check passes → commit
```

The old tool validates the new spec's structure; the new tool validates
the new format. Step 4 is the checkpoint — the spec is updated and
internally consistent before the implementation changes.

## License

[MIT](LICENSE)
