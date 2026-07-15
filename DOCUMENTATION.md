# Documentation

## Mental model

When specs and their implementations change, `drift` informs your LLM. This gives your LLM a chance to fix any drifts as they happen. `drift` is packaged as a CLI that is intended mainly for LLMs to use.

## How it works

`drift` asks you to save your specs in `*.pin.xml` files. Each file contains `<spec id="...">` elements that describe individual spec terms. `drift` then scans your code files for `#F <shortcode>` markers — short unique IDs placed in comments above the code that implements each spec term. Specs and markers form a many-to-many graph — a single spec term can be enforced by several markers, and a single marker can refer to several spec terms. Each link between a spec term and a marker is called an **edge**. When a change occurs on either side of an edge, `drift todo` surfaces one todo item per affected edge, with filepaths and line numbers where your LLM should check for drifts in specification.

`drift` hashes spec terms as well as markers (SHA1 of content), and saves those hashes inside `drift.pin`, which should be committed to git. This is an XML file that contains the hashes of specs and markers, the links between them, and a temporary resolution state area for partial todo-list resolutions. The algorithm manages this file itself — the user should refrain from touching `drift.pin` manually.

### Spec files

Specs are defined in `*.pin.xml` files anywhere in the project:

```xml
<specs>
  <spec id="validate_input">input must be validated before processing</spec>
</specs>
```

The scanner walks the project directory, parses each `*.pin.xml` file, extracts `<spec>` elements by their `id` attribute, and SHA1-hashes the element's inner content.

### Markers

Markers are `#F <shortcode>` comment lines in code files (`.go`, `.py`, `.js`, `.ts`, `.java`, `.c`, `.rs`, etc.):

```go
// #F 4hy7fh3h
func handleRequest() {
    validateInput()
}
```

The scanner finds these lines, records the shortcode, filepath, and line number, and SHA1-hashes the 10 lines following the marker. (The content window will be configurable per-marker in future versions.)

### Links

Markers and specs have separate IDs — a marker's shortcode does not match a spec's ID. Links between them are declared in `drift.pin` via the CLI:

```bash
$ drift link 4hy7fh3h:validate_input
```

This validates that both the marker and spec exist, then persists the link. Links can be many-to-many: one marker can link to multiple specs, and one spec can link to multiple markers.

## CLI commands

| Command | Description |
|---|---|
| `drift init` | Creates an empty `drift.pin` in the project root. Required before other commands. |
| `drift todo` | Scans the filesystem, reconciles with `drift.pin`, and surfaces any drift as a todo list. Does not modify `drift.pin`. |
| `drift reset <marker>:<spec>` | Marks a specific edge as resolved. Saves updated state to `drift.pin`. If all edges for a node are resolved, baselines collapse automatically. |
| `drift link <marker>:<spec>` | Declares a link between a marker and a spec term. Validates both exist and the link isn't a duplicate. Saves specs, markers, and the new link to `drift.pin`. |

## Reconciliation

When `drift todo` or `drift reset` runs, the orchestrator:

1. Loads `drift.pin` (baseline hashes, links, resolution state)
2. Scans the filesystem (current specs, markers, and their hashes)
3. **Reconciles** — for each discovered spec/marker:
   - If already in `drift.pin` → keeps the baseline hash from the pin, updates filepath/line if changed
   - If new (not in pin) → baseline = current hash (no drift on first discovery)
   - If in pin but not found on disk → error
4. Builds the scan and runs the core algorithm

This means the first `drift todo` after adding spec files or code markers discovers them and sets their baselines. On subsequent runs, changes are detected by comparing current hashes against these baselines.

## Example

Let's say you have a marker `4hy7fh3h` and a spec file that has `validate_input`.

First, initialize and discover:

```bash
$ drift init
Initialized drift.pin

$ drift todo

No changes detected.
```

The scanner discovers the spec and marker. Since they're new, baselines are set to current hashes — no drift. The `drift.pin` at this point contains:

```xml
<drift>
  <specs>
    <spec id="validate_input" hash="S98YH3T2T32..." filepath="main.pin.xml" line="1"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="JHIO34YU..." filepath="src/main.go" line="15"/>
  </markers>
  <links>
  </links>
  <resolutions>
  </resolutions>
</drift>
```

Note: no links yet. Link the marker to the spec:

