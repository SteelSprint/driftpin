package cli_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"drift/cli"
	"drift/internal/testutil"
	"drift/statestore"
)

func TestConcurrentLinkRace(t *testing.T) {
	const numGoroutines = 10
	const iterations = 5

	for iter := 0; iter < iterations; iter++ {
		dir := t.TempDir()
		writeMainDrift(t, dir, `<main>
  <spec id="s1">spec one</spec>
</main>`)
		cli.Run([]string{"init"}, dir)

		var codeBuilder strings.Builder
		for i := 0; i < numGoroutines; i++ {
			codeBuilder.WriteString(testutil.MarkerStart(fmt.Sprintf("m%d", i)))
			codeBuilder.WriteString("\nfunc f")
			codeBuilder.WriteString(fmt.Sprintf("%d", i))
			codeBuilder.WriteString("() {}\n")
			codeBuilder.WriteString(testutil.MarkerEnd(fmt.Sprintf("m%d", i)))
			codeBuilder.WriteString("\n")
		}
		testutil.WriteCodeFile(t, dir, "main.go", codeBuilder.String())
		cli.Run([]string{"todo"}, dir)

		barrier := make(chan struct{})
		var wg sync.WaitGroup
		start := func(idx int) {
			defer wg.Done()
			<-barrier
			_, code := cli.Run([]string{"link", fmt.Sprintf("m%d", idx), "main.s1"}, dir)
			if code != 0 {
				t.Errorf("iteration %d goroutine %d: link exit code %d", iter, idx, code)
			}
		}

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go start(i)
		}
		close(barrier)
		wg.Wait()

		store := statestore.NewFileStateStore(dir)
		state, err := store.Load()
		if err != nil {
			t.Fatalf("iteration %d: failed to load state: %v", iter, err)
		}
		if len(state.Edges) != numGoroutines {
			t.Fatalf("iteration %d: expected %d links after concurrent link, got %d — race condition caused silent data loss",
				iter, numGoroutines, len(state.Edges))
		}
	}
}
