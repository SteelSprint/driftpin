package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"driftpin/cli"
	"driftpin/internal/testutil"
	"driftpin/pinstore"
)

func writeMainPin(t *testing.T, dir, content string) {
	t.Helper()
	testutil.WriteSpecFile(t, dir, "main.pin.xml", content)
}

func TestCLIInit(t *testing.T) {
	t.Run("init_creates_drift_pin", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		output, code := cli.Run([]string{"init"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		pinPath := filepath.Join(dir, "drift.pin")
		if _, err := os.Stat(pinPath); os.IsNotExist(err) {
			t.Fatalf("drift.pin not created")
		}
	})

	t.Run("init_then_todo_no_changes", func(t *testing.T) {
		dir := t.TempDir()

		_, code := cli.Run([]string{"init"}, dir)
		if code != 0 {
			t.Fatalf("init failed with non-zero exit code")
		}

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.HasPrefix(output, "No changes detected.") {
			t.Fatalf("output = %q, want \"No changes detected.\" prefix", output)
		}
	})

	t.Run("init_creates_valid_empty_pin", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		_, code := cli.Run([]string{"init"}, dir)
		if code != 0 {
			t.Fatalf("init failed")
		}

		store := pinstore.NewFilePinStore(dir)
		state, err := store.Load()
		testutil.AssertNoError(t, err)
		testutil.AssertPinStateEquals(t, state, pinstore.PinState{})
	})
}

func TestCLITodoWithoutInit(t *testing.T) {
	t.Run("todo_without_init_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		output, code := cli.Run([]string{"todo"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
		if !strings.Contains(strings.ToLower(output), "init") {
			t.Fatalf("error message should mention init, got: %s", output)
		}
	})
}

func TestCLIResetWithoutInit(t *testing.T) {
	t.Run("reset_without_init_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		output, code := cli.Run([]string{"reset", "m1", "main.s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})
}

func TestCLINoArgs(t *testing.T) {
	t.Run("no_args_shows_help", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		output, code := cli.Run([]string{}, dir)
		if code != 0 {
			t.Fatalf("expected exit code 0 for no args, got %d, output: %s", code, output)
		}
		if !strings.Contains(strings.ToLower(output), "usage") {
			t.Fatalf("output should contain usage, got: %s", output)
		}
	})
}

func TestCLIUnknownCommand(t *testing.T) {
	t.Run("unknown_command_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		output, code := cli.Run([]string{"frobnicate"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for unknown command")
		}
		if !strings.Contains(strings.ToLower(output), "unknown") {
			t.Fatalf("output should mention unknown command, got: %s", output)
		}
	})
}

func TestCLIResetBadFormat(t *testing.T) {
	t.Run("reset_without_arguments", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)
		output, code := cli.Run([]string{"reset"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("reset_missing_spec_argument", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)
		output, code := cli.Run([]string{"reset", "m1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})
}

func TestCLIFullFlowSpecMarkerLinkDrift(t *testing.T) {
	t.Run("init_create_spec_create_marker_todo_no_links_no_drift", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.HasPrefix(output, "No changes detected.") {
			t.Fatalf("output = %q, want \"No changes detected.\" prefix", output)
		}
	})

	t.Run("link_then_todo_no_drift", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)

		output, code := cli.Run([]string{"link", "abc123", "main.validate_input"}, dir)
		if code != 0 {
			t.Fatalf("link failed, exit code = %d, output: %s", code, output)
		}

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.HasPrefix(output, "No changes detected.") {
			t.Fatalf("output = %q, want \"No changes detected.\" prefix", output)
		}
	})

	t.Run("link_then_modify_code_then_todo_shows_drift", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "abc123", "main.validate_input"}, dir)
		cli.Run([]string{"todo"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomethingElse()
}
`+testutil.MarkerEnd("abc123")+`
`)

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "1 marker has unchecked changes") {
			t.Fatalf("output should mention 1 marker with unchecked changes, got: %s", output)
		}
		if !strings.Contains(output, "abc123") {
			t.Fatalf("output should contain marker id abc123, got: %s", output)
		}
		if !strings.Contains(output, "main.validate_input") {
			t.Fatalf("output should contain spec id main.validate_input, got: %s", output)
		}
		if !strings.Contains(output, "drift reset abc123 main.validate_input") {
			t.Fatalf("output should contain reset command with space separator, got: %s", output)
		}
	})

	t.Run("drift_then_reset_clears_drift", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "abc123", "main.validate_input"}, dir)
		cli.Run([]string{"todo"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomethingElse()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)

		_, code := cli.Run([]string{"reset", "abc123", "main.validate_input"}, dir)
		if code != 0 {
			t.Fatalf("reset failed with non-zero exit code")
		}

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.HasPrefix(output, "No changes detected.") {
			t.Fatalf("output = %q, want \"No changes detected.\" prefix", output)
		}
	})

	t.Run("link_with_module_imports", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <import path="./core.pin.xml" />
</main>`)
		testutil.WriteSpecFile(t, dir, "core.pin.xml", `<module name="core">
  <spec id="validate">Validation must reject duplicates.</spec>
</module>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func validate() { check() }
`+testutil.MarkerEnd("m1")+`
`)

		cli.Run([]string{"todo"}, dir)

		output, code := cli.Run([]string{"link", "m1", "core.validate"}, dir)
		if code != 0 {
			t.Fatalf("link failed, exit code = %d, output: %s", code, output)
		}

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.HasPrefix(output, "No changes detected.") {
			t.Fatalf("output = %q, want \"No changes detected.\" prefix", output)
		}
	})
}

