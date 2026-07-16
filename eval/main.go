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
		runs         = flag.Int("runs", 2, "number of battery prompts to run (capped by available prompts)")
		label        = flag.String("label", "", "label for this run (defaults to timestamp)")
		dryRun       = flag.Bool("dry-run", false, "stage only, skip LLM calls")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run ./eval [flags] [prompt]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval \"create a working CLI version of poker\"\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval --battery              # run first 2 prompts in parallel\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval --battery --runs 3     # run first 3 prompts in parallel\n")
		fmt.Fprintf(os.Stderr, "  go run ./eval --subject openrouter/anthropic/claude-haiku-4.5 \"build a TODO app\"\n")
	}
	flag.Parse()

	repoRoot, err := os.Getwd()
	if err != nil {
		fatal("cannot determine working directory: %v", err)
	}

	var prompts []promptItem
	if *battery {
		prompts, err = loadBatteryPrompts(repoRoot)
		if err != nil {
			fatal("loading battery prompts: %v", err)
		}
		if len(prompts) == 0 {
			fatal("no prompts found in eval/prompts/")
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

	results := make([]runResult, len(prompts))
	var wg sync.WaitGroup

	for i, prompt := range prompts {
		wg.Add(1)
		go func(idx int, p promptItem) {
			defer wg.Done()
			rl := runLabel
			if len(prompts) > 1 {
				rl = fmt.Sprintf("%s-%d", runLabel, idx)
			}
			pipe := NewPipeline(repoRoot, rl, p.name, *subjectModel, *judgeModel)
			err := pipe.Run(p.content, *dryRun)
			results[idx] = runResult{dir: pipe.RunDir(), err: err}
		}(i, prompt)
	}
	wg.Wait()

	var runDirs []string
	for i, r := range results {
		if r.err != nil {
			fatal("run %d failed: %v", i, r.err)
		}
		runDirs = append(runDirs, r.dir)
	}

	if err := synthesize(repoRoot, runLabel, runDirs, *judgeModel, *dryRun); err != nil {
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
