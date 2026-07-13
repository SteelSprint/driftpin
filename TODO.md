# TODO

## Skill text workflow gap: recovery vs. editing

The WORKFLOW: UPDATING A SPEC CLAUSE says `filament sync` then `filament check`. This is correct when you just edited the spec — `sync` refreshes the spec section, then `check` detects drift against the new hashes.

But for **recovery** (restoring an old `.filament` from git), `sync` is wrong as a first step. `check` computes spec hashes fresh from the spec XML — it doesn't need the `[spec]` section to be current. The correct recovery workflow is:

1. `filament check` — see all drift (SPEC_DRIFT, SITE_DRIFT, MISSING, NOT_IN_STATE)
2. Review and resolve each finding
3. `filament sync` — only after resolving spec drift

The skill text should document this distinction. A future WORKFLOW: RECOVERING STATE section is needed.

**Status:** pending
**Blocked by:** .filament recovery from e87a9ee (in progress)
