# Marker format

<!-- #F id:p3q4r5st marker_format.syntax marker_format.id_format marker_format.comment_prefix -->
<!-- #F id:ouwak1e5 marker_format.content_normalization -->
<!-- #F id:e080ciju marker_format.malformed -->

A filament marker is a line in any text file that traces a site to one or more spec clauses. The tool matches the directive as a substring, regardless of the comment character that precedes it. This allows markers to work in any text file with any comment style.

## The syntax

```
#F id:<marker_id> <clause_id> <clause_id> ... [-- <flag>=<value> ...]
```

A marker contains:
- The directive `#F` — matched as a literal substring
- Whitespace
- The literal `id:` followed by an 8-character marker id
- Whitespace
- One or more whitespace-separated clause ids (dotted-path identifiers from the spec)
- An optional `--` separator followed by key=value flags (reserved for future use)

## The regex

Filament matches lines against this regular expression:

```
#F\s+id:([a-z0-9]{8})\s+(.+)
```

A line matches if it contains `#F`, followed by whitespace, then `id:` followed by exactly 8 lowercase alphanumeric characters, then whitespace, then the clause ids.

## Marker ids

<!-- #F id:4e21jq26 marker_format.id_generation -->

A marker id is an 8-character identifier composed of lowercase letters (a-z) and digits (0-9). Marker ids are generated using a cryptographically secure random source. Each marker id must be unique within the workspace.

Generate a new marker id with:

```
filament add <clause_id> [clause_id]...
```

## In Go

```go
// # F id:example1 tool.name tool.binary
func main() { ... }
```

## In Python

```python
# # F id:example2 tool.name tool.binary
def run_check():
    ...
```

## In SQL

```sql
-- # F id:example3 tool.name tool.binary
```

## In HTML or Markdown

```html
<!-- # F id:example4 tool.name tool.binary -->
```

## In a plain text file

A line with no comment prefix at all:

```
# F id:example5 tool.name tool.binary
```

The tool matches the directive, not the comment prefix. A line containing `#F id:` followed by a valid marker id and clause ids is a marker regardless of what precedes it on the line.

## Multiple clause ids on one marker

A single marker can reference multiple clauses. This is useful when the content near the marker covers several spec clauses:

```go
// # F id:example6 tool.name tool.binary tool.language
func main() { ... }
```

Each clause id is tracked independently for drift detection.

## Multiple markers per clause

<!-- #F id:m4rkr001 best_practices.multiple_markers -->

A clause MAY have multiple markers across different files or within the same file. Every location where a clause is implemented should have its own marker. This is expected and encouraged — markers grow as you trace more implementation sites.

Multiple markers give you granular drift detection:

- When a **spec clause changes**, every marker referencing it is independently flagged for SPEC_DRIFT. You review each site to confirm it still matches the new spec wording.
- When a **site changes**, only that specific marker is flagged for SITE_DRIFT. Other markers for the same clause are unaffected.

```
  spec clause: tool.binary

  install.sh:9     #F id:abc12345 tool.binary    ← BINARY="filament"
  main.go:33       #F id:def67890 tool.binary    ← const usage = "filament ..."
```

Without multiple markers, a change to `install.sh` would be invisible if the only marker were in `main.go`. With multiple markers, each implementation site is independently tracked.

## Comment-prefix agnosticism

The tool matches `#F` as a substring. It does not parse or validate the comment prefix. This means markers work in any language that uses any comment style — `//`, `#`, `--`, `<!-- -->`, `/* */`, `%`, `;`, `"`, or any other prefix.
