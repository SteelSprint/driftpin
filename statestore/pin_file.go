package statestore

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"drift/core"
	"drift/internal/fileio"
)

// D! id=pnope range-start
var ErrStateNotFound = errors.New(".drift/state.xml not found, run 'drift init' first")

// ErrStateVersionUnsupported is returned when state.xml carries a version
// this binary refuses to load. v4 drops <edgeResolutions> entirely; v3 and
// earlier are refused with a clear error directing the user to re-init.
var ErrStateVersionUnsupported = errors.New("state.xml version unsupported; delete .drift/ and run 'drift init'")

// D! id=pnope range-end

// State is the in-memory shape of .drift/state.xml. Edges is the unified
// list of both link-style (marker→spec) and ref-style (spec→spec) edges.
// State.xml v4 carries baseline only — no per-edge resolutions.
type State struct {
	Specs   []core.Spec
	Markers []core.Marker
	Edges   []core.Edge
}

// stateFileName is the name of the state file inside .drift/.
const stateFileName = "state.xml"

type StateStore interface {
	Load(sess *fileio.Session) (State, error)
	Save(sess *fileio.Session, state State) error
	Initialized() (bool, error)
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

func (s *FileStateStore) Initialized() (bool, error) {
	_, err := os.Stat(filepath.Join(s.dir, ".drift", stateFileName))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// stateFileXML serializes .drift/state.xml. version=4 is the
// provenance-closure format: baseline only, no <edgeResolutions>.
// v3 (with resolutions) and earlier are refused on Load.
type stateFileXML struct {
	XMLName xml.Name    `xml:"drift"`
	Version int         `xml:"version,attr,omitempty"`
	Specs   []specXML   `xml:"specs>spec"`
	Markers []markerXML `xml:"markers>marker"`
	Edges   []edgeXML   `xml:"edges>edge"`
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

// D! id=pload range-start
func (s *FileStateStore) Load(sess *fileio.Session) (State, error) {
	data, err := sess.Read(stateFileName)
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

	// Refuse pre-v4 state files. v4 dropped <edgeResolutions>; older files
	// must be deleted and re-initialized.
	if file.Version > 0 && file.Version < 4 {
		return State{}, fmt.Errorf("%w: version=%d", ErrStateVersionUnsupported, file.Version)
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

	return State{
		Specs:   specs,
		Markers: markers,
		Edges:   edges,
	}, nil
}

// D! id=pload range-end

// D! id=psave range-start
func (s *FileStateStore) Save(sess *fileio.Session, state State) error {
	file := stateFileXML{
		Version: 4,
		Specs:   make([]specXML, len(state.Specs)),
		Markers: make([]markerXML, len(state.Markers)),
		Edges:   make([]edgeXML, len(state.Edges)),
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

	data, err := xml.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return sess.Write(stateFileName, data)
}

// D! id=psave range-end
