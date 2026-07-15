package pinstore

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"

	"driftpin/core"
)

// #F pnope
var ErrPinNotFound = errors.New("drift.pin not found, run 'drift init' first")

type PinState struct {
	Specs           []core.Spec
	Markers         []core.Marker
	Links           []core.Link
	ResolutionState []core.ResolutionState
}

type PinStore interface {
	Load() (PinState, error)
	Save(PinState) error
}

type FilePinStore struct {
	path string
}

func NewFilePinStore(dir string) *FilePinStore {
	return &FilePinStore{path: filepath.Join(dir, "drift.pin")}
}

type pinFileXML struct {
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
	ID         string `xml:"id,attr"`
	Hash       string `xml:"hash,attr"`
	Filepath   string `xml:"filepath,attr"`
	LineNumber int    `xml:"line,attr"`
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

// #F pload
func (s *FilePinStore) Load() (PinState, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return PinState{}, ErrPinNotFound
		}
		return PinState{}, err
	}

	var file pinFileXML
	if err := xml.Unmarshal(data, &file); err != nil {
		return PinState{}, err
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
			ID:         m.ID,
			Hash:       m.Hash,
			Filepath:   m.Filepath,
			LineNumber: m.LineNumber,
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

	return PinState{
		Specs:           specs,
		Markers:         markers,
		Links:           links,
		ResolutionState: resolutions,
	}, nil
}

// #F psave
func (s *FilePinStore) Save(state PinState) error {
	file := pinFileXML{
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
			ID:         marker.ID,
			Hash:       marker.Hash,
			Filepath:   marker.Filepath,
			LineNumber: marker.LineNumber,
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
	return os.WriteFile(s.path, data, 0644)
}
