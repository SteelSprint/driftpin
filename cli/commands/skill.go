package commands

import "drift/cli/output"

// SkillCommand implements `drift skill`. The skill guide is injected by the
// Registry from the embedded skill.md.
type SkillCommand struct {
	Text string // full skill guide, injected by Registry
}

// D! id=cskill range-start
func (c SkillCommand) Run(ctx Context) (output.Result, int) {
	return output.TextResult{Text: c.Text}, 0
}

// D! id=cskill range-end
func (c SkillCommand) Meta() Meta {
	return Meta{
		Name:  "skill",
		Short: "Print comprehensive guide (for LLM agents)",
		Usage: "Usage: drift skill\n\nPrint the comprehensive drift guide for LLM agents: workflow, spec file format,\nmarker syntax and range hashing model, CLI command table, drift detection model,\n.drift/ directory layout, and edge cases.\n\nNo arguments.",
	}
}
