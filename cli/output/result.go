// Package output implements the presenter layer of the drift CLI.
//
// File layout (see output_impl.package_layout):
//   result.go        — Result interface and concrete types (TodoResult, etc.)
//   presenter.go     — Presenter interface + Render dispatcher
//   plain.go         — PlainPresenter (no ANSI, no JSON)
//   color.go         — ColorPresenter (theme-driven ANSI)
//   json.go          — JSONPresenter (structured output for LLM consumption)
//   ansi.go          — ANSI SGR constants and helpers
//   tty.go           — IsTerminal + SelectPresenter
//   theme.go         — Style, Theme types and Apply
//   themes.go        — 12 built-in themes
//   theme_load.go    — LoadCustomTheme from .drift/theme.xml
//   user_settings.go — LoadUserSettings / SaveUserSettings (.drift/user-settings.xml)
//   tokenize.go      — Language-agnostic code tokenizer (for syntax highlighting)
//   build.go         — Result constructors (BuildListResult, BuildShowResult, etc.)
//
// D! id=opkg range-start
package output

import (
	"drift/core"
	"drift/orchestrator"
)

// D! id=opkg range-end

// D! id=ores range-start
// Result is a sealed interface implemented by exactly the types below. Each
// command path in cli.RunWithRender produces one Result; the selected Presenter
// renders it to a string. Presenters never do command dispatch, I/O, or state
// mutation — they only format what the Result carries.
type Result interface {
	isResult()
}

// TodoResult carries the evaluated state for `drift todo`.
type TodoResult struct {
	State core.EvaluatedState
}

// ListResult carries state plus pre-resolved content for verbose previews.
// When Verbose is false, SpecContents and MarkerPreviews are nil and the
// presenter skips previews entirely.
type ListResult struct {
	State          core.EvaluatedState
	Verbose        bool
	SpecContents   map[string]string
	MarkerContents map[string]string
}

// ShowResult carries the resolved entity (spec or marker), its content, and
// any linked counterparts with their content. Outbound/Inbound refs are
// computed from baseline spec-spec edges for the spec view.
type ShowResult struct {
	IsSpec        bool
	ID            string
	Spec          *core.Spec
	Marker        *core.Marker
	Content       string
	LinkedSpecs   []LinkedSpec
	LinkedMarkers []LinkedMarker
	OutboundRefs  []string
	InboundRefs   []string
}

// LinkedSpec carries a spec linked to the primary marker plus its content.
type LinkedSpec struct {
	Spec    core.Spec
	Content string
}

// LinkedMarker carries a marker linked to the primary spec plus its content.
type LinkedMarker struct {
	Marker  core.Marker
	Content string
}

// DiffClosureResult carries the diff results for all nodes in one closure.
type DiffClosureResult struct {
	Hash  string
	Diffs []orchestrator.DiffResult
}

// DiffAllResult carries per-closure diff results alongside the evaluated state.
type DiffAllResult struct {
	State    core.EvaluatedState
	Closures []orchestrator.ClosureDiff
}

// ChangeSummaryResult carries a ChangeSummary for dry-run preview or
// post-apply printing. When Preview is true, presenters render a
// "Preview — no changes written" banner; otherwise they render the
// summary as the body following any lead-in message in Message.
type ChangeSummaryResult struct {
	Summary  orchestrator.ChangeSummary
	Preview  bool   // true for dry-run; false for post-apply
	Message  string // optional lead-in (e.g. "Closure a3f7b2c1 resolved.")
}

// OkResult is a generic success message for commands that don't produce
// structured data (init, link, unlink, reset).
type OkResult struct {
	Command string
	Message string
}

// ErrorResult carries a structured error.
type ErrorResult struct {
	Command string
	Message string
	Hint    string
	Exit    int
}

// TextResult is a passthrough for embedded prose (help, skill).
type TextResult struct {
	Text string
}

// VersionResult carries the version string for `drift version`.
type VersionResult struct {
	Version string
}

func (TodoResult) isResult()         {}
func (ListResult) isResult()         {}
func (ShowResult) isResult()         {}
func (DiffClosureResult) isResult()  {}
func (DiffAllResult) isResult()      {}
func (ChangeSummaryResult) isResult() {}
func (OkResult) isResult()           {}
func (ErrorResult) isResult()        {}
func (TextResult) isResult()         {}
func (VersionResult) isResult()      {}
// D! id=ores range-end
