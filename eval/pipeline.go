package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const (
	subjectTimeout = 30 * time.Minute
	judgeTimeout   = 15 * time.Minute
)

var stdoutMu sync.Mutex

type Pipeline struct {
	repoRoot     string
	runLabel     string
	fixtureName  string
	runDir       string
	workspaceDir string
	subjectModel string
	judgeModel   string
}

func NewPipeline(repoRoot, label, fixtureName, subjectModel, judgeModel string) *Pipeline {
	return &Pipeline{
		repoRoot:     repoRoot,
		runLabel:     label,
		fixtureName:  fixtureName,
		subjectModel: subjectModel,
		judgeModel:   judgeModel,
	}
}

func (p *Pipeline) Run(prompt string, dryRun bool) error {
	p.printf("=== eval run ===\n")
	p.printf("subject model: %s\n", p.subjectModel)
	p.printf("judge model:   %s\n", p.judgeModel)

	if err := p.stage(); err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	p.printf("[stage] done → %s\n", p.runDir)

	if dryRun {
		p.println("[dry-run] skipping LLM calls")
		return nil
	}

	if err := p.runSubject(prompt); err != nil {
		return fmt.Errorf("subject: %w", err)
	}
	p.println("[subject] done")

	if err := p.runJudge(prompt); err != nil {
		return fmt.Errorf("judge: %w", err)
	}
	p.println("[judge] done")

	if err := p.surface(); err != nil {
		return fmt.Errorf("surface: %w", err)
	}

	return nil
}

func (p *Pipeline) printf(format string, args ...any) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	fmt.Printf("["+p.runLabel+"] "+format, args...)
}

func (p *Pipeline) println(args ...any) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	fmt.Print("[" + p.runLabel + "] ")
	fmt.Println(args...)
}

