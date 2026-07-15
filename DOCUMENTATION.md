# Documentation

## Mental model

When specs and their implementations change, `drift` informs your LLM. This gives your LLM a chance to fix any drifts as they happen. `drift` is packaged as a CLI that is intended mainly for LLMs to use.

## How it works

`drift` asks you to save your specs in XML markdown files. It then provides simple shortcode markers that detect changes in their surrounding code. Specs and markers form a many-to-many graph — a single spec term can be enforced by several markers, and a single marker can refer to several spec terms. Each link between a spec term and a marker is called an **edge**. When a change occurs on either side of an edge, `drift todo` surfaces one todo item per affected edge, with filepaths and line numbers where your LLM should check for drifts in specification.

`drift` hashes spec terms as well as markers, and saves those hashes inside `drift.pin`, which should be committed to git. This is a TOML file that contains the hashes of specs and markers, along with the links between them. It also contains a temporary state area for todo resolutions, that saves partial todo-list resolutions as the user is working through them. The algorithm manages this itself, and the user should refrain from touching the `drift.pin` file manually.

## Example

Lets say you have a marker `4hy7fh3h` and a spec file that has `validate_input`

On a clean run, `drift todo` reports no changes.

```bash
$ drift todo

No changes detected.
```

At this point, the `drift.pin` has at least the following:

```toml
[spec]
validate_input=S98YH3T2T32....

[marker]
4hy7fh3h=JHIO34YU08924QJYIO....

[links]
4hy7fh3h=["validate_input"]

[resolution_state]
```

Let's say you modify some code

```bash
$ drift todo

1 marker has unchecked changes.

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "validate_input" in "/workspaces/my-project/main.pin.xml:37". The marker has changed but not the spec term. Please check whether the changed code still complies with the spec term and make any modifications necessary. Once you are satisfied, run `drift reset 4hy7fh3h:validate_input` to mark this todo item as complete.
```

At this point the `drift.pin` is still unchanged

Then you mark the edge as resolved by running `drift reset 4hy7fh3h:validate_input`

```bash
$ drift reset 4hy7fh3h:validate_input
```

At this point, since the marker in question has no more unchecked specs, and the spec in question has no more unchecked markers, we just update the `drift.pin` file with the updated hashes. No resolution state yet.

```toml
[spec]
validate_input=FGHJKNEOUHT325TEnsetEST....

[marker]
4hy7fh3h=0HGO24G420G9IO34G....

[links]
4hy7fh3h=["validate_input"]

[resolution_state]
```

Let's say you modify a spec.

```bash
$ drift todo

1 spec item has unchecked changes.

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "validate_input" in "/workspaces/my-project/main.pin.xml:37". The spec term has changed but not the marker. Please check whether the new version of the spec term is still reflected in the code and make any modifications necessary. Once you are satisfied, run `drift reset 4hy7fh3h:validate_input` to mark this todo item as complete.
```

Now let's say you modify both a spec term and its related marker around the same time.

```bash
$ drift todo

1 marker has unchecked changes.
1 spec item has unchecked changes.

1. [TODO] Edge between marker "4hy7fh3h" in "/workspaces/my-project/src/main.go:15" and spec term "validate_input" in "/workspaces/my-project/main.pin.xml:37". Both the marker and the spec term have changed. Please check whether the changed code still complies with the new version of the spec term and make any modifications necessary on either side. Once you are satisfied, run `drift reset 4hy7fh3h:validate_input` to mark this todo item as complete.
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

## Drift.pin

Let's take the 3x3 example above.

Let's say we resolve the 1st item by using `drift` CLI commands

```bash
$ drift reset a1b2c3d4:check_fraud_rules
```

At this point, we still need bookkeeping to make sure that all of a1b2c3d4's specs are resolved, and same with check_fraud_rule's markers.

So in drift.pin, we have the following structure:

The document ends at the 3×3 example after resolving 2 of 9 edges. At that point, 7 edges remain unresolved, so no collapse has occurred. The baselines still hold the old hashes, and [resolution_state] captures the 2 reviewed edges:

```toml
[spec]
validate_amount = "old_hash_3a"
check_fraud_rules = "old_hash_3b"
log_transaction = "old_hash_3c"

[marker]
a1b2c3d4 = "old_hash_1a"
e5f6g7h8 = "old_hash_2a"
f9g0h1i2 = "old_hash_3a"

[links]
a1b2c3d4 = ["validate_amount", "check_fraud_rules", "log_transaction"]
e5f6g7h8 = ["validate_amount", "check_fraud_rules", "log_transaction"]
f9g0h1i2 = ["validate_amount", "check_fraud_rules", "log_transaction"]

[resolution_state]
"a1b2c3d4:validate_amount" = "current_hash_1a:current_hash_3a"
"e5f6g7h8:check_fraud_rules" = "current_hash_2a:current_hash_3b"
```

Why:
- [spec] and [marker] still hold baseline hashes (the last known-good state). They haven't been updated because collapse only happens when all edges are resolved.
- [links] defines the graph topology — which markers connect to which spec terms. This is the many-to-many mapping.
- [resolution_state] records the 2 edges that were reviewed, keyed by "marker_id:spec_term", valued by "current_marker_hash:current_spec_hash" at the time of review. If either side changes again, the value no longer matches current content → edge re-drifts.
- 7 edges still have mismatches between baseline and current hashes, so they remain in drift todo.
If all 9 were resolved, collapse would occur automatically — [spec] and [marker] would update to current hashes, and [resolution_state] would be cleared entirely.