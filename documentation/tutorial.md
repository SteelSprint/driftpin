# Tutorial

<!-- #F id:scdym9ww output.tooltip -->
<!-- #F id:9dqytjyy output.neutral_language -->
<!-- #F id:rr7xarbj drift.spec_drift -->
<!-- #F id:eullqhkn drift.site_drift -->

filament keeps your spec and code aligned. Specs are the source of truth;
code, docs, and config are implementations. When either side changes,
filament flags it for review — nothing changes silently.

This tutorial walks through a real project lifecycle. Each step shows a
prompt you can copy into your LLM, what the LLM does with it, and why
it matters.

## What filament believes

Specs are not documentation. Specs are contracts. When you write a spec
clause like "The server MUST listen on port 8080," you're making a
decision. That decision should flow into the code, and if either side
changes, someone should notice.

filament makes that notice automatic. It doesn't check whether code
*implements* a spec — that's a semantic judgment for you or your LLM.
It checks whether something *changed* since you last looked. That's
the drift signal.

```
  Without filament                 With filament

  spec.md → code                  spec.xml ◄──#F── code
  (no link)                       (traceable links)
  (no enforcement)                (drift detection)
  (silent drift)                  (enforced review)
```

## The core model

```
  ┌──────────┐                   ┌──────────┐
  │   SPEC   │◄──── #F markers ──│  FILES   │
  │ (truth)  │                   │ (content)│
  └────┬─────┘                   └────┬─────┘
       │                              │
       ▼                              ▼
   spec changes                 content changes
       │                              │
       ▼                              ▼
   SPEC_DRIFT                    SITE_DRIFT
   "spec changed,               "content changed,
    re-check the code"           re-read the spec"
       │                              │
       └──────────┬───────────────────┘
                  ▼
            review required
                  │
                  ▼
             resolve → in sync
```

The state file (`.filament`) tracks three things per marker:
- the spec hash at the time you reviewed it
- the content hash at the time you stored it
- which spec clauses the marker traces to

When any hash changes, the marker goes stale. That's drift.

## Step 1: Set up

**The concept:** Markers create traceable links from files to spec clauses.
Without markers, there's no proof a clause is addressed. A spec clause
with no marker is a "missing" clause — the spec says something should
exist, but there's no evidence it does.

```
  spec.xml                     main.go
  ┌──────────┐                ┌──────────────────┐
  │server.port│                │ func main() {    │
  │server.host│   no link      │   ListenAndServe │
  └──────────┘                │ }                │
                              └──────────────────┘
  2 clauses.                  No markers. No proof.
```

**Your prompt:**

> set up filament in my project and tell me what you find.

**What the LLM does:**

1. Runs `filament skill` to learn the tool — this is how the LLM
   discovers the commands, the marker format, the drift model, and
   the workflow. Every prompt assumes the LLM starts here.

2. Reads your spec and source files to understand the project.

3. Runs `filament init` — creates the `.filament` state file with
   spec hashes for every clause.

4. Runs `filament check` — finds missing clauses (clauses with no
   markers in any file).

5. Runs `filament add <clause_id>` for each missing clause — generates
   a marker line with a fresh 8-character id.

6. Tells you where to paste each marker. Typically above the function
   or block that implements the clause.

7. After you paste, runs `filament resolve --site <id>` to register
   the content hash, then `filament check` to verify.

**What the LLM reports:**

> Your spec has 2 clauses: server.port and server.host. I placed a
> marker in main.go above the ListenAndServe call. Both clauses are
> now tracked. No drift detected — this is your baseline.
>
> Note: your code listens on port 3000 but the spec says 8080.
> filament doesn't check semantic compliance — that's a review for
> you to do. The marker just proves the clause is addressed at that
> location.

**Why this matters:** The baseline establishes what's been reviewed.
Future changes are measured against this baseline. Without it, drift
has no reference point.

## Step 2: Spec changed

**The concept:** When the spec changes, every marker referencing the
changed clause is flagged. You must re-read the new spec wording and
verify the code still matches. This is the core value — silent spec
changes are the #1 source of bugs in LLM-generated code.

```
  spec.xml                     main.go
  ┌──────────┐                ┌──────────────────┐
  │server.port│  CHANGED!     │ // # F id:abc     │
  │"port 9000"│◄──drift──────│ ListenAndServe   │
  └──────────┘                └──────────────────┘
       │
       ▼
  SPEC_DRIFT: spec changed,
  review code against new wording
```

**Your prompt:**

> i updated the spec. check what drifted, review each one against
> the code, and clear it if the code still matches.

**What the LLM does:**

1. Runs `filament sync` — refreshes the spec hashes in `.filament`
   from the current spec XML. The lock file now has the new hashes;
   the reviewed hashes are still old. That's the drift signal.

2. Runs `filament check` — finds SPEC_DRIFT on every marker
   referencing the changed clause.

3. For each drifted marker, reads the new spec clause wording and
   compares against the code at the marker's location.

4. If they match: runs `filament resolve --spec <id>` — acknowledges
   the code still implements the new wording.

5. If they don't match: does NOT resolve. Reports the mismatch and
   recommends a code fix.

**What the LLM reports:**

> Spec clause server.port changed from "port 8080" to "port 9000."
> The code at main.go:11 still listens on port 3000. This does NOT
> match the new spec. I did not clear the drift.
>
> Recommended fix: change `:3000` to `:9000` in main.go, then
> re-check. Or update the spec if 3000 is correct.