func (p *Pipeline) stage() error {
	p.runDir = filepath.Join(p.repoRoot, "eval", "runs", p.runLabel)
	p.workspaceDir = filepath.Join("/tmp/opencode/eval-runs", p.runLabel, "workspace")

	for _, dir := range []string{p.runDir, p.workspaceDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	if err := p.buildBinary(); err != nil {
		return err
	}

	if err := p.copyFixture(); err != nil {
		return fmt.Errorf("copy fixture: %w", err)
	}

	subjectAgentsDir := filepath.Join(p.workspaceDir, ".opencode", "agents")
	if err := os.MkdirAll(subjectAgentsDir, 0755); err != nil {
		return err
	}
	if err := copyFile(
		filepath.Join(p.repoRoot, "eval", "agents", "eval-subject.md"),
		filepath.Join(subjectAgentsDir, "eval-subject.md"),
	); err != nil {
		return fmt.Errorf("stage subject agent: %w", err)
	}

	judgeAgentsDir := filepath.Join(p.runDir, ".opencode", "agents")
	if err := os.MkdirAll(judgeAgentsDir, 0755); err != nil {
		return err
	}
	if err := copyFile(
		filepath.Join(p.repoRoot, "eval", "agents", "eval-judge.md"),
		filepath.Join(judgeAgentsDir, "eval-judge.md"),
	); err != nil {
		return fmt.Errorf("stage judge agent: %w", err)
	}

	return nil
}

func (p *Pipeline) copyFixture() error {
	fixtureDir := filepath.Join(p.repoRoot, "eval", "prompts", p.fixtureName)
	info, err := os.Stat(fixtureDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	if err := copyDir(fixtureDir, p.workspaceDir); err != nil {
		return err
	}

	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func (p *Pipeline) buildBinary() error {
	binaryPath := filepath.Join(p.workspaceDir, "drift")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/drift")
	cmd.Dir = p.repoRoot
	cmd.Stdout = newMutexWriter(os.Stdout, p.runLabel)
	cmd.Stderr = newMutexWriter(os.Stderr, p.runLabel)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return os.Chmod(binaryPath, 0755)
}

func (p *Pipeline) runSubject(prompt string) error {
	subjectOut, err := os.Create(filepath.Join(p.runDir, "subject.jsonl"))
	if err != nil {
		return err
	}
	defer subjectOut.Close()

	fullPrompt := buildSubjectPrompt(prompt)

	return p.runOpencode(&opencodeArgs{
		agent:   "eval-subject",
		model:   p.subjectModel,
		dir:     p.workspaceDir,
		title:   "subject",
		prompt:  fullPrompt,
		stdout:  subjectOut,
		timeout: subjectTimeout,
	})
}

func (p *Pipeline) runJudge(originalPrompt string) error {
	judgeOut, err := os.Create(filepath.Join(p.runDir, "judge.jsonl"))
	if err != nil {
		return err
	}
	defer judgeOut.Close()

	judgePrompt := p.resolveJudgePrompt(originalPrompt)

	return p.runOpencode(&opencodeArgs{
		agent:   "eval-judge",
		model:   p.judgeModel,
		dir:     p.runDir,
		title:   "judge",
		prompt:  judgePrompt,
		stdout:  judgeOut,
		timeout: judgeTimeout,
	})
}

// resolveJudgePrompt checks for a prompt-specific judge template at
// eval/prompts/<fixtureName>-judge.md. If found, substitutes placeholders
// and returns the specialized prompt. Otherwise falls back to the generic
// buildJudgePrompt.
func (p *Pipeline) resolveJudgePrompt(originalPrompt string) string {
	templatePath := filepath.Join(p.repoRoot, "eval", "prompts", p.fixtureName+"-judge.md")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return buildJudgePrompt(originalPrompt, p.workspaceDir, p.runDir)
	}
	fixtureDir := filepath.Join(p.repoRoot, "eval", "prompts", p.fixtureName)
	prompt := string(data)
	prompt = replaceAll(prompt, "{{TASK}}", originalPrompt)
	prompt = replaceAll(prompt, "{{WORKSPACE}}", p.workspaceDir)
	prompt = replaceAll(prompt, "{{FIXTURE_DIR}}", fixtureDir)
	prompt = replaceAll(prompt, "{{RUN_DIR}}", p.runDir)
	return prompt
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func (p *Pipeline) surface() error {
	reportPath := filepath.Join(p.runDir, "report.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("read report.md: %w", err)
	}
	stdoutMu.Lock()
	fmt.Printf("\n[%s] === REPORT ===\n", p.runLabel)
	fmt.Println(string(data))
	stdoutMu.Unlock()

	logPath := filepath.Join(p.repoRoot, "eval", "runs", "log.csv")
	row := fmt.Sprintf("%s,%q,%s,%s,%s\n",
		p.runLabel,
		"see report",
		p.runDir,
		p.subjectModel,
		p.judgeModel,
	)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log.csv: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(row)
	return err
}

type opencodeArgs struct {
	agent   string
	model   string
	dir     string
	title   string
	prompt  string
	stdout  io.Writer
	timeout time.Duration
	label   string
}

func (p *Pipeline) runOpencode(a *opencodeArgs) error {
	a.label = p.runLabel
	return runOpencodeStandalone(a)
}

func buildSubjectPrompt(task string) string {
	return fmt.Sprintf(`You are being evaluated on your ability to use a spec-drift tool called "drift".

A pre-built `+"`drift`"+` binary is in your working directory. It is executable. You have NO documentation, NO source code, and NO outside help — only the binary. Figure out how to use it by inspecting the binary (e.g. running it with no args, `+"`--help`"+`, `+"`drift skill`"+`, or trying subcommands).

%s

## What you must do

1. Figure out how the `+"`drift`"+` binary works by exploring it yourself.
2. Complete the task described above.
3. Write a file called `+"`self-debrief.md`"+` in your project root with these EXACT sections:
   - **What worked well**: What was easy or intuitive about using drift.
   - **What was confusing**: What was hard to understand or figure out.
   - **Errors encountered**: Any errors you hit and how you resolved them (or didn't).
   - **Missing documentation**: Things you needed to know that weren't discoverable from the binary alone.
   - **Suggestions for the tool authors**: Concrete improvements that would make drift easier for an LLM to use cold.

## Important

- Work autonomously. Do not ask questions. Make your best judgment.
- Your `+"`self-debrief.md`"+` is critical — it will be read by a judge LLM evaluating your work. Be thorough and honest.
`, task)
}

func buildJudgePrompt(originalTask, workspaceDir, runDir string) string {
	return fmt.Sprintf(`You are the JUDGE in an LLM-as-judge evaluation of a spec-drift tool called "drift".

## Context

A subject LLM was given a task and asked to use drift (a spec-drift tool) end-to-end while completing it. The subject received ONLY a pre-built `+"`drift`"+` binary and the task prompt — no documentation, no source code. You must evaluate how well the subject used drift and how well the tool served the subject.

## Artifacts to inspect

1. **The original task prompt:**
   %s

2. **The subject's workspace** (its completed project): `+"`%s`"+`
   - The task prompt includes a "## Success criteria" section with specific outcomes. You MUST verify each one.
   - Run `+"`drift todo`"+` from inside the workspace to check sync state.
   - Run `+"`drift list`"+` from inside the workspace to inspect specs, markers, and links.
   - Check `+"`*.drift.xml`"+` files — are specs meaningful?
   - Check markers (`+"`D! id=...`"+`) in code — are they placed at meaningful locations?
   - Read `+"`self-debrief.md`"+` — the subject's own feedback.

3. **The subject's transcript** (JSONL of its session): `+"`%s/subject.jsonl`"+`
   - Sample it for confusion, tool misuse, or errors. You don't need to read every line — focus on moments where the subject struggled.

## What you must produce

Write a file called `+"`report.md`"+` in the run directory (your current working directory) with these EXACT sections:

### 1. Scorecard

Rate EACH success criterion from the task prompt as PASS or FAIL with a one-line note. List them by number, matching the task prompt's "## Success criteria" section. Then rate these universal criteria:
- Used drift commands correctly (no tool misuse, correct syntax)
- Self-debrief.md quality (thorough, honest, actionable feedback)

### 2. Qualitative Assessment

3-5 paragraphs covering:
- How well did the subject understand and use drift?
- What patterns of confusion or success did you see?
- Did the subject's `+"`self-debrief.md`"+` reveal any UX problems?
- Was the binary self-describing enough for cold use?

### 3. Tool-Improvement Recommendations

A PRIORITIZED list of concrete, actionable improvements to drift, ordered by impact:
1. [High/Medium/Low] <recommendation> — <reasoning>
2. ...

These recommendations will be triaged into the tool's development plan, so be specific and practical.

## Constraints

- You may read any file in the workspace or run directory.
- You may run bash commands (e.g., `+"`drift todo`"+`, `+"`drift list`"+`, `+"`go build`"+`) to verify the subject's work. The `+"`drift`"+` binary is in the workspace directory.
- You may ONLY write to `+"`report.md`"+` — do not modify any other file.
- Be rigorous and fair. Don't inflate scores.
`, originalTask, workspaceDir, runDir)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func (p *Pipeline) RunDir() string {
	return p.runDir
}

func synthesize(repoRoot, batchLabel string, runDirs []string, judgeModel string, dryRun bool) error {
	stdoutMu.Lock()
	fmt.Printf("\n=== synthesis: %s ===\n", batchLabel)
	fmt.Printf("runs: %d\n", len(runDirs))
	fmt.Printf("judge model:   %s\n", judgeModel)
	stdoutMu.Unlock()

	if dryRun {
		stdoutMu.Lock()
		fmt.Println("[dry-run] skipping synthesis LLM call")
		stdoutMu.Unlock()
		return nil
	}

	synthesisRunDir := filepath.Join(repoRoot, "eval", "runs", batchLabel+"-synthesis")
	if err := os.MkdirAll(synthesisRunDir, 0755); err != nil {
		return err
	}

	synthesisAgentsDir := filepath.Join(synthesisRunDir, ".opencode", "agents")
	if err := os.MkdirAll(synthesisAgentsDir, 0755); err != nil {
		return err
	}
	if err := copyFile(
		filepath.Join(repoRoot, "eval", "agents", "eval-judge.md"),
		filepath.Join(synthesisAgentsDir, "eval-judge.md"),
	); err != nil {
		return fmt.Errorf("stage synthesis judge agent: %w", err)
	}

	synthesisOut, err := os.Create(filepath.Join(synthesisRunDir, "synthesis.jsonl"))
	if err != nil {
		return err
	}
	defer synthesisOut.Close()

	prompt := buildSynthesisPrompt(runDirs, batchLabel)

	if err := runOpencodeStandalone(&opencodeArgs{
		agent:   "eval-judge",
		model:   judgeModel,
		dir:     synthesisRunDir,
		title:   "synthesis",
		prompt:  prompt,
		stdout:  synthesisOut,
		timeout: judgeTimeout,
		label:   batchLabel,
	}); err != nil {
		return fmt.Errorf("synthesis opencode run: %w", err)
	}
	stdoutMu.Lock()
	fmt.Println("[synthesis] done")
	stdoutMu.Unlock()

	synthesisPath := filepath.Join(synthesisRunDir, "synthesis.md")
	if _, err := os.Stat(synthesisPath); err != nil {
		return fmt.Errorf("synthesis.md not written by judge: %w", err)
	}

	obsDir := filepath.Join(repoRoot, "observations")
	if err := os.MkdirAll(obsDir, 0755); err != nil {
		return err
	}
	num := nextObservationNumber(obsDir)
	obsPath := filepath.Join(obsDir, fmt.Sprintf("%04d-%s.md", num, batchLabel))
	if err := copyFile(synthesisPath, obsPath); err != nil {
		return fmt.Errorf("file observation: %w", err)
	}
	stdoutMu.Lock()
	fmt.Printf("[observation] filed → %s\n", obsPath)
	stdoutMu.Unlock()

	return nil
}

func nextObservationNumber(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	maxNum := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var num int
		_, err := fmt.Sscanf(e.Name(), "%d-", &num)
		if err != nil {
			continue
		}
		if num > maxNum {
			maxNum = num
		}
	}
	return maxNum + 1
}

func runOpencodeStandalone(a *opencodeArgs) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()

	args := []string{
		"run",
		"--model", a.model,
		"--auto",
		"--format", "json",
		"--dir", a.dir,
		"--title", a.title,
	}
	if a.agent != "" {
		args = append(args, "--agent", a.agent)
	}
	args = append(args, a.prompt)

	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Stdout = a.stdout
	cmd.Stderr = newMutexWriter(os.Stderr, a.label)

	agentLabel := a.agent
	if agentLabel == "" {
		agentLabel = "build (default)"
	}
	stdoutMu.Lock()
	fmt.Printf("[%s] [%s] running opencode (agent=%s model=%s dir=%s timeout=%v)\n",
		a.label, a.title, agentLabel, a.model, a.dir, a.timeout)
	stdoutMu.Unlock()
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("opencode timed out after %v", a.timeout)
		}
		return fmt.Errorf("opencode run: %w", err)
	}
	return nil
}