func TestCLILinkErrors(t *testing.T) {
	t.Run("link_nonexistent_marker", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"link", "nonexistent", "main.s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("link_nonexistent_spec", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() {}
`+testutil.MarkerEnd("m1")+`
`)

		output, code := cli.Run([]string{"link", "m1", "main.nonexistent"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("link_duplicate", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() {}
`+testutil.MarkerEnd("m1")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)

		output, code := cli.Run([]string{"link", "m1", "main.s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for duplicate link, got 0, output: %s", output)
		}
	})

	t.Run("link_missing_spec_argument", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"link", "m1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("link_without_init", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		output, code := cli.Run([]string{"link", "m1", "main.s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})
}

func assertTodoCountInOutput(t *testing.T, output string, want int) {
	t.Helper()
	if want == 0 {
		if !strings.HasPrefix(output, "No changes detected.") && !strings.HasPrefix(output, "Nothing to check:") {
			t.Fatalf("output = %q, want \"No changes detected.\" or \"Nothing to check:\" prefix", output)
		}
		return
	}
	if !strings.Contains(output, fmt.Sprintf("%d. [TODO]", want)) {
		t.Fatalf("output should contain %d todo items, got: %s", want, output)
	}
	lines := strings.Count(output, "[TODO]")
	if lines != want {
		t.Fatalf("output contains %d [TODO] entries, want %d, output: %s", lines, want, output)
	}
}

func assertPinResolutionCount(t *testing.T, dir string, want int) {
	t.Helper()
	store := pinstore.NewFilePinStore(dir)
	state, err := store.Load()
	testutil.AssertNoError(t, err)
	if len(state.ResolutionState) != want {
		t.Fatalf("resolution state count = %d, want %d", len(state.ResolutionState), want)
	}
}

func TestCLIManyToManyOneSpecManyMarkers(t *testing.T) {
	t.Run("1_spec_2_markers_full_cycle", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="auth_token_expiry">token must expire</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "middleware.go", testutil.MarkerStart("m1")+`
func authMiddleware() {
	checkExpiry()
}
`+testutil.MarkerEnd("m1")+`
`)
		testutil.WriteCodeFile(t, dir, "login.go", testutil.MarkerStart("m2")+`
func loginHandler() {
	checkExpiry()
}
`+testutil.MarkerEnd("m2")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.auth_token_expiry"}, dir)
		cli.Run([]string{"link", "m2", "main.auth_token_expiry"}, dir)

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)

		writeMainPin(t, dir, `<main>
  <spec id="auth_token_expiry">token must expire within 24 hours</spec>
</main>`)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 2)

		_, code = cli.Run([]string{"reset", "m1", "main.auth_token_expiry"}, dir)
		if code != 0 {
			t.Fatalf("reset m1 failed")
		}
		assertPinResolutionCount(t, dir, 1)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 1)

		_, code = cli.Run([]string{"reset", "m2", "main.auth_token_expiry"}, dir)
		if code != 0 {
			t.Fatalf("reset m2 failed")
		}
		assertPinResolutionCount(t, dir, 0)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)
	})
}