**Why this matters:** The LLM was forced to read the new spec wording,
compare it against the code, and make a judgment call. It didn't just
run a command — it reviewed. If it had blindly cleared the drift, the
spec and code would silently diverge.

## Step 3: Code changed

**The concept:** When content near a marker changes, the spec clauses
it traces to are flagged. You must re-read the spec and verify the
new code still implements it. This catches code edits that break spec
compliance — especially LLM-generated edits.

```
  spec.xml                     main.go
  ┌──────────┐                ┌──────────────────┐
  │server.port│                │ // # F id:abc     │
  │"port 9000"│────drift─────►│ :9000  CHANGED!  │
  └──────────┘                └──────────────────┘
                                    │
                                    ▼
                              SITE_DRIFT: content changed,
                              re-read the spec
```

**Your prompt:**

> i changed some code. check if anything drifted, review it against
> the spec, and clear what's still aligned.

**What the LLM does:**

1. Runs `filament check` — finds SITE_DRIFT on markers near the
   changed code. May also find SPEC_DRIFT if the spec changed too.

2. For each drifted marker, reads the spec clause(s) it traces to.

3. Compares the spec wording against the new content at the marker's
   location.

4. If they match: runs `filament resolve --site <id>` — acknowledges
   the new content still implements the spec.

5. If they don't match: does NOT resolve. Reports the mismatch.

**What the LLM reports:**

> I found SITE_DRIFT on marker abc (main.go:11). The code now
> listens on port 9000. Spec clause server.port says "port 9000."
> These match — I cleared the drift with `filament resolve --site abc`.
>
> I also found SPEC_DRIFT on the same marker — the spec was edited
> and never re-acknowledged. Since the code matches the current spec,
> I cleared that too with `filament resolve --spec abc`.
>
> All markers are now in sync.

**Why this matters:** The LLM reviewed both directions of drift
(spec changed AND code changed) and cleared them only because the
code and spec are aligned. If the code had been wrong, the LLM
would have refused to clear — that's the enforcement.

## Step 4: Add a new clause

**The concept:** New spec clauses need markers in the code that
implements them. Without a marker, the clause is "missing" — the
spec says something should exist, but there's no evidence it does.

**Your prompt:**

> i want to add a new clause to the spec. the clause is
> server.timeout and the text is: "The server MUST time out
> after 30 seconds." set it up and tell me where to put
> the marker.

**What the LLM does:**

1. Edits `spec.xml` to add the new `<clause id="server.timeout">`
   element.

2. Runs `filament sync` — refreshes spec hashes. The new clause
   now has a hash in `.filament`.

3. Runs `filament add server.timeout` — generates a new marker
   line: `# F id:x7k2m9pa server.timeout`

4. Tells you where to paste it — above the timeout-related code.

5. After you paste, runs `filament resolve --site x7k2m9pa` to
   register the content hash.

6. Runs `filament check` to verify everything is in sync.

**What the LLM reports:**

> Added server.timeout to spec.xml. Generated marker:
>
>     # F id:x7k2m9pa server.timeout
>
> Paste this into main.go above the ListenAndServe call (or above
> the timeout configuration if you add one). After you paste, tell
> me and I'll register it.

**Why this matters:** Adding a clause is a one-prompt operation. The
LLM handles spec edit, hash refresh, marker generation, and
registration. Your job is to review the marker placement.

## Step 5: CI gate

**The concept:** CI catches drift that was never resolved — a safety
net for when the review-and-resolve workflow is skipped. `filament
check` exits 0 if everything is in sync, 1 if there's drift.

**Your prompt:**

> i want filament to fail my CI build if there's drift.

**What the LLM does:**

Shows you the CI step:

```yaml
- name: Check spec alignment
  run: filament check
```

That's it. One line. If any marker has drift, the build fails.

## When to use what

| Situation | What to prompt | What happens |
|-----------|---------------|--------------|
| Starting fresh | "set up filament..." | init, markers, baseline |
| Spec changed | "i updated the spec..." | sync, check, resolve --spec |
| Code changed | "i changed some code..." | check, resolve --site |
| Adding a clause | "i want to add..." | edit spec, sync, add, resolve |
| PR review | "do a full audit..." | check everything, report |
| CI gate | "fail my CI if there's drift" | `filament check` in CI |

## The mental model, in one picture

```
  ┌─────────────────────────────────────────────────────┐
  │                                                     │
  │   spec.xml ◄──── #F markers ──── files              │
  │       │                                │            │
  │       ▼                                ▼            │
  │   spec changed                   content changed     │
  │       │                                │            │
  │       ▼                                ▼            │
  │   SPEC_DRIFT                      SITE_DRIFT        │
  │   review code                     review spec        │
  │       │                                │            │
  │       └──────────┬─────────────────────┘            │
  │                  ▼                                  │
  │            review required                          │
  │                  │                                  │
  │                  ▼                                  │
  │             resolve → in sync                       │
  │                                                     │
  │   .filament tracks: spec hash, content hash,        │
  │   reviewed hash per marker-clause pair              │
  │                                                     │
  │   No silent changes. Everything flagged.             │
  │   Human writes specs. LLM enforces alignment.       │
  │                                                     │
  └─────────────────────────────────────────────────────┘
```
