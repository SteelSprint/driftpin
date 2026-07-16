## Your environment

Your workspace contains a working Go temperature converter with driftpin set up and clean. The `Convert` function does all work — converting to Celsius then from Celsius — in a single function body. The project compiles with `go build`.

## Your task

Refactor the `Convert` function by extracting the "to Celsius" and "from Celsius" logic into separate helper functions (`toCelsius` and `fromCelsius`). The behavior must remain exactly the same — this is a purely cosmetic refactoring. Do NOT change the specs.

After refactoring, use driftpin to detect the drift and resolve it after verifying the behavior is unchanged.

## Success criteria

1. Helper functions extracted (toCelsius and fromCelsius, or equivalent named helpers)
2. Behavior is unchanged (same inputs produce same outputs — verify by running the program)
3. `drift todo` shows drift on the convert marker after refactoring
4. `drift reset` was used to resolve the drift
5. `drift todo` reports "No changes detected." after resolution
6. Code compiles with `go build`