func TestCLIManyToManyOneMarkerManySpecs(t *testing.T) {
	t.Run("2_specs_1_marker_full_cycle", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_file_size">file size must be validated</spec>
  <spec id="scan_for_malware">files must be scanned for malware</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "upload.go", testutil.MarkerStart("m1")+`
func uploadHandler() {
	validateAndScan()
}
`+testutil.MarkerEnd("m1")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.validate_file_size"}, dir)
		cli.Run([]string{"link", "m1", "main.scan_for_malware"}, dir)

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)

		testutil.WriteCodeFile(t, dir, "upload.go", testutil.MarkerStart("m1")+`
func uploadHandler() {
	upload()
}
`+testutil.MarkerEnd("m1")+`
`)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 2)

		_, code = cli.Run([]string{"reset", "m1", "main.validate_file_size"}, dir)
		if code != 0 {
			t.Fatalf("reset validate_file_size failed")
		}
		assertPinResolutionCount(t, dir, 1)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 1)

		_, code = cli.Run([]string{"reset", "m1", "main.scan_for_malware"}, dir)
		if code != 0 {
			t.Fatalf("reset scan_for_malware failed")
		}
		assertPinResolutionCount(t, dir, 0)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)
	})
}

func TestCLIManyToManyTwoByTwo(t *testing.T) {
	t.Run("2_specs_2_markers_4_edges_full_cycle", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="rate_limit_per_user">per-user rate limiting required</spec>
  <spec id="log_rate_limit_hits">rate limit hits must be logged</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "middleware.go", testutil.MarkerStart("m1")+`
func rateLimitMiddleware() {
	limit()
}
`+testutil.MarkerEnd("m1")+`
`)
		testutil.WriteCodeFile(t, dir, "handler.go", testutil.MarkerStart("m2")+`
func requestHandler() {
	handle()
}
`+testutil.MarkerEnd("m2")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.rate_limit_per_user"}, dir)
		cli.Run([]string{"link", "m1", "main.log_rate_limit_hits"}, dir)
		cli.Run([]string{"link", "m2", "main.rate_limit_per_user"}, dir)
		cli.Run([]string{"link", "m2", "main.log_rate_limit_hits"}, dir)

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)

		writeMainPin(t, dir, `<main>
  <spec id="rate_limit_per_user">per-user rate limiting required with 100 req/min</spec>
  <spec id="log_rate_limit_hits">rate limit hits must be logged to syslog</spec>
</main>`)
		testutil.WriteCodeFile(t, dir, "middleware.go", testutil.MarkerStart("m1")+`
func rateLimitMiddleware() {
	limitV2()
}
`+testutil.MarkerEnd("m1")+`
`)
		testutil.WriteCodeFile(t, dir, "handler.go", testutil.MarkerStart("m2")+`
func requestHandler() {
	handleV2()
}
`+testutil.MarkerEnd("m2")+`
`)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 4)

		_, code = cli.Run([]string{"reset", "m1", "main.rate_limit_per_user"}, dir)
		if code != 0 {
			t.Fatalf("reset 1 failed")
		}
		assertPinResolutionCount(t, dir, 1)
		output, _ = cli.Run([]string{"todo"}, dir)
		assertTodoCountInOutput(t, output, 3)

		_, code = cli.Run([]string{"reset", "m1", "main.log_rate_limit_hits"}, dir)
		if code != 0 {
			t.Fatalf("reset 2 failed")
		}
		assertPinResolutionCount(t, dir, 2)
		output, _ = cli.Run([]string{"todo"}, dir)
		assertTodoCountInOutput(t, output, 2)

		_, code = cli.Run([]string{"reset", "m2", "main.rate_limit_per_user"}, dir)
		if code != 0 {
			t.Fatalf("reset 3 failed")
		}
		assertPinResolutionCount(t, dir, 2)
		output, _ = cli.Run([]string{"todo"}, dir)
		assertTodoCountInOutput(t, output, 1)

		_, code = cli.Run([]string{"reset", "m2", "main.log_rate_limit_hits"}, dir)
		if code != 0 {
			t.Fatalf("reset 4 failed")
		}
		assertPinResolutionCount(t, dir, 0)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)
	})
}

