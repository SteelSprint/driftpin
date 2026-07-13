package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// #F id:7b8xrluy marker_format.syntax
// #F id:hayz954z marker_format.id_format
// #F id:bnivtwrc marker_format.comment_prefix
var markerPattern = regexp.MustCompile(`#F\s+id:([a-z0-9]{8})\s+(.+)`)

// attemptPattern matches lines that look like marker attempts but may be malformed.
// A line matching this but not markerPattern is a malformed marker.
// Requires a word character after id: to avoid matching string literals and prose.
var attemptPattern = regexp.MustCompile(`#F\s+id:\w`)

const markerIDLength = 8
const markerIDChars = "abcdefghijklmnopqrstuvwxyz0123456789"

// #F id:xpn0winy marker_format.id_generation

type MarkerMatch struct {
	MarkerID string
	ClauseIDs []string
	Flags    map[string]string
	Line     int
	Col      int
	Raw      string
}

func ParseMarker(line string) *MarkerMatch {
	m := markerPattern.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	markerID := m[1]
	rest := m[2]

	// Split on -- to separate clause_ids from flags
	var clausePart, flagPart string
	if idx := strings.Index(rest, "--"); idx >= 0 {
		clausePart = strings.TrimSpace(rest[:idx])
		flagPart = strings.TrimSpace(rest[idx+2:])
	} else {
		clausePart = strings.TrimSpace(rest)
	}

	// Parse clause_ids
	clauseIDs := strings.Fields(clausePart)
	if len(clauseIDs) == 0 {
		return nil
	}

	// Parse flags (key=value pairs)
	flags := make(map[string]string)
	if flagPart != "" {
		for _, f := range strings.Fields(flagPart) {
			if kv := strings.SplitN(f, "=", 2); len(kv) == 2 {
				flags[kv[0]] = kv[1]
			}
		}
	}

	return &MarkerMatch{
		MarkerID:  markerID,
		ClauseIDs: clauseIDs,
		Flags:     flags,
		Raw:       line,
	}
}

func GenerateMarkerID(existing map[string]bool) (string, error) {
	for attempts := 0; attempts < 1000; attempts++ {
		id, err := randomMarkerID()
		if err != nil {
			return "", err
		}
		if !existing[id] {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique marker id after 1000 attempts")
}

func randomMarkerID() (string, error) {
	b := make([]byte, markerIDLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(markerIDChars))))
		if err != nil {
			return "", err
		}
		b[i] = markerIDChars[n.Int64()]
	}
	return string(b), nil
}

func MarkerIDIsValid(id string) bool {
	// #F id:0gey5aep marker_format.malformed
	if len(id) != markerIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