```bash
$ drift link 4hy7fh3h:validate_input
Linked marker "4hy7fh3h" to spec "validate_input"
```

The `drift.pin` now includes the link:

```xml
<drift>
  <specs>
    <spec id="validate_input" hash="S98YH3T2T32..." filepath="main.pin.xml" line="1"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="JHIO34YU..." filepath="src/main.go" line="15"/>
  </markers>
  <links>
    <link specId="validate_input" markerId="4hy7fh3h"/>
  </links>
  <resolutions>
  </resolutions>
</drift>
```

Let's say you modify some code:

```bash
$ drift todo

1 marker has unchecked changes.

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "validate_input" in "/workspaces/my-project/main.pin.xml:1". The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary. Once you are satisfied, run `drift reset 4hy7fh3h:validate_input` to mark this todo item as complete.
```

At this point `drift.pin` is still unchanged — `drift todo` doesn't modify the file.

Then you mark the edge as resolved:

```bash
$ drift reset 4hy7fh3h:validate_input
```

Since the marker has no more unchecked specs, and the spec has no more unchecked markers, the baselines collapse — `drift.pin` is updated with the new hashes:

```xml
<drift>
  <specs>
    <spec id="validate_input" hash="FGHJKNE..." filepath="main.pin.xml" line="1"/>
  </specs>
  <markers>
    <marker id="4hy7fh3h" hash="0HGO24G4..." filepath="src/main.go" line="15"/>
  </markers>
  <links>
    <link specId="validate_input" markerId="4hy7fh3h"/>
  </links>
  <resolutions>
  </resolutions>
</drift>
```

Let's say you modify a spec:

```bash
$ drift todo

1 spec item has unchecked changes.

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "validate_input" in "/workspaces/my-project/main.pin.xml:1". The spec term has changed but not the marker. Please check whether the new version of the spec term is still reflected in the code and make any modifications necessary. Once you are satisfied, run `drift reset 4hy7fh3h:validate_input` to mark this todo item as complete.
```

Now let's say you modify both a spec term and its related marker around the same time:

```bash
$ drift todo

1 marker has unchecked changes.
1 spec item has unchecked changes.

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "validate_input" in "/workspaces/my-project/main.pin.xml:1". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset 4hy7fh3h:validate_input` to mark this todo item as complete.
```

Still no modifications to `drift.pin` here.

## Many-to-many relationships

Specs and markers form a many-to-many graph. A single spec term can be enforced by several markers (e.g. the same rule applied in multiple places), and a single marker can refer to several spec terms (e.g. one block of code that satisfies multiple requirements). Because todos are edge-based, the number of todo items is the product of changed specs and their related markers. Each example below includes an illustrative matrix (not CLI output — purely a visual aid) that shows edges at a glance: `●` = unchecked edge, `✓` = resolved edge, blank = no edge.

### One spec term, many markers

A spec term like `auth_token_expiry` is often enforced in more than one place — say a middleware layer and a login handler. When the spec changes, every edge connecting it to a related marker produces its own todo item. Here there is 1 changed spec term and 2 related markers, so 1 × 2 = 2 todo items.

```
                    ┌──────────────────────────────────┐
                    │             Markers              │
┌───────────────────┼───────────┬──────────────────────┤
│   Spec terms      │ a1b2c3d4  │ e5f6g7h8             │
├───────────────────┼───────────┼──────────────────────┤
│ auth_token_expiry │     ●     │     ●                │
└───────────────────┴───────────┴──────────────────────┘
```

```bash
$ drift todo

1 spec item has unchecked changes.

1. [TODO] Edge between marker "a1b2c3d4" in "/workspaces/my-project/src/middleware/auth.go:42" and spec term "auth_token_expiry" in "/workspaces/my-project/specs/auth.pin.xml:24". The spec term has changed but not the marker. Please check whether the new version of the spec term is still reflected in the code and make any modifications necessary. Once you are satisfied, run `drift reset a1b2c3d4:auth_token_expiry` to mark this todo item as complete.

