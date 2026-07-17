## Your environment

Your workspace contains a working Go supply chain management system with drift already set up and clean. The project has 15 spec modules across inventory, storage, validation, supplier, order, shipping, notify, report, cli, auth, audit, config, errors, and search. Markers are placed in the Go source files (all in package main), all linked to their specs, and `drift todo` reports clean. The project compiles with `go build`.

## Your task

Add Category and Priority to inventory items:
- Categories: "electronics", "food", "clothing", "tools", "other" (default: "other")
- Priorities: "critical", "normal" (default: "normal"), "low"

Changes needed:
1. Add Category and Priority fields to the Item struct
2. Change internal storage from a flat slice to a map[Category][]Item
3. Modify ListItems to group by category, sorted by priority within each group
4. Update validation to reject unknown categories and priorities
5. Update serialization to include category and priority fields
6. Update the CLI add command to accept --category and --priority flags
7. Update all reports to include per-category breakdowns
8. Update shipping to vary rates by category (electronics: +15%, food: +25%, others: flat)
9. Update notifications to include category in alert text

After making your changes, use drift to detect and resolve any drift.

## Success criteria

1. Items have Category and Priority fields with correct defaults
2. Storage uses map[Category][]Item internally
3. ListItems groups by category, priority-sorted within each group
4. Validation rejects unknown categories and priorities
5. Serialization includes category and priority
6. CLI accepts --category and --priority flags
7. Reports include per-category breakdowns
8. Shipping rates vary by category
9. `drift todo` reports "No changes detected." after resolution
10. Code compiles with `go build`
11. All specs accurately describe the final code behavior