type mutexWriter struct {
	mu    *sync.Mutex
	w     io.Writer
	label string
}

func newMutexWriter(w io.Writer, label string) *mutexWriter {
	return &mutexWriter{mu: &stdoutMu, w: w, label: label}
}

func (mw *mutexWriter) Write(p []byte) (int, error) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	fmt.Fprintf(mw.w, "[%s] ", mw.label)
	return mw.w.Write(p)
}

func buildSynthesisPrompt(runDirs []string, batchLabel string) string {
	var reportPaths []string
	for _, dir := range runDirs {
		reportPaths = append(reportPaths, filepath.Join(dir, "report.md"))
	}

	var pathsList string
	for i, p := range reportPaths {
		pathsList += fmt.Sprintf("   %d. `%s`\n", i+1, p)
	}

	return fmt.Sprintf(`You are the JUDGE in an LLM-as-judge evaluation of a spec-drift tool called "drift".

You previously evaluated %d subject run(s) in batch %q. Each run produced a `+"`report.md`"+`. Your job now is to synthesize all reports into a single cross-run observation record.

## Reports to read

%s

## What you must produce

Write a file called `+"`synthesis.md`"+` in your current working directory with this EXACT format:

# Observation NNNN — %s

Date: <today's date>
Runs: <list of the run directories>

## Known issues

Any harness issues, sandbox escapes, tainted runs, or methodology problems discovered. If a run was compromised, note it here and exclude its findings from the convergent analysis.

## Convergent findings

A markdown table of themes that appeared across multiple runs:

| Theme | Runs | Priority |
|---|---|---|
| <theme> | <run numbers> | <High/Medium/Low> |

Only include themes that appeared in 2+ runs (or all runs if only 1 run). Single-run findings go in "Divergent findings" below.

## Divergent findings

Run-specific observations that didn't converge across runs. Note which run each came from.

## Prioritized recommendations (consolidated)

A merged, deduplicated, prioritized list of all tool-improvement recommendations from all runs:
1. [High/Medium/Low] <recommendation> — <which runs flagged this>
2. ...

## Next steps

The judge's recommendation for what the tool authors should do next, based on the consolidated findings.

## Constraints

- You may ONLY write to `+"`synthesis.md`"+` — do not modify any other file.
- Read each report.md thoroughly before writing.
- If a run was tainted (e.g. sandbox escape), note it in "Known issues" and exclude its findings from convergence, but still note what happened.
`, len(runDirs), batchLabel, pathsList, batchLabel)
}