2. [TODO] Edge between marker "e5f6g7h8" in "/workspaces/my-project/src/api/handlers/login.go:88" and spec term "auth_token_expiry" in "/workspaces/my-project/specs/auth.pin.xml:24". The spec term has changed but not the marker. Please check whether the new version of the spec term is still reflected in the code and make any modifications necessary. Once you are satisfied, run `drift reset e5f6g7h8:auth_token_expiry` to mark this todo item as complete.
```

### One marker, many spec terms

Conversely, a single block of code can satisfy more than one spec term at once. For example, an upload handler might enforce both `validate_file_size` and `scan_for_malware`. When that marker changes, every edge connecting it to a related spec term produces its own todo item. Here there is 1 changed marker and 2 related spec terms, so 1 × 2 = 2 todo items.

```
                    ┌──────────────────┐
                    │     Markers      │
┌───────────────────┼──────────────────┤
│   Spec terms      │ k9l0m1n2         │
├───────────────────┼──────────────────┤
│ validate_file_size│     ●            │
│ scan_for_malware  │     ●            │
└───────────────────┴──────────────────┘
```

```bash
$ drift todo

1 marker has unchecked changes.

1. [TODO] Edge between marker "k9l0m1n2" in "/workspaces/my-project/src/api/handlers/upload.go:115" and spec term "validate_file_size" in "/workspaces/my-project/specs/uploads.pin.xml:12". The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary. Once you are satisfied, run `drift reset k9l0m1n2:validate_file_size` to mark this todo item as complete.

2. [TODO] Edge between marker "k9l0m1n2" in "/workspaces/my-project/src/api/handlers/upload.go:115" and spec term "scan_for_malware" in "/workspaces/my-project/specs/uploads.pin.xml:48". The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary. Once you are satisfied, run `drift reset k9l0m1n2:scan_for_malware` to mark this todo item as complete.
```

### Many markers, many spec terms

When a refactor touches several files where multiple spec terms are in play, the graph can fan out in both directions. Here two spec terms (`rate_limit_per_user` and `log_rate_limit_hits`) are each enforced by two markers (`p3q4r5s6` in the middleware and `t7u8v9w0` in the request handler), and both specs and both markers have changed in the same pass. There are 2 changed markers × 2 changed spec terms = 4 affected edges, so 4 todo items.

```
                    ┌──────────────────────────────────┐
                    │             Markers              │
┌───────────────────┼───────────┬─────────────────────┤
│   Spec terms      │ p3q4r5s6  │ t7u8v9w0            │
├───────────────────┼───────────┼─────────────────────┤
│ rate_limit_per_user│     ●     │     ●               │
│ log_rate_limit_hits│     ●     │     ●               │
└───────────────────┴───────────┴─────────────────────┘
```

```bash
$ drift todo

2 markers have unchecked changes.
2 spec items have unchecked changes.

1. [TODO] Edge between marker "p3q4r5s6" in "/workspaces/my-project/src/middleware/ratelimit.go:55" and spec term "rate_limit_per_user" in "/workspaces/my-project/specs/api.pin.xml:23". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset p3q4r5s6:rate_limit_per_user` to mark this todo item as complete.

2. [TODO] Edge between marker "p3q4r5s6" in "/workspaces/my-project/src/middleware/ratelimit.go:55" and spec term "log_rate_limit_hits" in "/workspaces/my-project/specs/api.pin.xml:67". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset p3q4r5s6:log_rate_limit_hits` to mark this todo item as complete.

3. [TODO] Edge between marker "t7u8v9w0" in "/workspaces/my-project/src/api/handlers/request.go:201" and spec term "rate_limit_per_user" in "/workspaces/my-project/specs/api.pin.xml:23". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset t7u8v9w0:rate_limit_per_user` to mark this todo item as complete.

4. [TODO] Edge between marker "t7u8v9w0" in "/workspaces/my-project/src/api/handlers/request.go:201" and spec term "log_rate_limit_hits" in "/workspaces/my-project/specs/api.pin.xml:67". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset t7u8v9w0:log_rate_limit_hits` to mark this todo item as complete.
```

### Many markers, many spec terms, at scale

When the graph grows further, the matrix becomes especially useful for tracking progress across a large surface area. Consider a payments system where three spec terms (`validate_amount`, `check_fraud_rules`, and `log_transaction`) are each enforced by three markers (the card, bank transfer, and wallet handlers). All three spec terms and all three markers change in the same pass, producing 3 × 3 = 9 affected edges.

```
                    ┌──────────────────────────────────────────────┐
                    │                   Markers                     │
