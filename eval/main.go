package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type promptItem struct {
	name    string
	content string
}

func main() {
	var (
		subjectModel = flag.String("subject", "openrouter/xiaomi/mimo-v2.5-pro", "subject LLM model ID")
		judgeModel   = flag.String("judge", "openrouter/z-ai/glm-5.2", "judge LLM model ID")
		battery      = flag.Bool("battery", false, "run all prompts in eval/prompts/ instead of a single prompt")
		only         = flag.String("only", "", "comma-separated list of prompt names to run (implies --battery). Example: --only level1,level2,level3")
		runs         = flag.Int("runs", 2, "number of battery prompts to run (capped by available prompts)")
		repeat       = flag.Int("repeat", 1, "repeat each prompt N times for statistical baseline")
		label        = flag.String("label", "", "label for this run (defaults to timestamp)")
		dryRun       = flag.Bool("dry-run", false, "stage only, skip LLM calls")
		skipJudge    = flag.Bool("skip-judge", false, "skip the judge pass; subject transcripts are still written. Useful for cheap iteration when the subject signal is the primary value")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run ./eval [flags] [prompt]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval \"create a working CLI version of poker\"\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval --battery              # run first 2 prompts in parallel\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval --battery --runs 3     # run first 3 prompts in parallel\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval --only level1,level2   # run only the named prompts\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval --subject openrouter/anthropic/claude-haiku-4.5 \"build a TODO app\"\n")
	}
	flag.Parse()

	repoRoot, err := os.Getwd()
	if err != nil {
		fatal("cannot determine working directory: %v", err)
	}

	var prompts []promptItem
	if *battery || *only != "" {
		prompts, err = loadBatteryPrompts(repoRoot)
		if err != nil {
			fatal("loading battery prompts: %v", err)
		}
		if len(prompts) == 0 {
			fatal("no prompts found in eval/prompts/")
		}
		// --only filter: keep prompts whose name matches one of the comma-separated entries.
		if *only != "" {
			want := map[string]bool{}
			for _, n := range strings.Split(*only, ",") {
				n = strings.TrimSpace(n)
				if n != "" {
					want[n] = true
				}
			}
			filtered := make([]promptItem, 0, len(prompts))
			for _, p := range prompts {
				if want[p.name] {
					filtered = append(filtered, p)
				}
			}
			if len(filtered) == 0 {
				fatal("--only filter matched no prompts (looked for %d names; have %d total)", len(want), len(prompts))
			}
			prompts = filtered
		}
		if *runs > 0 && *runs < len(prompts) {
			prompts = prompts[:*runs]
		}
	} else {
		if flag.NArg() < 1 || strings.TrimSpace(flag.Arg(0)) == "" {
			flag.Usage()
			os.Exit(1)
		}
		prompts = []promptItem{{name: "custom", content: flag.Arg(0)}}
	}

	runLabel := *label
	if runLabel == "" {
		runLabel = time.Now().Format("2006-01-02-150405")
	}

	type runResult struct {
		dir string
		err error
	}

	totalRuns := len(prompts) * *repeat
	results := make([]runResult, totalRuns)
	var wg sync.WaitGroup

	for i, prompt := range prompts {
		for j := 0; j < *repeat; j++ {
			idx := i**repeat + j
			wg.Add(1)
			go func(idx int, p promptItem) {
				defer wg.Done()
				rl := runLabel
				if totalRuns > 1 {
					rl = fmt.Sprintf("%s-r%d", runLabel, idx)
				}
				pipe := NewPipeline(repoRoot, rl, p.name, *subjectModel, *judgeModel)
				err := pipe.Run(p.content, *dryRun, *skipJudge)
				results[idx] = runResult{dir: pipe.RunDir(), err: err}
			}(idx, prompt)
		}
	}
	wg.Wait()

	var runDirs []string
	for i, r := range results {
		if r.err != nil {
			fatal("run %d failed: %v", i, r.err)
		}
		runDirs = append(runDirs, r.dir)
	}

	if err := synthesize(repoRoot, runLabel, runDirs, *judgeModel, *dryRun, *skipJudge); err != nil {
		fatal("synthesize failed: %v", err)
	}
}

func loadBatteryPrompts(root string) ([]promptItem, error) {
	dir := filepath.Join(root, "eval", "prompts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var prompts []promptItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if strings.HasSuffix(e.Name(), "-judge.md") {
			continue // judge templates, not subject prompts
		}
		if strings.HasSuffix(e.Name(), "-subject.md") {
			continue // subject wrapper templates, not task prompts
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		prompts = append(prompts, promptItem{name: name, content: string(data)})
	}
	return prompts, nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "eval: "+format+"\n", args...)
	os.Exit(1)
}
