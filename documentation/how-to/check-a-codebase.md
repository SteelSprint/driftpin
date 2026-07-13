# Check a codebase

<!-- #F id:f7g8h9ij public_api.check public_api.file_walk -->

## Verify all markers are in sync

```
filament check
```

This validates the spec, loads the state file, scans all text files in the current directory (skipping any files matching exclude patterns in the state file), and reports any drift, missing clauses, orphans, or malformed markers.

## Check specific files or directories

```
filament check src/ docs/ README.md
```

Only the given paths are scanned. Non-text files are skipped silently.

## Use in CI

Add to your CI pipeline:

```yaml
- name: Check filament drift
  run: filament check
```

The check exits 0 if all markers are in sync, 1 otherwise. Prose output goes to stderr.

## Suppress the tooltip

```
filament check --quiet
```

The tooltip preamble is suppressed. Per-finding prose is still printed.

## Override the spec path

```
filament check --spec=path/to/spec.xml src/
```

Default is `./filament.spec.xml`.