┌───────────────────┼───────────┬───────────┬─────────────────────┤
│   Spec terms      │ a1b2c3d4  │ e5f6g7h8  │ f9g0h1i2            │
├───────────────────┼───────────┼───────────┼─────────────────────┤
│ validate_amount   │     ●     │     ●     │     ●               │
│ check_fraud_rules │     ●     │     ●     │     ●               │
│ log_transaction   │     ●     │     ●     │     ●               │
└───────────────────┴───────────┴───────────┴─────────────────────┘

9 edges remaining
```

```bash
$ drift todo

3 markers have unchecked changes.
3 spec items have unchecked changes.

1. [TODO] Edge between marker "a1b2c3d4" in "/workspaces/my-project/src/api/handlers/card.go:52" and spec term "validate_amount" in "/workspaces/my-project/specs/payments.pin.xml:18". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset a1b2c3d4:validate_amount` to mark this todo item as complete.

2. [TODO] Edge between marker "a1b2c3d4" in "/workspaces/my-project/src/api/handlers/card.go:52" and spec term "check_fraud_rules" in "/workspaces/my-project/specs/payments.pin.xml:45". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset a1b2c3d4:check_fraud_rules` to mark this todo item as complete.

3. [TODO] Edge between marker "a1b2c3d4" in "/workspaces/my-project/src/api/handlers/card.go:52" and spec term "log_transaction" in "/workspaces/my-project/specs/payments.pin.xml:71". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset a1b2c3d4:log_transaction` to mark this todo item as complete.

4. [TODO] Edge between marker "e5f6g7h8" in "/workspaces/my-project/src/api/handlers/bank_transfer.go:67" and spec term "validate_amount" in "/workspaces/my-project/specs/payments.pin.xml:18". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset e5f6g7h8:validate_amount` to mark this todo item as complete.

5. [TODO] Edge between marker "e5f6g7h8" in "/workspaces/my-project/src/api/handlers/bank_transfer.go:67" and spec term "check_fraud_rules" in "/workspaces/my-project/specs/payments.pin.xml:45". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset e5f6g7h8:check_fraud_rules` to mark this todo item as complete.

6. [TODO] Edge between marker "e5f6g7h8" in "/workspaces/my-project/src/api/handlers/bank_transfer.go:67" and spec term "log_transaction" in "/workspaces/my-project/specs/payments.pin.xml:71". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset e5f6g7h8:log_transaction` to mark this todo item as complete.

7. [TODO] Edge between marker "f9g0h1i2" in "/workspaces/my-project/src/api/handlers/wallet.go:43" and spec term "validate_amount" in "/workspaces/my-project/specs/payments.pin.xml:18". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset f9g0h1i2:validate_amount` to mark this todo item as complete.

8. [TODO] Edge between marker "f9g0h1i2" in "/workspaces/my-project/src/api/handlers/wallet.go:43" and spec term "check_fraud_rules" in "/workspaces/my-project/specs/payments.pin.xml:45". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset f9g0h1i2:check_fraud_rules` to mark this todo item as complete.

9. [TODO] Edge between marker "f9g0h1i2" in "/workspaces/my-project/src/api/handlers/wallet.go:43" and spec term "log_transaction" in "/workspaces/my-project/specs/payments.pin.xml:71". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset f9g0h1i2:log_transaction` to mark this todo item as complete.
```

Let's say you resolve two edges — the card handler's `validate_amount` and the bank transfer handler's `check_fraud_rules`:

```bash
$ drift reset a1b2c3d4:validate_amount
$ drift reset e5f6g7h8:check_fraud_rules
```

The matrix now shows progress at a glance:

```
                    ┌──────────────────────────────────────────────┐
                    │                   Markers                     │
┌───────────────────┼───────────┬───────────┬─────────────────────┤
│   Spec terms      │ a1b2c3d4  │ e5f6g7h8  │ f9g0h1i2            │
├───────────────────┼───────────┼───────────┼─────────────────────┤
│ validate_amount   │     ✓     │     ●     │     ●               │
│ check_fraud_rules │     ●     │     ✓     │     ●               │
│ log_transaction   │     ●     │     ●     │     ●               │
└───────────────────┴───────────┴───────────┴─────────────────────┘

7 edges remaining
```

```bash
$ drift todo

3 markers have unchecked changes.
3 spec items have unchecked changes.
7 edges remaining.

1. [TODO] Edge between marker "a1b2c3d4" in "/workspaces/my-project/src/api/handlers/card.go:52" and spec term "check_fraud_rules" in "/workspaces/my-project/specs/payments.pin.xml:45". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset a1b2c3d4:check_fraud_rules` to mark this todo item as complete.

