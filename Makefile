.PHONY: build eval gate build-bin

# build-bin compiles the drift binary. The prior ./drift (if any) is staged
# to bak/.drift-stash before the build and promoted to bak/drift-<UTC-timestamp>
# only on a successful build — so a failed build leaves ./drift untouched and
# produces no bak/ entry. bak/ is gitignored. Roll back with
# `cp bak/drift-<ts> drift`.
build-bin:
	@mkdir -p bak
	@if [ -f drift ]; then cp drift bak/.drift-stash; fi
	go build -o drift ./cmd/drift
	@if [ -f bak/.drift-stash ]; then \
		ts=$$(date -u +%Y%m%dT%H%M%SZ); \
		mv bak/.drift-stash "bak/drift-$$ts"; \
		echo "backed up prior drift → bak/drift-$$ts"; \
	fi

# gate runs drift todo as a spec-drift gate. Exits non-zero if any drift
# is detected (link/spec/marker hash drift, ref-graph drift, cascade drift,
# unlinked markers, broken refs). Blocks the build until the tree is clean.
# Run `drift todo` directly to see what drifted; `drift diff --all` to review.
gate: build-bin
	./drift todo
	@echo "drift gate: clean"

# build runs the gate, then leaves the compiled binary in ./drift.
build: gate
	@echo "build complete: ./drift"

eval: build
	@if [ -z "$(PROMPT)" ]; then \
		echo "Usage: make eval PROMPT=\"<your prompt>\""; \
		echo "  e.g. make eval PROMPT=\"create a working CLI version of poker\""; \
		echo ""; \
		echo "Or run the full test battery:"; \
		echo "  go run ./eval --battery"; \
		echo ""; \
		echo "Other eval options:"; \
		echo "  go run ./eval --dry-run \"<prompt>\"     # stage only, skip LLM calls"; \
		echo "  go run ./eval --subject <model> \"<prompt>\"  # override subject model"; \
		echo "  go run ./eval --judge <model> \"<prompt>\"    # override judge model"; \
		echo "  go run ./eval --label <name> \"<prompt>\"     # name the run"; \
		exit 1; \
	fi
	@go run ./eval "$(PROMPT)"
