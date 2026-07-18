package statestore

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"

	"drift/core"
)

// D! id=pnope range-start
var ErrStateNotFound = errors.New(".drift/state.xml not found, run 'drift init' first")

// D! id=pnope range-end

// State is the in-memory shape of .drift/state.xml. Edges is the unified
// list of both link-style (marker→spec) and ref-style (spec→spec) edges;
// Resolutions is the unified list of edge resolutions covering either kind.
type State struct {
	Specs       []core.Spec
	Markers     []core.Marker
	Edges       []core.Edge
	Resolutions []core.EdgeResolution
}

type StateStore interface {
	Load() (State, error)
	Save(State) error
	// Initialized reports whether the project's state.xml already exists on
	// disk. Returns (true, nil) when state.xml exists (regardless of content);
	// (false, nil) when it does not exist; (false, err) when the existence
	// check itself fails (e.g. permission error). Used by Init to make the
	// init command non-idempotent.
	Initialized() (bool, error)
	// Lock acquires an exclusive advisory lock on the state file, blocking
	// until acquired. The returned function releases the lock and must be
	// called (typically via defer). All state-mutating operations must hold
	// the lock for the entire Load→modify→Save window to prevent concurrent
	// writers from silently overwriting each other's changes.
	Lock() (func(), error)
}

type FileStateStore struct {
	dir string
}

func NewFileStateStore(dir string) *FileStateStore {
	return &FileStateStore{dir: dir}
}

func (s *FileStateStore) Dir() string {
	return s.dir
}

// Initialized reports whether .drift/state.xml already exists on disk.
func (s *FileStateStore) Initialized() (bool, error) {
	_, err := os.Stat(s.statePath())
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *FileStateStore) statePath() string {
	return filepath.Join(s.dir, ".drift", "state.xml")
}

func (s *FileStateStore) baselinesDir() string {
	return filepath.Join(s.dir, ".drift", "baselines")
}

// stateFileXML serializes .drift/state.xml. version=3 is the post-collapse
// format with unified <edges> and <edgeResolutions> sections. Earlier
// versions used separate <links>+<refs> and <resolutions>+<refResolutions>.
type stateFileXML struct {
	XMLName       xml.Name            `xml:"drift"`
	Version       int                 `xml:"version,attr,omitempty"`
	Specs         []specXML           `xml:"specs>spec"`
	Markers       []markerXML         `xml:"markers>marker"`
	Edges         []edgeXML           `xml:"edges>edge"`
	Resolutions   []edgeResolutionXML `xml:"edgeResolutions>edgeResolution"`
}

type specXML struct {
	ID         string `xml:"id,attr"`
	Hash       string `xml:"hash,attr"`
	Filepath   string `xml:"filepath,attr"`
	LineNumber int    `xml:"line,attr"`
}

type markerXML struct {
	ID            string `xml:"id,attr"`
	Hash          string `xml:"hash,attr"`
	Filepath      string `xml:"filepath,attr"`
	LineNumber    int    `xml:"line,attr"`
	EndLineNumber int    `xml:"endline,attr"`
}

type edgeXML struct {
	From string `xml:"from,attr"`
	To   string `xml:"to,attr"`
}

type edgeResolutionXML struct {
	From            string `xml:"from,attr"`
	To              string `xml:"to,attr"`
	CurrentFromHash string `xml:"currentFromHash,attr"`
	CurrentToHash   string `xml:"currentToHash,attr"`
}

// D! id=pload range-start
func (s *FileStateStore) Load() (State, error) {
	data, err := os.ReadFile(s.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, ErrStateNotFound
		}
		return State{}, err
	}

	var file stateFileXML
	if err := xml.Unmarshal(data, &file); err != nil {
		return State{}, err
	}

	specs := make([]core.Spec, len(file.Specs))
	for i, sp := range file.Specs {
		specs[i] = core.Spec{
			ID:         sp.ID,
			Hash:       sp.Hash,
			Filepath:   sp.Filepath,
			LineNumber: sp.LineNumber,
		}
	}

	markers := make([]core.Marker, len(file.Markers))
	for i, m := range file.Markers {
		markers[i] = core.Marker{
			ID:            m.ID,
			Hash:          m.Hash,
			Filepath:      m.Filepath,
			LineNumber:    m.LineNumber,
			EndLineNumber: m.EndLineNumber,
		}
	}

	edges := make([]core.Edge, len(file.Edges))
	for i, e := range file.Edges {
		edges[i] = core.Edge{From: e.From, To: e.To}
	}

	resolutions := make([]core.EdgeResolution, len(file.Resolutions))
	for i, r := range file.Resolutions {
		resolutions[i] = core.EdgeResolution{
			From:            r.From,
			To:              r.To,
			CurrentFromHash: r.CurrentFromHash,
			CurrentToHash:   r.CurrentToHash,
		}
	}

	return State{
		Specs:       specs,
		Markers:     markers,
		Edges:       edges,
		Resolutions: resolutions,
	}, nil
}

// D! id=pload range-end

// D! id=psave range-start
func (s *FileStateStore) Save(state State) error {
	if err := os.MkdirAll(s.baselinesDir(), 0755); err != nil {
		return err
	}
	file := stateFileXML{
		Version:     3,
		Specs:       make([]specXML, len(state.Specs)),
		Markers:     make([]markerXML, len(state.Markers)),
		Edges:       make([]edgeXML, len(state.Edges)),
		Resolutions: make([]edgeResolutionXML, len(state.Resolutions)),
	}

	for i, spec := range state.Specs {
		file.Specs[i] = specXML{
			ID:         spec.ID,
			Hash:       spec.Hash,
			Filepath:   spec.Filepath,
			LineNumber: spec.LineNumber,
		}
	}

	for i, marker := range state.Markers {
		file.Markers[i] = markerXML{
			ID:            marker.ID,
			Hash:          marker.Hash,
			Filepath:      marker.Filepath,
			LineNumber:    marker.LineNumber,
			EndLineNumber: marker.EndLineNumber,
		}
	}

	for i, e := range state.Edges {
		file.Edges[i] = edgeXML{From: e.From, To: e.To}
	}

	for i, r := range state.Resolutions {
		file.Resolutions[i] = edgeResolutionXML{
			From:            r.From,
			To:              r.To,
			CurrentFromHash: r.CurrentFromHash,
			CurrentToHash:   r.CurrentToHash,
		}
	}

	data, err := xml.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return os.WriteFile(s.statePath(), data, 0644)
}

// D! id=psave range-end
