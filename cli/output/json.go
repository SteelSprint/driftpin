package output

import (
	"encoding/json"
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
	Ok              bool            `json:"ok"`
	Specs           int             `json:"specs"`
	Markers         int             `json:"markers"`
	Edges           int             `json:"edges"`
	Closures        []jsonClosure   `json:"closures"`
	UnlinkedMarkers int             `json:"unlinkedMarkers"`
}

type jsonClosure struct {
	Hash   string          `json:"hash"`
	Nodes  []jsonNodeRef   `json:"nodes"`
	Edges  []jsonClosureEdge `json:"edges"`
	Events []jsonEvent     `json:"events"`
}

type jsonNodeRef struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Filepath string `json:"filepath,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type jsonClosureEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type jsonEvent struct {
	Kind     string          `json:"kind"`
	NodeID   string          `json:"nodeId,omitempty"`
	Edge     *jsonClosureEdge `json:"edge,omitempty"`
	OldHash  string          `json:"oldHash,omitempty"`
	NewHash  string          `json:"newHash,omitempty"`
	Seed     string          `json:"seed"`
}

func eventKindString(k core.EventKind) string {
	switch k {
	case core.EventNodeChanged:
		return "nodeChanged"
	case core.EventNodeAdded:
		return "nodeAdded"
	case core.EventNodeRemoved:
		return "nodeRemoved"
	case core.EventEdgeAdded:
		return "edgeAdded"
	case core.EventEdgeRemoved:
		return "edgeRemoved"
	case core.EventEdgeBroken:
		return "brokenEdge"
	default:
		return "unknown"
	}
}

func (p JSONPresenter) Todo(r TodoResult) string {
	state := r.State
	closures := make([]jsonClosure, 0, len(state.Closures))
	for _, c := range state.Closures {
		closures = append(closures, closureToJSON(c))
	}
	out := jsonTodo{
		Ok:              len(state.Closures) == 0 && (len(state.Specs) > 0 || len(state.Markers) > 0),
		Specs:           len(state.Specs),
		Markers:         len(state.Markers),
		Edges:           len(state.Edges),
		Closures:        closures,
		UnlinkedMarkers: countUnlinkedMarkers(state),
	}
	return marshal(out)
}

func closureToJSON(c core.Closure) jsonClosure {
	nodes := make([]jsonNodeRef, 0, len(c.Nodes))
	for _, n := range c.Nodes {
		kind := "marker"
		if n.IsSpec {
			kind = "spec"
		}
		nodes = append(nodes, jsonNodeRef{
			ID:       n.ID,
			Kind:     kind,
			Filepath: n.Filepath,
			Line:     n.LineNumber,
		})
	}
	edges := make([]jsonClosureEdge, 0, len(c.Edges))
	for _, e := range c.Edges {
		edges = append(edges, jsonClosureEdge{From: e.From, To: e.To})
	}
	events := make([]jsonEvent, 0, len(c.Events))
	for _, ev := range c.Events {
		je := jsonEvent{
			Kind:    eventKindString(ev.Kind),
			NodeID:  ev.NodeID,
			OldHash: ev.OldHash,
			NewHash: ev.NewHash,
			Seed:    ev.Seed,
		}
		if ev.Edge != nil {
			je.Edge = &jsonClosureEdge{From: ev.Edge.From, To: ev.Edge.To}
		}
		events = append(events, je)
	}
	return jsonClosure{
		Hash:   c.Hash,
		Nodes:  nodes,
		Edges:  edges,
		Events: events,
	}
}

func countUnlinkedMarkers(state core.EvaluatedState) int {
	linked := make(map[string]bool)
	for _, e := range state.Edges {
		if isSpecIDOutput(e.From) {
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
	Drifted  bool   `json:"drifted"`
	Text     string `json:"text,omitempty"`
}

type jsonListMarker struct {
	ID        string `json:"id"`
	Filepath  string `json:"filepath"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Deleted   bool   `json:"deleted"`
	Unlinked  bool   `json:"unlinked"`
	Drifted   bool   `json:"drifted"`
	Preview   string `json:"preview,omitempty"`
}

type jsonListEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Status string `json:"status"`
}

func (p JSONPresenter) List(r ListResult) string {
	state := r.State
	driftedNodes := make(map[string]bool)
	for _, c := range state.Closures {
		for _, n := range c.Nodes {
			driftedNodes[n.ID] = true
		}
	}
	linkedSpecs := make(map[string]bool)
	linkedMarkers := make(map[string]bool)
	for _, e := range state.Edges {
		if isSpecIDOutput(e.From) {
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
			Drifted:  driftedNodes[s.ID],
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
			Drifted:   driftedNodes[m.ID],
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

	sortedEdges := append([]core.Edge(nil), state.Edges...)
	sortEdgesByFromTo(sortedEdges)
	edges := make([]jsonListEdge, 0, len(sortedEdges))
	for _, e := range sortedEdges {
		status := "synced"
		if driftedNodes[e.From] || driftedNodes[e.To] {
			status = "drifted"
		}
		edges = append(edges, jsonListEdge{From: e.From, To: e.To, Status: status})
	}

	return marshal(jsonList{Specs: specs, Markers: markers, Edges: edges})
}

// --- Show ---

func (p JSONPresenter) Show(r ShowResult) string {
	// Not-found: no nodes at all means the seed wasn't in state.
	if len(r.Nodes) == 0 {
		kind := "spec"
		if !r.IsSpec {
			kind = "marker"
		}
		return marshal(jsonError{Error: kind + " \"" + r.ID + "\" not found", Exit: 1})
	}

	nodes := make([]jsonShowNode, 0, len(r.Nodes))
	for _, n := range r.Nodes {
		nodes = append(nodes, jsonShowNode{
			Kind:     n.Kind,
			ID:       n.ID,
			Filepath: n.Filepath,
			Lines:    n.Lines,
			Hash:     n.Hash,
			Content:  n.Content,
			Deleted:  n.Deleted,
		})
	}
	edges := make([]jsonShowEdge, 0, len(r.Edges))
	for _, e := range r.Edges {
		edges = append(edges, jsonShowEdge{From: e.From, To: e.To})
	}

	specCount, markerCount, contentBytes := 0, 0, 0
	for _, n := range r.Nodes {
		if n.Kind == "spec" {
			specCount++
		} else {
			markerCount++
		}
		contentBytes += len(n.Content)
	}

	return marshal(jsonShowClosure{
		Seed:   r.ID,
		Nodes:  nodes,
		Edges:  edges,
		Summary: jsonShowSummary{
			NodeCount:        len(r.Nodes),
			EdgeCount:        len(r.Edges),
			SpecCount:        specCount,
			MarkerCount:      markerCount,
			TotalContentBytes: contentBytes,
		},
	})
}

type jsonShowClosure struct {
	Seed    string           `json:"seed"`
	Nodes   []jsonShowNode   `json:"nodes"`
	Edges   []jsonShowEdge   `json:"edges"`
	Summary jsonShowSummary  `json:"summary"`
}

type jsonShowNode struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Filepath string `json:"filepath"`
	Lines    string `json:"lines,omitempty"`
	Hash     string `json:"hash"`
	Content  string `json:"content,omitempty"`
	Deleted  bool   `json:"deleted,omitempty"`
}

type jsonShowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type jsonShowSummary struct {
	NodeCount         int `json:"nodeCount"`
	EdgeCount         int `json:"edgeCount"`
	SpecCount         int `json:"specCount"`
	MarkerCount       int `json:"markerCount"`
	TotalContentBytes int `json:"totalContentBytes"`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
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

type jsonDiffEntry struct {
	Kind   string        `json:"kind"`
	IsSeed bool          `json:"isSeed"`
	Role   string        `json:"role"` // "seed" or "citer"
	Spec   *jsonDiffSide `json:"spec,omitempty"`
	Marker *jsonDiffSide `json:"marker,omitempty"`
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

func diffEntryToJSON(d orchestrator.DiffResult) jsonDiffEntry {
	out := jsonDiffEntry{IsSeed: d.IsSeed}
	if d.IsSeed {
		out.Role = "seed"
	} else {
		out.Role = "citer"
	}
	if d.Spec != nil {
		out.Kind = "spec"
		s := diffSideToJSON(*d.Spec)
		out.Spec = &s
	} else if d.Marker != nil {
		out.Kind = "marker"
		s := diffSideToJSON(*d.Marker)
		out.Marker = &s
	}
	return out
}

type jsonDiffClosure struct {
	Hash   string          `json:"hash"`
	Events []jsonEvent     `json:"events"`
	Diffs  []jsonDiffEntry `json:"diffs"`
}

func (p JSONPresenter) DiffClosure(r DiffClosureResult) string {
	diffs := make([]jsonDiffEntry, 0, len(r.Diffs))
	for _, d := range r.Diffs {
		diffs = append(diffs, diffEntryToJSON(d))
	}
	return marshal(jsonDiffClosure{Hash: r.Hash, Diffs: diffs})
}

func (p JSONPresenter) DiffAll(r DiffAllResult) string {
	out := make([]jsonDiffClosure, 0, len(r.Closures))
	for _, c := range r.Closures {
		events := make([]jsonEvent, 0, len(c.Events))
		for _, ev := range c.Events {
			je := jsonEvent{
				Kind:    eventKindString(ev.Kind),
				NodeID:  ev.NodeID,
				OldHash: ev.OldHash,
				NewHash: ev.NewHash,
				Seed:    ev.Seed,
			}
			if ev.Edge != nil {
				je.Edge = &jsonClosureEdge{From: ev.Edge.From, To: ev.Edge.To}
			}
			events = append(events, je)
		}
		diffs := make([]jsonDiffEntry, 0, len(c.Diffs))
		for _, d := range c.Diffs {
			diffs = append(diffs, diffEntryToJSON(d))
		}
		out = append(out, jsonDiffClosure{Hash: c.Hash, Events: events, Diffs: diffs})
	}
	return marshal(struct {
		Closures []jsonDiffClosure `json:"closures"`
	}{Closures: out})
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

// D! id=ojcsum range-start
type jsonChangeNode struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	OldHash string `json:"oldHash,omitempty"`
	NewHash string `json:"newHash,omitempty"`
}

type jsonChangeEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type jsonChangeSummary struct {
	Operation   string           `json:"operation"`
	NodeChanges []jsonChangeNode `json:"nodeChanges,omitempty"`
	EdgeChanges []jsonChangeEdge `json:"edgeChanges,omitempty"`
}

type jsonChangeSummaryResult struct {
	Preview bool               `json:"preview"`
	Message string             `json:"message,omitempty"`
	Summary jsonChangeSummary   `json:"summary"`
}

func truncateHash(h string) string {
	if len(h) <= 8 {
		return h
	}
	return h[:8]
}

func (p JSONPresenter) ChangeSummary(r ChangeSummaryResult) string {
	jcs := jsonChangeSummary{Operation: r.Summary.Operation}
	for _, nc := range r.Summary.NodeChanges {
		jcs.NodeChanges = append(jcs.NodeChanges, jsonChangeNode{
			ID: nc.ID, Kind: nc.Kind,
			OldHash: truncateHash(nc.OldHash),
			NewHash: truncateHash(nc.NewHash),
		})
	}
	for _, ec := range r.Summary.EdgeChanges {
		jcs.EdgeChanges = append(jcs.EdgeChanges, jsonChangeEdge{
			From: ec.From, To: ec.To, Kind: ec.Kind,
		})
	}
	return marshal(jsonChangeSummaryResult{
		Preview: r.Preview,
		Message: r.Message,
		Summary: jcs,
	})
}

// D! id=ojcsum range-end

type jsonVersion struct {
	Version string `json:"version"`
}

func (p JSONPresenter) Version(r VersionResult) string {
	return marshal(jsonVersion{Version: r.Version})
}

// D! id=ojson range-end
