package main

import (
	"crypto/sha256"
	"encoding/hex"
)

type ElementKind int

const (
	KindClause ElementKind = iota
	KindSection
	KindTerm
)

func (k ElementKind) String() string {
	switch k {
	case KindClause:
		return "clause"
	case KindSection:
		return "section"
	case KindTerm:
		return "term"
	}
	return "unknown"
}

type Element struct {
	Kind          ElementKind
	ID            string
	Label         string
	Text          string
	Refs          []string // IDs from <ref> elements, in order of appearance
	Parent        *Element
	Kids          []*Element
	InDefinitions bool
}

func (e *Element) Path() string {
	return e.ID
}

type Spec struct {
	Name        string
	Version     string
	Description string
	Elements    []*Element
}

func (s *Spec) All() []*Element {
	var out []*Element
	var walk func(e *Element)
	walk = func(e *Element) {
		for _, k := range e.Kids {
			out = append(out, k)
			walk(k)
		}
	}
	walk(&Element{Kids: s.Elements})
	return out
}

func (s *Spec) DefinedIDs() map[string]bool {
	out := make(map[string]bool)
	for _, e := range s.All() {
		if isStructural(e) {
			continue
		}
		out[e.ID] = true
	}
	return out
}

func (s *Spec) ElementKinds() map[string]ElementKind {
	out := make(map[string]ElementKind)
	for _, e := range s.All() {
		out[e.ID] = e.Kind
	}
	return out
}

func isStructural(e *Element) bool {
	if e.Kind == KindSection && (e.ID == "spec" || e.ID == "description" || e.ID == "definitions") {
		return true
	}
	return false
}

func (e *Element) HasText() bool {
	t := e.Text
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return true
		}
	}
	return false
}

func (e *Element) Content() string {
	if e.Kind == KindSection {
		return e.Label
	}
	return trimSpace(e.Text)
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func ReferencesInOrder(e *Element, defined map[string]bool) []string {
	if e.Kind == KindSection {
		return nil
	}
	var out []string
	seen := make(map[string]bool)
	for _, r := range e.Refs {
		if !defined[r] {
			continue
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

func ComputeHash(e *Element, defined map[string]bool, hashes map[string]string) string {
	if e.Kind == KindSection {
		return hashOf(e.Content(), nil)
	}
	refs := ReferencesInOrder(e, defined)
	hashRefs := make([]string, 0, len(refs))
	for _, r := range refs {
		hashRefs = append(hashRefs, hashes[r])
	}
	return hashOf(e.Content(), hashRefs)
}

// #F id:3aewemki hash.algorithm hash.input.content hash.input.references hash.input.references_order hash.input.whitespace hash.input.separator hash.output.format
func hashOf(content string, refs []string) string {
	h := sha256.New()
	h.Write([]byte(content))
	for _, r := range refs {
		h.Write([]byte{0x0A})
		h.Write([]byte(r))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ComputeAllHashes(s *Spec) map[string]string {
	hashes := make(map[string]string)
	defined := s.DefinedIDs()
	for _, e := range s.All() {
		hashes[e.Path()] = ComputeHash(e, defined, hashes)
	}
	return hashes
}