2. [TODO] Edge between marker "a1b2c3d4" in "/workspaces/my-project/src/api/handlers/card.go:52" and spec term "log_transaction" in "/workspaces/my-project/specs/payments.pin.xml:71". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset a1b2c3d4:log_transaction` to mark this todo item as complete.

3. [TODO] Edge between marker "e5f6g7h8" in "/workspaces/my-project/src/api/handlers/bank_transfer.go:67" and spec term "validate_amount" in "/workspaces/my-project/specs/payments.pin.xml:18". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset e5f6g7h8:validate_amount` to mark this todo item as complete.

4. [TODO] Edge between marker "e5f6g7h8" in "/workspaces/my-project/src/api/handlers/bank_transfer.go:67" and spec term "log_transaction" in "/workspaces/my-project/specs/payments.pin.xml:71". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset e5f6g7h8:log_transaction` to mark this todo item as complete.

5. [TODO] Edge between marker "f9g0h1i2" in "/workspaces/my-project/src/api/handlers/wallet.go:43" and spec term "validate_amount" in "/workspaces/my-project/specs/payments.pin.xml:18". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset f9g0h1i2:validate_amount` to mark this todo item as complete.

6. [TODO] Edge between marker "f9g0h1i2" in "/workspaces/my-project/src/api/handlers/wallet.go:43" and spec term "check_fraud_rules" in "/workspaces/my-project/specs/payments.pin.xml:45". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset f9g0h1i2:check_fraud_rules` to mark this todo item as complete.

7. [TODO] Edge between marker "f9g0h1i2" in "/workspaces/my-project/src/api/handlers/wallet.go:43" and spec term "log_transaction" in "/workspaces/my-project/specs/payments.pin.xml:71". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset f9g0h1i2:log_transaction` to mark this todo item as complete.
```

## Drift.pin walkthrough

Let's trace the `drift.pin` file through the 3×3 example above, starting from the point where all 9 edges have been discovered and linked, but before any drift occurs.

### Clean state (no drift)

After `drift init`, `drift todo` (discovers specs/markers), and `drift link` for all 9 edges, the `drift.pin` looks like this — baselines match current content, no resolution entries:

```xml
<drift>
  <specs>
    <spec id="validate_amount" hash="baseline_3a" filepath="specs/payments.pin.xml" line="18"/>
    <spec id="check_fraud_rules" hash="baseline_3b" filepath="specs/payments.pin.xml" line="45"/>
    <spec id="log_transaction" hash="baseline_3c" filepath="specs/payments.pin.xml" line="71"/>
  </specs>
  <markers>
    <marker id="a1b2c3d4" hash="baseline_1a" filepath="src/api/handlers/card.go" line="52"/>
    <marker id="e5f6g7h8" hash="baseline_2a" filepath="src/api/handlers/bank_transfer.go" line="67"/>
    <marker id="f9g0h1i2" hash="baseline_3a" filepath="src/api/handlers/wallet.go" line="43"/>
  </markers>
  <links>
    <link specId="validate_amount" markerId="a1b2c3d4"/>
    <link specId="check_fraud_rules" markerId="a1b2c3d4"/>
    <link specId="log_transaction" markerId="a1b2c3d4"/>
    <link specId="validate_amount" markerId="e5f6g7h8"/>
    <link specId="check_fraud_rules" markerId="e5f6g7h8"/>
    <link specId="log_transaction" markerId="e5f6g7h8"/>
    <link specId="validate_amount" markerId="f9g0h1i2"/>
    <link specId="check_fraud_rules" markerId="f9g0h1i2"/>
    <link specId="log_transaction" markerId="f9g0h1i2"/>
  </links>
  <resolutions>
  </resolutions>