func TestCLIManyToManyThreeByThree(t *testing.T) {
	t.Run("3_specs_3_markers_9_edges_partial_then_full_collapse", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_amount">amount must be validated</spec>
  <spec id="check_fraud_rules">fraud rules must be checked</spec>
  <spec id="log_transaction">transactions must be logged</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "card.go", testutil.MarkerStart("m1")+`
func cardHandler() {
	processCard()
}
`+testutil.MarkerEnd("m1")+`
`)
		testutil.WriteCodeFile(t, dir, "bank.go", testutil.MarkerStart("m2")+`
func bankTransferHandler() {
	processBank()
}
`+testutil.MarkerEnd("m2")+`
`)
		testutil.WriteCodeFile(t, dir, "wallet.go", testutil.MarkerStart("m3")+`
func walletHandler() {
	processWallet()
}
`+testutil.MarkerEnd("m3")+`
`)

		cli.Run([]string{"todo"}, dir)

		links := []struct{ marker, spec string }{
			{"m1", "main.validate_amount"}, {"m1", "main.check_fraud_rules"}, {"m1", "main.log_transaction"},
			{"m2", "main.validate_amount"}, {"m2", "main.check_fraud_rules"}, {"m2", "main.log_transaction"},
			{"m3", "main.validate_amount"}, {"m3", "main.check_fraud_rules"}, {"m3", "main.log_transaction"},
		}
		for _, link := range links {
			_, code := cli.Run([]string{"link", link.marker, link.spec}, dir)
			if code != 0 {
				t.Fatalf("link %s %s failed", link.marker, link.spec)
			}
		}

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)

		writeMainPin(t, dir, `<main>
  <spec id="validate_amount">amount must be validated and positive</spec>
  <spec id="check_fraud_rules">fraud rules must be checked with ML model</spec>
  <spec id="log_transaction">transactions must be logged with audit trail</spec>
</main>`)
		testutil.WriteCodeFile(t, dir, "card.go", testutil.MarkerStart("m1")+`
func cardHandler() {
	processCardV2()
}
`+testutil.MarkerEnd("m1")+`
`)
		testutil.WriteCodeFile(t, dir, "bank.go", testutil.MarkerStart("m2")+`
func bankTransferHandler() {
	processBankV2()
}
`+testutil.MarkerEnd("m2")+`
`)
		testutil.WriteCodeFile(t, dir, "wallet.go", testutil.MarkerStart("m3")+`
func walletHandler() {
	processWalletV2()
}
`+testutil.MarkerEnd("m3")+`
`)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 9)

		_, code = cli.Run([]string{"reset", "m1", "main.validate_amount"}, dir)
		if code != 0 {
			t.Fatalf("reset m1 main.validate_amount failed")
		}
		_, code = cli.Run([]string{"reset", "m2", "main.check_fraud_rules"}, dir)
		if code != 0 {
			t.Fatalf("reset m2 main.check_fraud_rules failed")
		}
		assertPinResolutionCount(t, dir, 2)

		output, _ = cli.Run([]string{"todo"}, dir)
		assertTodoCountInOutput(t, output, 7)

		for _, link := range links {
			if link.marker == "m1" && link.spec == "main.validate_amount" {
				continue
			}
			if link.marker == "m2" && link.spec == "main.check_fraud_rules" {
				continue
			}
			_, code := cli.Run([]string{"reset", link.marker, link.spec}, dir)
			if code != 0 {
				t.Fatalf("reset %s %s failed", link.marker, link.spec)
			}
		}
		assertPinResolutionCount(t, dir, 0)

		output, code = cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, output: %s", code, output)
		}
		assertTodoCountInOutput(t, output, 0)

		store := pinstore.NewFilePinStore(dir)
		state, err := store.Load()
		testutil.AssertNoError(t, err)
		if len(state.Links) != 9 {
			t.Fatalf("expected 9 links in pin, got %d", len(state.Links))
		}
		if len(state.ResolutionState) != 0 {
			t.Fatalf("expected 0 resolutions after full collapse, got %d", len(state.ResolutionState))
		}
	})
}

