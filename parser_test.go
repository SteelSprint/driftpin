package main

import (
	"path/filepath"
	"testing"
)

func mustParse(t *testing.T, path string) (*Spec, []Violation) {
	t.Helper()
	spec, violations, err := ParseSpecFile(path)
	if err != nil {
		t.Fatalf("parse error for %s: %v", path, err)
	}
	return spec, violations
}

func assertViolations(t *testing.T, path string) {
	t.Helper()
	_, violations := mustParse(t, path)
	if len(violations) == 0 {
		t.Fatalf("expected at least one violation in %s, got none", path)
	}
}

func assertClean(t *testing.T, path string) {
	t.Helper()
	_, violations := mustParse(t, path)
	if len(violations) != 0 {
		t.Fatalf("expected no violations in %s, got %d: %v", path, len(violations), violations)
	}
}

func invalidCase(name string) string {
	return filepath.Join("cases", "invalid", name)
}

func validCase(name string) string {
	return filepath.Join("cases", "valid", name)
}

func TestParserRules_PathGroup(t *testing.T) {
// #F id:fg37r9bw parser_rules.path_group
	assertViolations(t, invalidCase("f1_path_group_mismatch.spec.xml"))
}

func TestParserRules_NoForwardRefs(t *testing.T) {
// #F id:11s1xuip parser_rules.no_forward_refs
	assertViolations(t, invalidCase("f2_forward_reference.spec.xml"))
}

func TestParserRules_NoCycles(t *testing.T) {
// #F id:gzua1kli parser_rules.no_cycles
	assertViolations(t, invalidCase("f3_cycle_length_2.spec.xml"))
	assertViolations(t, invalidCase("f8_cycle_length_3.spec.xml"))
	assertViolations(t, invalidCase("f12_term_term_cycle.spec.xml"))
	assertViolations(t, invalidCase("f13_term_clause_cycle.spec.xml"))
}

func TestParserRules_NoSelfReferences(t *testing.T) {
// #F id:lnoq98em parser_rules.no_self_references
	assertViolations(t, invalidCase("f4_self_reference.spec.xml"))
	assertViolations(t, invalidCase("f11_term_self_reference.spec.xml"))
}

func TestParserRules_NoTextOnSections(t *testing.T) {
// #F id:i2hs1wbd parser_rules.no_text_on_sections
	assertViolations(t, invalidCase("f5_text_on_section.spec.xml"))
}

func TestParserRules_UniqueIDs(t *testing.T) {
// #F id:tvpe18cs parser_rules.unique_ids
	assertViolations(t, invalidCase("f6_duplicate_id.spec.xml"))
}

func TestParserRules_ValidIDFormat(t *testing.T) {
// #F id:yfn9sh5h parser_rules.valid_id_format
	assertViolations(t, invalidCase("f7_invalid_id_format.spec.xml"))
}

func TestParserRules_TermsInDefinitions(t *testing.T) {
// #F id:0twn6plj parser_rules.terms_in_definitions
	assertViolations(t, invalidCase("f9_term_outside_definitions.spec.xml"))
}

func TestParserRules_NoEmptyClauses(t *testing.T) {
// #F id:rix4s726 parser_rules.no_empty_clauses
	assertViolations(t, invalidCase("f10_empty_clause.spec.xml"))
}

func TestParserRules_NoEmptySections(t *testing.T) {
// #F id:zlw376pa parser_rules.no_empty_sections
	assertClean(t, validCase("simple.spec.xml"))
}

func TestParserRules_NoEmptyTerms(t *testing.T) {
// #F id:mnt7q7un parser_rules.no_empty_terms
	assertClean(t, validCase("simple.spec.xml"))
}

func TestParserRules_DescriptionIgnored(t *testing.T) {
// #F id:zr4zgh7h parser_rules.description_ignored
	assertClean(t, "filament.spec.xml")
}

func TestParserRules_SingleDefinitions(t *testing.T) {
// #F id:duummbj5 parser_rules.single_definitions
	assertClean(t, "filament.spec.xml")
	assertViolations(t, invalidCase("f14_single_definitions.spec.xml"))
}

// #F id:r3ftr001 parser_rules.ref_target_undefined
func TestParserRules_RefTargetUndefined(t *testing.T) {
	assertViolations(t, invalidCase("f14_ref_target_undefined.spec.xml"))
}

// #F id:t3rmr001 parser_rules.term_refs_terms
func TestParserRules_TermRefsTerms(t *testing.T) {
	assertViolations(t, invalidCase("f15_term_refs_clause.spec.xml"))
}

// #F id:r3fc0001 parser_rules.ref_content
func TestParserRules_RefContent(t *testing.T) {
	assertViolations(t, invalidCase("f16_empty_ref.spec.xml"))
}

func TestValidSpecs(t *testing.T) {
	assertClean(t, validCase("simple.spec.xml"))
	assertClean(t, validCase("nested.spec.xml"))
	assertClean(t, validCase("alphabetic.spec.xml"))
	assertClean(t, validCase("nested_refs.spec.xml"))
	assertClean(t, validCase("duplicate_refs.spec.xml"))
}

func TestMainSpec(t *testing.T) {
	// #F id:fa7f2cy9 tool.name tool.location tool.language tool.design tool.binary




	assertClean(t, "filament.spec.xml")
}
