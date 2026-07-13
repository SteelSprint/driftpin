# Recovery Plan: .filament state from e87a9ee

## Background

During implementation of the `<ref>` element support and related features, I ran
`filament init` destructively 6 times instead of using `filament sync` + `filament
resolve`. This wiped review history every time. The `.filament` file in commit
`e87a9ee` is the last good state before the destructive inits began.

## Why not `filament sync` first?

`filament check` computes spec hashes fresh from the spec XML — it does not need
the `[spec]` section of the state file. So for recovery, the correct first step
is `filament check` directly, NOT `filament sync`. The skill text's "sync then
check" workflow is for when you just edited the spec, not for recovery.

See TODO.md for the skill text workflow gap.

## Steps

1. `git checkout e87a9ee -- .filament` — recover old state file ✅ done
2. `filament check` — see all drift ✅ done
3. Review each finding (parallel agents) ✅ done
4. Resolve 1-way drifts with `filament resolve --spec` or `filament resolve --site`
5. Manually fix 2-way drifts (relocate markers, update docs)
6. Exclude testdata/ from project-level check
7. `filament sync` — bring `[spec]` section up to date
8. `filament check` — should pass
9. Commit recovered + resolved `.filament`

## Verified findings

### 1-way drifts — resolve (20 findings across 17 markers)

| Marker | Type | Clause(s) | Action |
|--------|------|-----------|--------|
| `ttsbgeqq` | SITE_DRIFT | `drift.missing` | `resolve --site` |
| `x2k9m3vf` | NOT_IN_STATE | `drift.transitive_coverage` | `resolve --site` |
| `gegp90b7` | SITE_DRIFT | `output.result_prose` | `resolve --site` |
| `a47bacif` | SITE_DRIFT | `output.tooltip` | `resolve --site` |
| `zmtuvlt0` | SITE_DRIFT | `output.neutral_language` | `resolve --site` |
| `lp8n3vwb` | SITE_DRIFT | `tool.language` | `resolve --site` |
| `b3vm90d1` | SITE_DRIFT | `public_api.subcommands` | `resolve --site` |
| `exag17d2` | SPEC_DRIFT + SITE_DRIFT | `public_api.init` | `resolve --spec` + `resolve --site` |
| `yd9c6bpz` | SPEC_DRIFT | `public_api.check` | `resolve --spec` |
| `n9lv604a` | SPEC_DRIFT | `public_api.status` | `resolve --spec` |
| `f7g8h9ij` | SPEC_DRIFT | `public_api.check` | `resolve --spec` |
| `h1i2j3kl` | SPEC_DRIFT | `drift.missing` | `resolve --spec` |
| `c9d0e1fg` | SPEC_DRIFT ×2 | `public_api.check` + `public_api.status` | `resolve --spec` |
| `pgbp65gt` | SPEC_DRIFT | `public_api.check` | `resolve --spec` |
| `ljsysabe` | SPEC_DRIFT | `hash.input.references` | `resolve --spec` |
| `3aewemki` | SPEC_DRIFT | `hash.input.references` | `resolve --spec` |
| `6uv349nx` | SITE_DRIFT | `public_api.skill` | `resolve --site` |
| `5k4j8r5y` | NOT_IN_STATE | `doctor.subcommand` + 3 others | `resolve --site` |

### 2-way drifts — manual fix needed (7 markers)

| Marker | Clause(s) | File | Problem | Fix |
|--------|-----------|------|---------|-----|
| `xr2m4kqt` | `tool.location` | main.go:10 | Content window has no path/location reference | Relocate to where `/filament` path is established |
| `qm5t9xkj` | `tool.design` | main.go:12 | Imports/dependency requirements not in content window | Relocate near `import (...)` block |
| `ws7j2yhv` | `tool.binary` | main.go:13 | No binary-name reference in content window | Relocate to build/go.mod config |
| `8tf919gk` | `versioning.source` | main.go:14 | Clause is about spec versioning, not code | Relocate (or remove if no code site exists) |
| `cdi0ftqy` | `versioning.amendments` | main.go:15 | Clause is about PR process, not code | Relocate (or remove if no code site exists) |
| `ocwydxem` | `self_hosting.test` | main.go:16 | Content is not a test | Relocate to `_test.go` file |
| `t2s5fcx8` | `public_api.status` | documentation/public-api.md:31 | Doc says "Always exits 0" contradicting new spec | Update doc: exit 1 if findings, add coverage summary |
| `si1coe00` | `public_api.init` | documentation/public-api.md:41 | Doc omits existing-state-file safety check | Update doc: add error-if-exists description |

### testdata/ fixtures — exclude from check (9 findings: 3 ORPHAN + 6 DUPLICATE_MARKER)

All 9 findings come from intentional test fixtures in `testdata/`. Each fixture
(`fixture_new_valid.go`, `fixture_new_spec_drift.go`, `fixture_new_site_drift.go`)
is a self-contained test case driven by `cli_test.go` with its own spec
(`golden.spec.xml`) and state file.

- The `x`/`y`/`z` clauses and `aaaa1111`/`bbbb2222`/`cccc3333` markers are valid
  within each fixture's own context
- They only appear as ORPHAN/DUPLICATE when `filament check` scans `testdata/`
  against the project-level spec
- **Do NOT fix the fixtures** — that would break the test suite
- **Solution:** Exclude `testdata/` from the project-level check (see discussion
  below)

## Summary

| Category | Count | Action |
|----------|-------|--------|
| 1-way (resolve) | 20 findings / 17 markers | `resolve --spec` or `resolve --site` |
| 2-way (manual fix) | 8 findings / 7 markers | Relocate markers or update docs |
| Testdata exclusions | 9 findings | Exclude `testdata/` from check |

## Post-recovery: best_practices.multiple_markers

During recovery, we added a new spec clause and associated markers to document
that multiple markers per clause are expected and encouraged.

### Spec clause added

```xml
<section id="best_practices" label="Best practices">
  <clause id="best_practices.multiple_markers">
    A clause MAY have multiple markers across different files or locations
    in the same file. Multiple markers per clause are expected and
    encouraged: every location where a clause is implemented should have
    its own marker. This provides granular drift detection — when a spec
    clause changes, every implementation site is independently flagged for
    review, and when a site changes, only that specific site is flagged.
  </clause>
</section>
```

### Markers added

| Marker | File | Site |
|--------|------|------|
| `m4rkr001` | documentation/marker-format.md | New "Multiple markers per clause" section |
| `m4rkr002` | comment.go | Tooltip (now mentions multiple markers) |

### Skill text updated

The skill text (skill.go) now includes a paragraph in THE MARKER FORMAT
section explaining that multiple markers per clause are expected and
encouraged, with the rationale (granular drift detection).

### Tooltip updated

The Tooltip (comment.go) now includes a brief mention: "A clause may have
multiple markers — one at every site where it's implemented."

### Actions needed during recovery

1. `filament sync` — add `best_practices.multiple_markers` to `[spec]` section
2. `filament resolve --site m4rkr001 m4rkr002` — register both markers
3. `filament check` — verify
