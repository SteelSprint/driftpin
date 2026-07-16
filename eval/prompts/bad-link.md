## Your environment

Your workspace contains a working Go project with three string utility functions and driftpin set up. All specs, markers, and links exist, and `drift todo` reports "No changes detected." — but one of the marker-to-spec links is WRONG. A marker is linked to the incorrect spec. The link is consistent (just wrong), so `drift todo` doesn't flag it.

## Your task

Inspect the driftpin links to find the incorrect one. Fix it by removing the wrong link and creating the correct one. After fixing, `drift todo` should still report "No changes detected."

## Success criteria

1. Used `drift list` (or equivalent inspection) to examine links
2. Identified the incorrect link (which marker is linked to the wrong spec)
3. Used `drift unlink` to remove the incorrect link
4. Used `drift link` to create the correct link
5. `drift todo` reports "No changes detected." after the fix
6. All links are now correct (each marker linked to its matching spec)
