package main

const SkillText = `WHAT IS FILAMENT?

filament tracks whether files stay aligned with their spec. It works on
any plaintext file — source code, documentation, configuration, SQL, HTML,
plain text. Markers (#F) in your files trace to clauses in a spec XML.
filament detects two kinds of drift:

  - Spec drift: the spec changed. Every file location referencing the
    changed clause is flagged so you can verify the content still matches.
  - Site drift: the content near a marker changed. The clause(s) it traces
    to are flagged so you can verify the spec still describes what the
    content does.

Both directions force review. This is the point: spec and files must stay
traceable to each other, and changes in either direction must be
consciously acknowledged, not silently merged.


THE MARKER FORMAT

Markers are placed in any text file, in any comment style:

  Go:       // # F id:example1 tool.name tool.binary
  Python:   # # F id:example1 tool.name tool.binary
  SQL:      -- # F id:example1 tool.name tool.binary
  HTML:     <!-- # F id:example1 tool.name tool.binary -->
  Markdown: <!-- # F id:example1 tool.name tool.binary -->

The format is: #F id:<marker_id> <clause_id> <clause_id> ...

The marker_id is an 8-character identifier (lowercase letters and digits).
The clause_ids are dotted-path identifiers from the spec XML.

The tool matches the #F directive as a substring, regardless of the
comment character that precedes it. This allows markers to work in any
text file with any comment style.


SPEC XML SCHEMA

The spec XML has these elements:

  <spec name="...">  — root element, name is required
  <description>      — optional, at most one, top-level only, prose only
  <definitions>      — optional, at most one, container for <term>
  <term text="...">  — prose + optional <ref> children, text attr is the id
  <section id="..." label="..."> — container, can nest sections and clauses
  <clause id="...">  — leaf, prose + optional <ref> children
  <ref>id</ref>      — inline reference inside clauses/terms only

IDs are dotted-path identifiers, segments match [a-z0-9_]+. A child
element's id must extend its parent section's id (e.g., parent
'operations' → child 'operations.create'). Each id must be unique.

References: <ref>id</ref> creates an explicit reference inside a clause
or term. Prose words are NOT references — only <ref> elements are. The
text inside <ref> must be a defined id. Terms may only reference other
terms, not clauses or sections (vocabulary must not depend on requirements).

Transitive coverage: if clause B <ref>s clause A and B has a marker, A
is covered. Coverage follows refs through terms.

Examples:

  Minimal:
    <spec name="simple">
      <clause id="first">A single leaf clause.</clause>
      <clause id="second">Another leaf clause.</clause>
    </spec>

  With definitions and refs:
    <spec name="with_refs">
      <definitions>
        <term text="backend">The storage engine.</term>
      </definitions>
      <clause id="storage">The storage layer uses the <ref>backend</ref>.</clause>
    </spec>

  Nested sections:
    <spec name="nested">
      <definitions>
        <term text="example">A term used in clauses.</term>
      </definitions>
      <section id="1" label="First section">
        <clause id="1.1">A leaf clause referencing <ref>example</ref>.</clause>
        <clause id="1.2">A second leaf referencing <ref>1.1</ref>.</clause>
        <section id="1.3" label="Subsection">
          <clause id="1.3.1">A deeply nested leaf.</clause>
        </section>
      </section>
      <section id="2" label="Second section">
        <clause id="2.1">References <ref>1.2</ref> and <ref>1.3.1</ref>.</clause>
      </section>
    </spec>


THE STATE FILE (.filament)

The .filament file stores three sections:

  [spec]    — current spec clause hashes
  [site]    — per-marker content hashes
  [state]   — per-marker-clause reviewed spec hashes

The spec section stores the current hash of each clause. The site section
stores the hash of the content near each marker. The state section stores
the spec hash that was in effect when each marker was last reviewed against
each clause it references.

The state file is auto-generated. Do not edit it manually.


DRIFT DETECTION

filament detects two kinds of drift:

  SPEC_DRIFT — the spec clause changed since the marker was last reviewed.
    The code/content at the marker's location may no longer match the
    spec's intent. Review the content, then run:
    filament resolve --spec <marker_id>

  SITE_DRIFT — the content near the marker changed since the marker was
    last stored. The spec clause(s) it traces to may no longer describe
    what the content does. Read the spec clause(s), compare against the
    content, then run:
    filament resolve --site <marker_id>

Both drifts are independent. A marker can have neither, one, or both.
When both are drifted, two separate findings are reported.


COMMANDS

  filament check [file-or-dir]...
    Verify that every #F marker is in sync with the spec. Exits 1 if any
    drift, missing, orphan, or malformed marker is found. Use in CI/CD as
    a failure gate. Default is current directory.

  filament status [file-or-dir]...
    Show every marker and its drift state, including OK markers. Detects
    every condition that check detects. Prints a coverage summary. Exits 1
    if any finding is found, 0 otherwise.

  filament init [file-or-dir]...
    Create .filament from the current spec and source markers.

  filament add <clause_id> [clause_id]...
    Print a #F marker line with a new marker id. Paste it into your file
    above the content that covers these clauses.

  filament resolve --spec <marker_id> [marker_id]...
    Clear spec drift for the given marker(s). Use after you've reviewed
    the spec changes and confirmed the content still implements them.

  filament resolve --site <marker_id> [marker_id]...
    Clear site drift for the given marker(s). Use after you've reviewed
    the content changes and confirmed they still match the spec.

  filament sync
    Refresh the [spec] section from the current spec XML. Run this after
    editing the spec, before running 'filament check'.

  filament migrate [file-or-dir]...
    Convert old filament:hash comments to #F markers and generate the
    state file. Run this once when upgrading from the old format.

  filament skill
    Print this guide.


THE SPEC-FIRST PHILOSOPHY

The spec is the control plane. The code, tests, and docs are
implementations of it. If the spec is vague, the implementation is forced
to make decisions that silently become de-facto spec — invisible,
untestable, and irreversible.

filament enforces this by requiring every implementation site to be
traceable to a spec clause via a #F marker. When the spec changes,
every site is flagged for review. When a site changes, the spec clauses
it references are flagged for review. Nothing changes silently.


WORKFLOW: UPDATING A SPEC CLAUSE

  1. Edit the spec XML (change clause prose)
  2. filament sync
  3. filament check — reports SPEC_DRIFT for every marker referencing
     the changed clause(s)
  4. Review each flagged marker's content against the new spec wording
  5. filament resolve --spec <marker_id> for each reviewed site
  6. filament check — should pass


WORKFLOW: CONTENT CHANGED NEAR A MARKER

  1. Edit the file (content near a #F marker changes)
  2. filament check — reports SITE_DRIFT for that marker
  3. Read the spec clause(s) the marker traces to
  4. Compare against the new content
  5. filament resolve --site <marker_id>
  6. filament check — should pass


WORKFLOW: ADDING A NEW MARKER

  1.   filament add tool.name tool.binary
     — prints: # F id:example1 tool.name tool.binary
  2. Paste into the file above the relevant content
  3. filament init (if no state file) or
     filament resolve --site example1


WORKFLOW: CI/CD

  - run: filament check
  - Exit 0 = pass, exit 1 = fail
  - Prose output goes to stderr; CI captures exit code


OPTIONS

  --spec=<path>    Path to spec XML (default: ./filament.spec.xml)
  --quiet          Suppress the tooltip preamble
`