func TestCLIUnlink(t *testing.T) {
	t.Run("unlink_removes_link", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "abc123", "main.validate_input"}, dir)

		output, code := cli.Run([]string{"unlink", "abc123", "main.validate_input"}, dir)
		if code != 0 {
			t.Fatalf("unlink failed, exit code = %d, output: %s", code, output)
		}
		if !strings.Contains(output, "Unlinked") {
			t.Fatalf("output should contain 'Unlinked', got: %s", output)
		}

		store := pinstore.NewFilePinStore(dir)
		state, err := store.Load()
		testutil.AssertNoError(t, err)
		if len(state.Links) != 0 {
			t.Fatalf("expected 0 links after unlink, got %d", len(state.Links))
		}
	})

	t.Run("unlink_nonexistent_link_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() {}
`+testutil.MarkerEnd("m1")+`
`)

		cli.Run([]string{"todo"}, dir)

		output, code := cli.Run([]string{"unlink", "m1", "main.s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for unlinking nonexistent link, got 0, output: %s", output)
		}
		if !strings.Contains(strings.ToLower(output), "no link") {
			t.Fatalf("error should mention 'no link', got: %s", output)
		}
	})

	t.Run("unlink_missing_args_returns_usage", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"unlink"}, dir)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(output, "usage: drift unlink") {
			t.Fatalf("output should contain usage, got: %s", output)
		}
	})

	t.Run("unlink_without_init_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)

		output, code := cli.Run([]string{"unlink", "m1", "main.s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("unlink_removes_resolution_state", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"todo"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomethingElse() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"reset", "m1", "main.s1"}, dir)

		store := pinstore.NewFilePinStore(dir)
		state, _ := store.Load()
		if len(state.ResolutionState) != 0 {
			t.Fatalf("expected 0 resolutions after reset-collapse, got %d", len(state.ResolutionState))
		}

		cli.Run([]string{"link", "m1", "main.s1"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { anotherChange() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"reset", "m1", "main.s1"}, dir)

		store = pinstore.NewFilePinStore(dir)
		state, _ = store.Load()
		assertPinResolutionCount(t, dir, 0)

		cli.Run([]string{"unlink", "m1", "main.s1"}, dir)

		store = pinstore.NewFilePinStore(dir)
		state, _ = store.Load()
		if len(state.Links) != 0 {
			t.Fatalf("expected 0 links after unlink, got %d", len(state.Links))
		}
		if len(state.ResolutionState) != 0 {
			t.Fatalf("expected 0 resolutions after unlink, got %d", len(state.ResolutionState))
		}
	})
}

func TestCLIList(t *testing.T) {
	t.Run("list_no_init_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)

		output, code := cli.Run([]string{"list"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})

	t.Run("list_empty_shows_no_specs", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"list"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "No specs or markers registered") {
			t.Fatalf("output should mention no specs, got: %s", output)
		}
	})

	t.Run("list_shows_specs_markers_links", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "abc123", "main.validate_input"}, dir)

		output, code := cli.Run([]string{"list"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "Specs (1):") {
			t.Fatalf("output should show 1 spec, got: %s", output)
		}
		if !strings.Contains(output, "Markers (1):") {
			t.Fatalf("output should show 1 marker, got: %s", output)
		}
		if !strings.Contains(output, "Links (1):") {
			t.Fatalf("output should show 1 link, got: %s", output)
		}
		if !strings.Contains(output, "main.validate_input") {
			t.Fatalf("output should contain spec ID, got: %s", output)
		}
		if !strings.Contains(output, "abc123") {
			t.Fatalf("output should contain marker ID, got: %s", output)
		}
		if !strings.Contains(output, "[synced]") {
			t.Fatalf("output should show synced status, got: %s", output)
		}
	})

	t.Run("list_shows_drifted_status", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"todo"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomethingElse() }
`+testutil.MarkerEnd("m1")+`
`)

		output, code := cli.Run([]string{"list"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "[DRIFTED]") {
			t.Fatalf("output should show drifted status, got: %s", output)
		}
	})

	t.Run("list_shows_unlinked", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
  <spec id="s2">spec two</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)
		testutil.WriteCodeFile(t, dir, "other.go", testutil.MarkerStart("m2")+`
func b() { doOther() }
`+testutil.MarkerEnd("m2")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)

		output, code := cli.Run([]string{"list"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "[unlinked]") {
			t.Fatalf("output should show unlinked items, got: %s", output)
		}
	})
}

