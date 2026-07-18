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

type State struct {
	Specs           []core.Spec
	Markers         []core.Marker
	Links           []core.Link
	ResolutionState []core.ResolutionState
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

type stateFileXML struct {
	XMLName     xml.Name        `xml:"drift"`
	Specs       []specXML       `xml:"specs>spec"`
	Markers     []markerXML     `xml:"markers>marker"`
	Links       []linkXML       `xml:"links>link"`
	Resolutions []resolutionXML `xml:"resolutions>resolution"`
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

type linkXML struct {
	SpecID   string `xml:"specId,attr"`
	MarkerID string `xml:"markerId,attr"`
}

type resolutionXML struct {
	SpecID            string `xml:"specId,attr"`
	MarkerID          string `xml:"markerId,attr"`
	CurrentSpecHash   string `xml:"currentSpecHash,attr"`
	CurrentMarkerHash string `xml:"currentMarkerHash,attr"`
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
	for i, s := range file.Specs {
		specs[i] = core.Spec{
			ID:         s.ID,
			Hash:       s.Hash,
			Filepath:   s.Filepath,
			LineNumber: s.LineNumber,
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

	links := make([]core.Link, len(file.Links))
	for i, l := range file.Links {
		links[i] = core.Link{
			SpecID:   l.SpecID,
			MarkerID: l.MarkerID,
		}
	}

	resolutions := make([]core.ResolutionState, len(file.Resolutions))
	for i, r := range file.Resolutions {
		resolutions[i] = core.ResolutionState{
			SpecID:            r.SpecID,
			MarkerID:          r.MarkerID,
			CurrentSpecHash:   r.CurrentSpecHash,
			CurrentMarkerHash: r.CurrentMarkerHash,
		}
	}

	return State{
		Specs:           specs,
		Markers:         markers,
		Links:           links,
		ResolutionState: resolutions,
	}, nil
}

// D! id=pload range-end

// D! id=psave range-start
func (s *FileStateStore) Save(state State) error {
	if err := os.MkdirAll(s.baselinesDir(), 0755); err != nil {
		return err
	}
	file := stateFileXML{
		Specs:       make([]specXML, len(state.Specs)),
		Markers:     make([]markerXML, len(state.Markers)),
		Links:       make([]linkXML, len(state.Links)),
		Resolutions: make([]resolutionXML, len(state.ResolutionState)),
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

	for i, link := range state.Links {
		file.Links[i] = linkXML{
			SpecID:   link.SpecID,
			MarkerID: link.MarkerID,
		}
	}

	for i, res := range state.ResolutionState {
		file.Resolutions[i] = resolutionXML{
			SpecID:            res.SpecID,
			MarkerID:          res.MarkerID,
			CurrentSpecHash:   res.CurrentSpecHash,
			CurrentMarkerHash: res.CurrentMarkerHash,
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
