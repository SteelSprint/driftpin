## Your environment

Your workspace contains a working Go calculator with driftpin already set up: spec file (main.pin.xml), markers in the code, all links, and a clean drift.pin. The project compiles with `go build`.

## Your task

The calculator's `div` function returns an error only for division by zero. Modify it to also return an error when the divisor is negative zero (i.e., `b == 0 && math.Signbit(b)`). Do NOT change any other function.

After modifying the code, use driftpin to detect the drift and resolve it once you've verified the code still matches the spec.

## Success criteria

1. The `div` function returns an error for negative zero division (in addition to regular zero)
2. `drift todo` shows drift on the div marker after the code change
3. `drift reset` was used to resolve the drift
4. `drift todo` reports "No changes detected." after resolution
5. No other markers or specs were affected or changed