func TestCLIPerSubcommandHelp(t *testing.T) {
	dir := t.TempDir()
	writeMainPin(t, dir, `<main></main>`)
	cli.Run([]string{"init"}, dir)

	tests := []struct {
		cmd   []string
		usage string
	}{
		{[]string{"link", "--help"}, "Usage: drift link"},
		{[]string{"link", "-h"}, "Usage: drift link"},
		{[]string{"reset", "--help"}, "drift reset <marker>"},
		{[]string{"reset", "-h"}, "drift reset <marker>"},
		{[]string{"unlink", "--help"}, "Usage: drift unlink"},
		{[]string{"unlink", "-h"}, "Usage: drift unlink"},
		{[]string{"list", "--help"}, "Usage: drift list"},
		{[]string{"list", "-h"}, "Usage: drift list"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.cmd, " "), func(t *testing.T) {
			output, code := cli.Run(tt.cmd, dir)
			if code != 0 {
				t.Fatalf("expected exit code 0 for %s, got %d, output: %s", strings.Join(tt.cmd, " "), code, output)
			}
			if !strings.Contains(output, tt.usage) {
				t.Fatalf("output should contain %q, got: %s", tt.usage, output)
			}
		})
	}
}

func TestCLIDeletionDrift(t *testing.T) {
	t.Run("spec_deleted_shows_deletion_message", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
  <spec id="s2">spec two</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"todo"}, dir)

		writeMainPin(t, dir, `<main>
  <spec id="s2">spec two</spec>
</main>`)

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "deleted from disk") {
			t.Fatalf("output should mention deletion, got: %s", output)
		}
		if !strings.Contains(output, "drift reset m1 main.s1") {
			t.Fatalf("output should contain reset command, got: %s", output)
		}
	})

	t.Run("spec_deleted_reset_prunes_and_cleans", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
  <spec id="s2">spec two</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"todo"}, dir)

		writeMainPin(t, dir, `<main>
  <spec id="s2">spec two</spec>
</main>`)

		_, code := cli.Run([]string{"reset", "m1", "main.s1"}, dir)
		if code != 0 {
			t.Fatalf("reset failed")
		}

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.HasPrefix(output, "No changes detected.") {
			t.Fatalf("expected clean todo after reset, got: %s", output)
		}

		store := pinstore.NewFilePinStore(dir)
		state, err := store.Load()
		testutil.AssertNoError(t, err)
		for _, s := range state.Specs {
			if s.ID == "main.s1" {
				t.Fatalf("deleted spec main.s1 should have been pruned from drift.pin")
			}
		}
		for _, l := range state.Links {
			if l.SpecID == "main.s1" {
				t.Fatalf("link to deleted spec should have been pruned")
			}
		}
	})

	t.Run("marker_deleted_shows_deletion_message", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`+testutil.MarkerStart("m2")+`
func b() { doOther() }
`+testutil.MarkerEnd("m2")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"link", "m2", "main.s1"}, dir)
		cli.Run([]string{"todo"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)

		output, code := cli.Run([]string{"todo"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "deleted from disk") {
			t.Fatalf("output should mention deletion, got: %s", output)
		}
	})
}

func TestCLIOrphanReset(t *testing.T) {
	t.Run("reset_orphan_spec", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
  <spec id="s2">spec two</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"link", "m2", "main.s2"}, dir)
		cli.Run([]string{"todo"}, dir)

		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"reset", "m2", "main.s2"}, dir)

		output, code := cli.Run([]string{"reset", "main.s2"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "Removed deleted spec") {
			t.Fatalf("output should mention removal, got: %s", output)
		}

		store := pinstore.NewFilePinStore(dir)
		state, _ := store.Load()
		for _, s := range state.Specs {
			if s.ID == "main.s2" {
				t.Fatalf("orphan spec should have been removed from drift.pin")
			}
		}
	})

	t.Run("reset_orphan_marker", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`+testutil.MarkerStart("m2")+`
func b() { doOther() }
`+testutil.MarkerEnd("m2")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		// m2 is NOT linked — it's just baselined
		cli.Run([]string{"todo"}, dir)

		// Delete m2 from code — it becomes an orphan (no links)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)

		// m2 is now stale (in drift.pin but not on disk, no links)
		output, code := cli.Run([]string{"reset", "m2"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "Removed deleted marker") {
			t.Fatalf("output should mention removal, got: %s", output)
		}

		store := pinstore.NewFilePinStore(dir)
		state, _ := store.Load()
		for _, m := range state.Markers {
			if m.ID == "m2" {
				t.Fatalf("orphan marker should have been removed from drift.pin")
			}
		}
	})

	t.Run("reset_orphan_live_spec_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"todo"}, dir)

		output, code := cli.Run([]string{"reset", "main.s1"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for live spec, got 0, output: %s", output)
		}
		if !strings.Contains(strings.ToLower(output), "still on disk") {
			t.Fatalf("error should mention 'still on disk', got: %s", output)
		}
	})

	t.Run("reset_orphan_with_links_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
  <spec id="s2">spec two</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s2"}, dir)
		cli.Run([]string{"todo"}, dir)

		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)

		output, code := cli.Run([]string{"reset", "main.s2"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for spec with links, got 0, output: %s", output)
		}
		if !strings.Contains(strings.ToLower(output), "link") {
			t.Fatalf("error should mention links, got: %s", output)
		}
	})

	t.Run("reset_orphan_nonexistent_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"reset", "nonexistent"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
	})
}

