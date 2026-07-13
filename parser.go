package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[a-z0-9_]+(\.[a-z0-9_]+)*$`)

type Violation struct {
	Rule   string
	Detail string
}

func (v Violation) String() string {
	return v.Rule + ": " + v.Detail
}

// #F id:0lsoy7u2 parser_rules.description_ignored parser_rules.path_group parser_rules.no_forward_refs parser_rules.no_cycles parser_rules.no_self_references parser_rules.no_text_on_sections parser_rules.no_empty_clauses parser_rules.no_empty_sections parser_rules.no_empty_terms parser_rules.unique_ids parser_rules.valid_id_format parser_rules.single_definitions parser_rules.terms_in_definitions parser_rules.ref_content parser_rules.ref_target_undefined parser_rules.term_refs_terms













func ParseSpecFile(path string) (*Spec, []Violation, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, nil, err
	}
	return ParseSpec(data)
}

func ParseSpec(data []byte) (*Spec, []Violation, error) {
	spec, err := parseFromReader(stringReader(data))
	if err != nil {
		return nil, nil, err
	}
	var violations []Violation
	validate(spec, &violations)
	return spec, violations, nil
}

func stringReader(s []byte) io.Reader {
	return &byteReader{buf: s, pos: 0}
}

type byteReader struct {
	buf []byte
	pos int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n := copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}

func parseFromReader(r io.Reader) (*Spec, error) {
	dec := xml.NewDecoder(r)
	spec := &Spec{}
	stack := []*Element{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml parse error: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var parent *Element
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			el, selfClosing, err := parseStartElement(t, dec, parent, spec)
			if err != nil {
				return nil, err
			}
			if el == nil {
				continue
			}
			if parent != nil {
				el.Parent = parent
				parent.Kids = append(parent.Kids, el)
			} else {
				spec.Elements = append(spec.Elements, el)
			}
			if !selfClosing {
				stack = append(stack, el)
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}

	return spec, nil
}

func parseStartElement(t xml.StartElement, dec *xml.Decoder, parent *Element, spec *Spec) (*Element, bool, error) {
	switch t.Name.Local {
	case "spec":
		for _, attr := range t.Attr {
			if attr.Name.Local == "name" {
				spec.Name = attr.Value
			}
		}
		return &Element{Kind: KindSection, ID: "spec", Label: spec.Name}, false, nil
	case "description":
		body, err := readCharDataOnly(dec, "description")
		if err != nil {
			return nil, false, err
		}
		return &Element{Kind: KindSection, ID: "description", Text: body}, true, nil
	case "definitions":
		return &Element{Kind: KindSection, ID: "definitions", Label: "Definitions"}, false, nil
	case "term":
		var textAttr string
		for _, attr := range t.Attr {
			if attr.Name.Local == "text" {
				textAttr = attr.Value
			}
		}
		body, refs, err := readClauseBody(dec, "term")
		if err != nil {
			return nil, false, err
		}
		inDefs := parent != nil && parent.ID == "definitions"
		return &Element{Kind: KindTerm, ID: textAttr, Text: body, Refs: refs, InDefinitions: inDefs}, true, nil
	case "section":
		id := attrString(t, "id")
		label := attrString(t, "label")
		return &Element{Kind: KindSection, ID: id, Label: label}, false, nil
	case "clause":
		id := attrString(t, "id")
		body, refs, err := readClauseBody(dec, "clause")
		if err != nil {
			return nil, false, err
		}
		return &Element{Kind: KindClause, ID: id, Text: body, Refs: refs}, true, nil
	}
	return nil, false, fmt.Errorf("unexpected element: %s", t.Name.Local)
}

func attrString(t xml.StartElement, name string) string {
	for _, attr := range t.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func readCharDataOnly(dec *xml.Decoder, endName string) (string, error) {
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return "", fmt.Errorf("unexpected nested element %s inside %s", t.Name.Local, endName)
		case xml.EndElement:
			if t.Name.Local != endName {
				return "", fmt.Errorf("unexpected end element %s", t.Name.Local)
			}
			return buf.String(), nil
		case xml.CharData:
			buf.Write(t)
		}
	}
}

func readClauseBody(dec *xml.Decoder, endName string) (string, []string, error) {
	var buf strings.Builder
	var refs []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "ref" {
				refBody, err := readRefContent(dec)
				if err != nil {
					return "", nil, err
				}
				buf.WriteString(refBody)
				refs = append(refs, refBody)
			} else {
				return "", nil, fmt.Errorf("unexpected nested element %s inside %s", t.Name.Local, endName)
			}
		case xml.EndElement:
			if t.Name.Local != endName {
				return "", nil, fmt.Errorf("unexpected end element %s", t.Name.Local)
			}
			return buf.String(), refs, nil
		case xml.CharData:
			buf.Write(t)
		}
	}
}

func readRefContent(dec *xml.Decoder) (string, error) {
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return "", fmt.Errorf("unexpected nested element %s inside ref", t.Name.Local)
		case xml.EndElement:
			if t.Name.Local != "ref" {
				return "", fmt.Errorf("unexpected end element %s", t.Name.Local)
			}
			return buf.String(), nil
		case xml.CharData:
			buf.Write(t)
		}
	}
}

func validate(spec *Spec, out *[]Violation) {
	checkDescriptionIgnored(spec, out)
	checkPathGroup(spec, out)
	checkNoForwardRefs(spec, out)
	checkNoSelfReferences(spec, out)
	checkNoCycles(spec, out)
	checkNoTextOnSections(spec, out)
	checkNoEmptyClauses(spec, out)
	checkNoEmptySections(spec, out)
	checkNoEmptyTerms(spec, out)
	checkUniqueIDs(spec, out)
	checkValidIDFormat(spec, out)
	checkSingleDefinitions(spec, out)
	checkTermsInDefinitions(spec, out)
	checkRefContent(spec, out)
	checkRefTargetUndefined(spec, out)
	checkTermRefsTerms(spec, out)
}

func checkDescriptionIgnored(spec *Spec, out *[]Violation) {
	count := 0
	var walk func(e *Element)
	walk = func(e *Element) {
		if e.Kind == KindSection && e.ID == "description" {
			count++
		}
		for _, k := range e.Kids {
			walk(k)
		}
	}
	for _, e := range spec.Elements {
		walk(e)
	}
	if count > 1 {
		*out = append(*out, Violation{"parser_rules.description_ignored", "description element appears more than once"})
	}
}

func checkPathGroup(spec *Spec, out *[]Violation) {
	var walk func(parent *Element, e *Element)
	walk = func(parent *Element, e *Element) {
		if parent != nil && parent.ID != "spec" && parent.ID != "definitions" && parent.ID != "description" {
			expected := parent.ID + "." + lastSegment(e.ID)
			if e.ID != expected {
				*out = append(*out, Violation{"parser_rules.path_group", fmt.Sprintf("%s: id is %q, expected %q", e.ID, e.ID, expected)})
			}
		}
		for _, k := range e.Kids {
			walk(e, k)
		}
	}
	for _, e := range spec.Elements {
		walk(nil, e)
	}
}

func lastSegment(id string) string {
	if idx := strings.LastIndex(id, "."); idx >= 0 {
		return id[idx+1:]
	}
	return id
}

func checkNoForwardRefs(spec *Spec, out *[]Violation) {
	defined := spec.DefinedIDs()
	order := make(map[string]int)
	i := 0
	var walk func(e *Element)
	walk = func(e *Element) {
		if isStructural(e) {
			for _, k := range e.Kids {
				walk(k)
			}
			return
		}
		order[e.ID] = i
		i++
		for _, k := range e.Kids {
			walk(k)
		}
	}
	for _, e := range spec.Elements {
		walk(e)
	}
	for _, e := range spec.All() {
		if e.Kind == KindSection {
			continue
		}
		refs := ReferencesInOrder(e, defined)
		for _, r := range refs {
			if order[r] >= order[e.ID] {
				*out = append(*out, Violation{"parser_rules.no_forward_refs", fmt.Sprintf("%s refs %s which is not defined earlier", e.ID, r)})
			}
		}
	}
}

func checkNoSelfReferences(spec *Spec, out *[]Violation) {
	defined := spec.DefinedIDs()
	for _, e := range spec.All() {
		if e.Kind == KindSection {
			continue
		}
		refs := ReferencesInOrder(e, defined)
		for _, r := range refs {
			if r == e.ID {
				*out = append(*out, Violation{"parser_rules.no_self_references", fmt.Sprintf("%s refs itself", e.ID)})
			}
		}
	}
}

func checkNoCycles(spec *Spec, out *[]Violation) {
	defined := spec.DefinedIDs()
	graph := make(map[string][]string)
	for _, e := range spec.All() {
		if e.Kind == KindSection {
			continue
		}
		for _, r := range ReferencesInOrder(e, defined) {
			graph[e.ID] = append(graph[e.ID], r)
		}
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	for n := range graph {
		color[n] = white
	}
	var dfs func(n string) bool
	dfs = func(n string) bool {
		color[n] = gray
		for _, m := range graph[n] {
			if color[m] == gray {
				return true
			}
			if color[m] == white && dfs(m) {
				return true
			}
		}
		color[n] = black
		return false
	}
	for n := range graph {
		if color[n] == white && dfs(n) {
			*out = append(*out, Violation{"parser_rules.no_cycles", "reference graph contains a cycle"})
			return
		}
	}
}

func checkNoTextOnSections(spec *Spec, out *[]Violation) {
	for _, e := range spec.All() {
		if isStructural(e) {
			continue
		}
		if e.Kind == KindSection && e.HasText() {
			*out = append(*out, Violation{"parser_rules.no_text_on_sections", fmt.Sprintf("section %s has non-whitespace text content", e.ID)})
		}
	}
}

func checkNoEmptyClauses(spec *Spec, out *[]Violation) {
	for _, e := range spec.All() {
		if e.Kind == KindClause && !e.HasText() {
			*out = append(*out, Violation{"parser_rules.no_empty_clauses", fmt.Sprintf("clause %s has no text content", e.ID)})
		}
	}
}

func checkNoEmptySections(spec *Spec, out *[]Violation) {
	for _, e := range spec.All() {
		if isStructural(e) {
			continue
		}
		if e.Kind == KindSection && len(e.Kids) == 0 {
			*out = append(*out, Violation{"parser_rules.no_empty_sections", fmt.Sprintf("section %s has no sub-elements", e.ID)})
		}
	}
}

func checkNoEmptyTerms(spec *Spec, out *[]Violation) {
	for _, e := range spec.All() {
		if e.Kind == KindTerm && !e.HasText() {
			*out = append(*out, Violation{"parser_rules.no_empty_terms", fmt.Sprintf("term %s has no text content", e.ID)})
		}
	}
}

func checkUniqueIDs(spec *Spec, out *[]Violation) {
	seen := make(map[string]int)
	for _, e := range spec.All() {
		seen[e.ID]++
	}
	for id, n := range seen {
		if n > 1 {
			*out = append(*out, Violation{"parser_rules.unique_ids", fmt.Sprintf("id %q appears %d times", id, n)})
		}
	}
}

func checkValidIDFormat(spec *Spec, out *[]Violation) {
	for _, e := range spec.All() {
		if isStructural(e) {
			continue
		}
		if !idPattern.MatchString(e.ID) {
			*out = append(*out, Violation{"parser_rules.valid_id_format", fmt.Sprintf("id %q does not match [a-z0-9_]+ per segment", e.ID)})
		}
	}
}

func checkSingleDefinitions(spec *Spec, out *[]Violation) {
	count := 0
	var walk func(e *Element)
	walk = func(e *Element) {
		if e.Kind == KindSection && e.ID == "definitions" {
			count++
		}
		for _, k := range e.Kids {
			walk(k)
		}
	}
	for _, e := range spec.Elements {
		walk(e)
	}
	if count > 1 {
		*out = append(*out, Violation{"parser_rules.single_definitions", "definitions block appears more than once"})
	}
}

func checkTermsInDefinitions(spec *Spec, out *[]Violation) {
	for _, e := range spec.All() {
		if e.Kind == KindTerm && !e.InDefinitions {
			*out = append(*out, Violation{"parser_rules.terms_in_definitions", fmt.Sprintf("term %s appears outside a definitions block", e.ID)})
		}
	}
}

func checkRefContent(spec *Spec, out *[]Violation) {
	for _, e := range spec.All() {
		for _, ref := range e.Refs {
			if ref == "" {
				*out = append(*out, Violation{"parser_rules.ref_content", fmt.Sprintf("%s has an empty <ref> element", e.ID)})
			}
		}
	}
}

func checkRefTargetUndefined(spec *Spec, out *[]Violation) {
	defined := spec.DefinedIDs()
	for _, e := range spec.All() {
		for _, ref := range e.Refs {
			if !defined[ref] {
				*out = append(*out, Violation{"parser_rules.ref_target_undefined", fmt.Sprintf("%s refs %s which is not a defined id", e.ID, ref)})
			}
		}
	}
}

func checkTermRefsTerms(spec *Spec, out *[]Violation) {
	kinds := spec.ElementKinds()
	for _, e := range spec.All() {
		if e.Kind != KindTerm {
			continue
		}
		for _, ref := range e.Refs {
			targetKind, ok := kinds[ref]
			if !ok {
				continue // ref_target_undefined will catch this
			}
			if targetKind != KindTerm {
				*out = append(*out, Violation{"parser_rules.term_refs_terms", fmt.Sprintf("term %s refs %s which is a %s, not a term. Terms are vocabulary; clauses are requirements. A term may only reference another term because vocabulary must not depend on requirements — dependencies flow downward (clauses → terms), not upward. Remedy: reword the term to not reference the clause, or move the dependency into a clause instead.", e.ID, ref, targetKind.String())})
			}
		}
	}
}

func ruleExplanation(rule string) string {
	switch rule {
	case "parser_rules.description_ignored":
		return "A description element must appear at most once and only at the top level. Remedy: remove duplicate description elements."
	case "parser_rules.path_group":
		return "A child element's id must extend its parent section's id. Remedy: rename the child id or move it under the correct parent."
	case "parser_rules.no_forward_refs":
		return "A <ref> must point to an element defined earlier in the spec. Remedy: reorder so the referenced element appears before the referencing one."
	case "parser_rules.no_cycles":
		return "The <ref> reference graph must be acyclic. Remedy: break the cycle by removing a <ref> that creates a circular dependency."
	case "parser_rules.no_self_references":
		return "An element must not <ref> itself. Remedy: remove the self-referencing <ref>."
	case "parser_rules.no_text_on_sections":
		return "A section must not contain non-whitespace text. Remedy: move text into a child clause."
	case "parser_rules.no_empty_clauses":
		return "A clause must contain non-whitespace text. Remedy: add content or remove the empty clause."
	case "parser_rules.no_empty_sections":
		return "A section must contain at least one sub-clause or sub-section. Remedy: add a child element or remove the empty section."
	case "parser_rules.no_empty_terms":
		return "A term must contain non-whitespace text. Remedy: add content or remove the empty term."
	case "parser_rules.unique_ids":
		return "Each id must be unique within the spec. Remedy: rename the duplicate element."
	case "parser_rules.valid_id_format":
		return "Each id segment must match [a-z0-9_]+. Remedy: rename using only lowercase letters, digits, and underscores."
	case "parser_rules.single_definitions":
		return "A spec must contain at most one definitions block. Remedy: merge the definitions blocks into one."
	case "parser_rules.terms_in_definitions":
		return "A term must appear inside a definitions block. Remedy: move the term inside a <definitions> element."
	case "parser_rules.ref_content":
		return "A <ref> element must contain non-empty text content matching a defined id. Remedy: add text content (the referenced id) inside the <ref> element."
	case "parser_rules.ref_target_undefined":
		return "A <ref> element's text content must match a defined clause, section, or term id. Remedy: fix the id or define the missing element."
	case "parser_rules.term_refs_terms":
		return "A <ref> inside a term must reference another term, not a clause or section. Terms are vocabulary; clauses are requirements — dependencies flow downward, not upward. Remedy: reword the term to not reference the clause, or move the dependency into a clause."
	default:
		return ""
	}
}
