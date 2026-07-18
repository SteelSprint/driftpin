package output

import (
	"encoding/json"
	"strconv"
	"strings"

	"drift/core"
	"drift/internal/diff"
	"drift/orchestrator"
)

// D! id=ojson range-start
// JSONPresenter serializes each Result as a JSON object. Field order is
// deterministic (struct-defined). Output is a single JSON object with no
// trailing newline (main.go adds the newline). JSON output never contains
// ANSI sequences.
type JSONPresenter struct{}

// marshal serializes v to JSON with HTML escaping disabled for readability.
func marshal(v interface{}) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(b.String(), "\n")
}

// --- Todo ---

type jsonTodo struct {
	Ok              bool           `json:"ok"`
	Specs           int            `json:"specs"`
	Markers         int            `json:"markers"`
	Edges           int            `json:"edges"`
	Todos           []jsonTodoItem `json:"todos"`
	UnlinkedMarkers int            `json:"unlinkedMarkers"`
}

type jsonTodoItem struct {
	Kind           string `json:"kind"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	SourceSpec     string `json:"sourceSpec,omitempty"`
	FromLocation   string `json:"fromLocation,omitempty"`
	ToLocation     string `json:"toLocation,omitempty"`
	SourceLocation string `json:"sourceLocation,omitempty"`
	FromChanged    bool   `json:"fromChanged,omitempty"`
	ToChanged      bool   `json:"toChanged,omitempty"`
	FromDeleted    bool   `json:"fromDeleted,omitempty"`
	ToDeleted      bool   `json:"toDeleted,omitempty"`
}

func todoKindString(k core.TodoKind) string {
	switch k {
	case core.TodoEdgeDrift:
		return "edgeDrift"
	case core.TodoCascade:
		return "cascade"
	case core.TodoEdgeAdded:
		return "edgeAdded"
	case core.TodoEdgeRemoved:
		return "edgeRemoved"
	case core.TodoBrokenEdge:
		return "brokenEdge"
	default:
		return "unknown"
	}
}

func locationOrEmpty(filepath string, line int) string {
	if filepath == "" && line == 0 {
		return ""
	}
	if filepath == "" {
		return ":" + strconv.Itoa(line)
	}
	return filepath + ":" + strconv.Itoa(line)
}

func (p JSONPresenter) Todo(r TodoResult) string {
	state := r.State
	items := make([]jsonTodoItem, 0, len(state.Todos))
	for _, t := range state.Todos {
		items = append(items, jsonTodoItem{
			Kind:           todoKindString(t.Kind),
			From:           t.From,
			To:             t.To,
			SourceSpec:     t.SourceSpecID,
			FromLocation:   locationOrEmpty(t.FromFilepath, t.FromLineNumber),
			ToLocation:     locationOrEmpty(t.ToFilepath, t.ToLineNumber),
			SourceLocation: locationOrEmpty(t.SourceFilepath, 0),
			FromChanged:    t.FromChanged,
			ToChanged:      t.ToChanged,
			FromDeleted:    t.FromDeleted,
			ToDeleted:      t.ToDeleted,
		})
	}
	out := jsonTodo{
		Ok:              len(state.Todos) == 0 && (len(state.Specs) > 0 || len(state.Markers) > 0),
		Specs:           len(state.Specs),
		Markers:         len(state.Markers),
		Edges:           len(state.Edges),
		Todos:           items,
		UnlinkedMarkers: countUnlinkedMarkers(state),
	}
	return marshal(out)
}

func countUnlinkedMarkers(state core.EvaluatedState) int {
	linked := make(map[string]bool)
	for _, e := range state.Edges {
		if isSpecID(e.From) {
			continue
		}
		linked[e.From] = true
	}
	n := 0
	for _, m := range state.Markers {
		if !m.Deleted && !linked[m.ID] {
			n++
		}
	}
	return n
}

// --- List ---

type jsonList struct {
	Specs   []jsonListSpec   `json:"specs"`
	Markers []jsonListMarker `json:"markers"`
	Edges   []jsonListEdge   `json:"edges"`
}

type jsonListSpec struct {
	ID       string `json:"id"`
	Filepath string `json:"filepath"`
	Deleted  bool   `json:"deleted"`
	Unlinked bool   `json:"unlinked"`
	Text     string `json:"text,omitempty"`
}

type jsonListMarker struct {
	ID       string `json:"id"`
	Filepath string `json:"filepath"`
	StartLine int   `json:"startLine"`
	EndLine   int   `json:"endLine"`
	Deleted  bool   `json:"deleted"`
	Unlinked bool   `json:"unlinked"`
	Preview  string `json:"preview,omitempty"`
}

type jsonListEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Status string `json:"status"`
}

func (p JSONPresenter) List(r ListResult) string {
	state := r.State
	drifted := make(map[string]bool)
	for _, t := range state.Todos {
		if t.Kind == core.TodoEdgeDrift && t.SourceSpecID == "" {
			drifted[t.From+"\x00"+t.To] = true
		}
	}
	linkedSpecs := make(map[string]bool)
	linkedMarkers := make(map[string]bool)
	for _, e := range state.Edges {
		if isSpecID(e.From) {
			continue
		}
		linkedMarkers[e.From] = true
		linkedSpecs[e.To] = true
	}

	specs := make([]jsonListSpec, 0, len(state.Specs))
	for _, s := range state.Specs {
		entry := jsonListSpec{
			ID:       s.ID,
			Filepath: s.Filepath,
			Deleted:  s.Deleted,
			Unlinked: !s.Deleted && !linkedSpecs[s.ID],
		}
		if r.Verbose && !s.Deleted {
			if content, ok := r.SpecContents[s.ID]; ok {
				entry.Text = content
			}
		}
		specs = append(specs, entry)
	}

	markers := make([]jsonListMarker, 0, len(state.Markers))
	for _, m := range state.Markers {
		entry := jsonListMarker{
			ID:        m.ID,
			Filepath:  m.Filepath,
			StartLine: m.LineNumber,
			EndLine:   m.EndLineNumber,
			Deleted:   m.Deleted,
			Unlinked:  !m.Deleted && !linkedMarkers[m.ID],
		}
		if r.Verbose && !m.Deleted {
			if content, ok := r.MarkerContents[m.ID]; ok {
				firstLine := strings.Split(content, "\n")[0]
				if len(firstLine) > 80 {
					firstLine = firstLine[:80] + "..."
				}
				entry.Preview = firstLine
			}
		}
		markers = append(markers, entry)
	}

	edges := make([]jsonListEdge, 0, len(state.Edges))
	for _, e := range state.Edges {
		status := "synced"
		if drifted[e.From+"\x00"+e.To] || drifted[e.To+"\x00"+e.From] {
			status = "drifted"
		}
		edges = append(edges, jsonListEdge{From: e.From, To: e.To, Status: status})
	}

	return marshal(jsonList{Specs: specs, Markers: markers, Edges: edges})
}

// --- Show ---

func (p JSONPresenter) Show(r ShowResult) string {
	if r.IsSpec && r.Spec == nil {
		return marshal(jsonError{Error: "spec \"" + r.ID + "\" not found", Exit: 1})
	}
	if !r.IsSpec && r.Marker == nil {
		return marshal(jsonError{Error: "marker \"" + r.ID + "\" not found", Exit: 1})
	}

	if r.IsSpec {
		linked := make([]jsonLinked, 0, len(r.LinkedMarkers))
		for _, m := range r.LinkedMarkers {
			linked = append(linked, jsonLinked{
				Kind:     "marker",
				ID:       m.Marker.ID,
				Filepath: m.Marker.Filepath,
				Lines:    strconv.Itoa(m.Marker.LineNumber) + "-" + strconv.Itoa(m.Marker.EndLineNumber),
				Hash:     m.Marker.Hash,
				Content:  m.Content,
			})
		}
		return marshal(jsonShow{
			Kind: "spec", ID: r.ID, Filepath: r.Spec.Filepath,
			Hash: r.Spec.Hash, Content: r.Content, Linked: linked,
		})
	}

	linked := make([]jsonLinked, 0, len(r.LinkedSpecs))
	for _, s := range r.LinkedSpecs {
		linked = append(linked, jsonLinked{
			Kind:     "spec",
			ID:       s.Spec.ID,
			Filepath: s.Spec.Filepath,
			Hash:     s.Spec.Hash,
			Content:  s.Content,
		})
	}
	return marshal(jsonShow{
		Kind: "marker", ID: r.ID, Filepath: r.Marker.Filepath,
		Lines:   strconv.Itoa(r.Marker.LineNumber) + "-" + strconv.Itoa(r.Marker.EndLineNumber),
		Hash:    r.Marker.Hash,
		Content: r.Content,
		Linked:  linked,
	})
}

type jsonShow struct {
	Kind    string       `json:"kind"`
	ID      string       `json:"id"`
	Filepath string      `json:"filepath"`
	Lines   string       `json:"lines,omitempty"`
	Hash    string       `json:"hash"`
	Content string       `json:"content"`
	Linked  []jsonLinked `json:"linked"`
}

type jsonLinked struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Filepath string `json:"filepath"`
	Lines    string `json:"lines,omitempty"`
	Hash     string `json:"hash"`
	Content  string `json:"content"`
}

// --- Diff ---

type jsonDiffEdge struct {
	Spec   jsonDiffSide `json:"spec"`
	Marker jsonDiffSide `json:"marker"`
}

type jsonDiffSide struct {
	ID           string `json:"id"`
	Filepath     string `json:"filepath,omitempty"`
	Lines        string `json:"lines,omitempty"`
	BaselineHash string `json:"baselineHash"`
	CurrentHash  string `json:"currentHash"`
	HasBaseline  bool   `json:"hasBaseline"`
	Deleted      bool   `json:"deleted"`
	Baseline     string `json:"baseline,omitempty"`
	Current      string `json:"current,omitempty"`
	Patch        string `json:"patch,omitempty"`
}

func diffSideToJSON(side orchestrator.DiffSide) jsonDiffSide {
	out := jsonDiffSide{
		ID:           side.ID,
		Filepath:     side.Filepath,
		Lines:        side.Lines,
		BaselineHash: side.BaselineHash,
		CurrentHash:  side.CurrentHash,
		HasBaseline:  side.HasBaseline,
		Deleted:      side.Deleted,
		Baseline:     side.Baseline,
		Current:      side.Current,
	}
	if side.HasBaseline && side.Baseline != side.Current {
		out.Patch = diff.UnifiedDiff(side.Baseline, side.Current)
	}
	return out
}

func (p JSONPresenter) DiffEdge(r DiffEdgeResult) string {
	return marshal(jsonDiffEdge{
		Spec:   diffSideToJSON(r.Result.Spec),
		Marker: diffSideToJSON(r.Result.Marker),
	})
}

type jsonDiffExpanded struct {
	ID    string        `json:"id,omitempty"`
	Edges []jsonDiffEdge `json:"edges"`
}

func edgesToJSON(edges []orchestrator.DiffResult) []jsonDiffEdge {
	out := make([]jsonDiffEdge, 0, len(edges))
	for _, e := range edges {
		out = append(out, jsonDiffEdge{
			Spec:   diffSideToJSON(e.Spec),
			Marker: diffSideToJSON(e.Marker),
		})
	}
	return out
}

func (p JSONPresenter) DiffExpanded(r DiffExpandedResult) string {
	return marshal(jsonDiffExpanded{ID: r.ID, Edges: edgesToJSON(r.Edges)})
}

func (p JSONPresenter) DiffAll(r DiffAllResult) string {
	return marshal(jsonDiffExpanded{Edges: edgesToJSON(r.Edges)})
}

// --- Ok / Error / Text / Version ---

type jsonOk struct {
	Ok      bool   `json:"ok"`
	Command string `json:"command"`
	Message string `json:"message"`
}

func (p JSONPresenter) Ok(r OkResult) string {
	return marshal(jsonOk{Ok: true, Command: r.Command, Message: r.Message})
}

type jsonError struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
	Hint  string `json:"hint,omitempty"`
	Exit  int    `json:"exit"`
}

func (p JSONPresenter) Error(r ErrorResult) string {
	return marshal(jsonError{Ok: false, Error: r.Message, Hint: r.Hint, Exit: r.Exit})
}

type jsonText struct {
	Text string `json:"text"`
}

func (p JSONPresenter) Text(r TextResult) string {
	return marshal(jsonText{Text: r.Text})
}

type jsonVersion struct {
	Version string `json:"version"`
}

func (p JSONPresenter) Version(r VersionResult) string {
	return marshal(jsonVersion{Version: r.Version})
}
// D! id=ojson range-end
