# write-code

Use this skill when implementing features, fixing bugs, or making any code changes in this project.

## Core principle

The spec is the source of truth. The code implements the spec. Nothing exists outside the spec.

If you need to understand filament itself — commands, marker format, state file, drift model — run `filament skill`. This skill describes how we work, not how filament works.

## Two loops

Development has two nested loops:

- **Outer loop (walking skeleton):** Pick a small set of related clauses, implement them end-to-end, verify, commit. Repeat.
- **Inner loop (spec → test → code):** For each clause in the set, follow this three-step workflow.

### Outer loop: walking skeleton

Do not implement by layer (e.g., all parser changes, then all validator changes, then all commands). Instead, build a walking skeleton: a thin, end-to-end implementation of a small set of related clauses.

1. Pick a small set of related clauses (e.g., one new parser rule, its validation, its test case)
2. For each clause in the set, run the inner loop (spec → test → code)
3. Place `#F` markers in the code tracing back to the spec clauses you implemented
4. Verify the skeleton works end-to-end: `filament check` passes, `go test ./...` passes
5. Commit

The skeleton proves the architecture works before you build on it. Each iteration builds on a verified foundation, not on assumptions.

### Inner loop: spec → test → code

#### Step 1: Spec

Before writing any code, ensure the behavior is described in the spec XML (`filament.spec.xml`). If it's not in the spec, don't build it. If the spec is ambiguous, ask the user.

After editing the spec, sync and check. The check output is your work item list — each finding (MISSING, SPEC_DRIFT, SITE_DRIFT) is a thing to fix. Work through them one by one.

#### Step 2: Test

For each clause that needs implementation, write tests first. The tests must fail before you write implementation code (red/green).

Red: Write a test that exercises the behavior described in the spec clause. Run it. It must fail.

Green: Write the minimum code to make the test pass. Run it. It must pass.

A test that was never red is not a test. It's a rubber stamp.

For each clause, test:

- The happy path: behavior as described in the spec
- Edge cases: empty input, boundary values, missing data
- Error cases: invalid input, malformed data, missing dependencies

If a clause says "MUST" — test that it does. If a clause says "MUST NOT" — test that it doesn't.

#### Step 3: Code

Implement the minimum code to satisfy the spec clause and pass the tests. Nothing more.

When writing code that produces output (error messages, findings, status reports), follow the self-explaining philosophy:

- WHAT happened: state the fact
- WHY it matters: explain the implication
- HOW to fix: provide the exact command or action

The LLM reading the output should never need external context to understand what went wrong, why it matters, or what to do next.

After implementing, place a `#F` marker above the code that implements each clause. Use `filament add <clause_id>` to generate the marker line. Then resolve drift and verify: `filament check` passes, `go test ./...` passes.

## Rules

### The spec is the contract

If the spec doesn't describe it, don't build it. Do not add convenience features, helper functions, or "nice to have" improvements unless the spec explicitly requires them. If you see an opportunity for improvement, note it for the user but do not implement it.

### Clarify ambiguity

If a spec clause is ambiguous, ask the user before implementing. Do not guess. Ambiguity in the spec is a bug in the spec, not an invitation to interpret. Examples of ambiguity:

- A clause says "the tool MUST handle errors" but doesn't specify how
- A clause says "the output MUST be prose" but doesn't define the format
- Two clauses seem to contradict each other

When in doubt, ask.

### Commit after each walking skeleton

After each walking skeleton is complete and verified:

1. `filament check` passes
2. `go test ./...` passes
3. `go vet ./...` passes

Then commit. A commit is one walking skeleton — the end-to-end implementation of one small set of related clauses. The commit message should reference the spec clauses that were implemented.