func TestCLIListDeletedTag(t *testing.T) {
	t.Run("list_shows_deleted_spec", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
  <spec id="s2">spec two</spec>
</main>`)
		cli.Run([]string{"init"}, dir)
		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("m1")+`
func a() { doSomething() }
`+testutil.MarkerEnd("m1")+`
`)
		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "m1", "main.s1"}, dir)
		cli.Run([]string{"todo"}, dir)

		writeMainPin(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)

		output, code := cli.Run([]string{"list"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "[deleted]") {
			t.Fatalf("output should show [deleted] tag, got: %s", output)
		}
	})
}

func TestCLIShow(t *testing.T) {
	t.Run("show_marker_displays_spec_and_content", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "abc123", "main.validate_input"}, dir)

		output, code := cli.Run([]string{"show", "abc123"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "=== Spec: main.validate_input ===") {
			t.Fatalf("output should contain spec section header, got: %s", output)
		}
		if !strings.Contains(output, "input must be validated") {
			t.Fatalf("output should contain spec text, got: %s", output)
		}
		if !strings.Contains(output, "=== Marker: abc123 ===") {
			t.Fatalf("output should contain marker section header, got: %s", output)
		}
		if !strings.Contains(output, "func handleRequest()") {
			t.Fatalf("output should contain marker code content, got: %s", output)
		}
		if !strings.Contains(output, "main.go") {
			t.Fatalf("output should contain filepath, got: %s", output)
		}
	})

	t.Run("show_spec_displays_spec_and_linked_markers", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)
		cli.Run([]string{"link", "abc123", "main.validate_input"}, dir)

		output, code := cli.Run([]string{"show", "main.validate_input"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "=== Spec: main.validate_input ===") {
			t.Fatalf("output should contain spec section header, got: %s", output)
		}
		if !strings.Contains(output, "input must be validated") {
			t.Fatalf("output should contain spec text, got: %s", output)
		}
		if !strings.Contains(output, "=== Marker: abc123 ===") {
			t.Fatalf("output should contain marker section header, got: %s", output)
		}
		if !strings.Contains(output, "func handleRequest()") {
			t.Fatalf("output should contain marker code content, got: %s", output)
		}
	})

	t.Run("show_nonexistent_marker_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"show", "nonexistent"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
		if !strings.Contains(output, "not found") {
			t.Fatalf("error should mention 'not found', got: %s", output)
		}
	})

	t.Run("show_nonexistent_spec_errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"show", "main.nonexistent"}, dir)
		if code == 0 {
			t.Fatalf("expected non-zero exit code, got 0, output: %s", output)
		}
		if !strings.Contains(output, "not found") {
			t.Fatalf("error should mention 'not found', got: %s", output)
		}
	})

	t.Run("show_missing_args_returns_usage", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main></main>`)
		cli.Run([]string{"init"}, dir)

		output, code := cli.Run([]string{"show"}, dir)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(output, "usage") {
			t.Fatalf("output should contain usage, got: %s", output)
		}
	})

	t.Run("show_unlinked_marker_shows_no_specs", func(t *testing.T) {
		dir := t.TempDir()
		writeMainPin(t, dir, `<main>
  <spec id="validate_input">input must be validated</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		testutil.WriteCodeFile(t, dir, "main.go", testutil.MarkerStart("abc123")+`
func handleRequest() {
	doSomething()
}
`+testutil.MarkerEnd("abc123")+`
`)

		cli.Run([]string{"todo"}, dir)

		output, code := cli.Run([]string{"show", "abc123"}, dir)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0, output: %s", code, output)
		}
		if !strings.Contains(output, "=== Marker: abc123 ===") {
			t.Fatalf("output should contain marker section header, got: %s", output)
		}
		if strings.Contains(output, "=== Spec:") {
			t.Fatalf("output should not contain spec section for unlinked marker, got: %s", output)
		}
	})
}