</drift>
```

### After drift detected (all 9 edges drifted)

When all specs and markers change, `drift todo` surfaces 9 edges. But `drift.pin` is **not modified** by `drift todo` — the baselines stay as-is. Drift is detected by comparing current content hashes against the baselines in the file.

### After resolving 2 of 9 edges

```bash
$ drift reset a1b2c3d4:validate_amount
$ drift reset e5f6g7h8:check_fraud_rules
```

Since not all edges for any single node are resolved, **no collapse occurs**. The baselines stay unchanged, and 2 resolution entries are added to track the reviewed edges:

```xml
<drift>
  <specs>
    <spec id="validate_amount" hash="baseline_3a" filepath="specs/payments.pin.xml" line="18"/>
    <spec id="check_fraud_rules" hash="baseline_3b" filepath="specs/payments.pin.xml" line="45"/>
    <spec id="log_transaction" hash="baseline_3c" filepath="specs/payments.pin.xml" line="71"/>
  </specs>
  <markers>
    <marker id="a1b2c3d4" hash="baseline_1a" filepath="src/api/handlers/card.go" line="52"/>
    <marker id="e5f6g7h8" hash="baseline_2a" filepath="src/api/handlers/bank_transfer.go" line="67"/>
    <marker id="f9g0h1i2" hash="baseline_3a" filepath="src/api/handlers/wallet.go" line="43"/>
  </markers>
  <links>
    <link specId="validate_amount" markerId="a1b2c3d4"/>
    <link specId="check_fraud_rules" markerId="a1b2c3d4"/>
    <link specId="log_transaction" markerId="a1b2c3d4"/>
    <link specId="validate_amount" markerId="e5f6g7h8"/>
    <link specId="check_fraud_rules" markerId="e5f6g7h8"/>
    <link specId="log_transaction" markerId="e5f6g7h8"/>
    <link specId="validate_amount" markerId="f9g0h1i2"/>
    <link specId="check_fraud_rules" markerId="f9g0h1i2"/>
    <link specId="log_transaction" markerId="f9g0h1i2"/>
  </links>
  <resolutions>
    <resolution specId="validate_amount" markerId="a1b2c3d4" currentSpecHash="current_3a" currentMarkerHash="current_1a"/>
    <resolution specId="check_fraud_rules" markerId="e5f6g7h8" currentSpecHash="current_3b" currentMarkerHash="current_2a"/>
  </resolutions>
</drift>
```

**Why:**
- `[specs]` and `[markers]` still hold **baseline** hashes (the last known-good state). They haven't been updated because collapse only happens when **all** edges for a node are resolved.
- `[links]` defines the graph topology — unchanged.
- `[resolutions]` records the 2 edges that were reviewed. Each entry stores `currentSpecHash` and `currentMarkerHash` — the content hashes at the time of review. If either side changes again after this, the stored values no longer match current content, and the edge re-drifts.
- 7 edges still have mismatches between baseline and current hashes, so they remain in `drift todo`.

### After resolving all 9 edges (full collapse)

When the last edge is resolved, all nodes have all edges checked. The collapse algorithm runs to fixpoint: baselines are updated to current hashes, and all resolution entries are pruned:

```xml
<drift>
  <specs>
    <spec id="validate_amount" hash="current_3a" filepath="specs/payments.pin.xml" line="18"/>
    <spec id="check_fraud_rules" hash="current_3b" filepath="specs/payments.pin.xml" line="45"/>
    <spec id="log_transaction" hash="current_3c" filepath="specs/payments.pin.xml" line="71"/>
  </specs>
  <markers>
    <marker id="a1b2c3d4" hash="current_1a" filepath="src/api/handlers/card.go" line="52"/>
    <marker id="e5f6g7h8" hash="current_2a" filepath="src/api/handlers/bank_transfer.go" line="67"/>
    <marker id="f9g0h1i2" hash="current_3a" filepath="src/api/handlers/wallet.go" line="43"/>
  </markers>
  <links>
    <link specId="validate_amount" markerId="a1b2c3d4"/>
    <link specId="check_fraud_rules" markerId="a1b2c3d4"/>
    <link specId="log_transaction" markerId="a1b2c3d4"/>
    <link specId="validate_amount" markerId="e5f6g7h8"/>
    <link specId="check_fraud_rules" markerId="e5f6g7h8"/>
    <link specId="log_transaction" markerId="e5f6g7h8"/>
    <link specId="validate_amount" markerId="f9g0h1i2"/>
    <link specId="check_fraud_rules" markerId="f9g0h1i2"/>
    <link specId="log_transaction" markerId="f9g0h1i2"/>
  </links>
  <resolutions>
  </resolutions>
</drift>
```

Back to clean state — baselines match current content, no resolution entries. The next `drift todo` will report "No changes detected."
