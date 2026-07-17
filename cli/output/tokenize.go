package output

import "strings"

// D! id=otok range-start

// Token represents a classified segment of a line.
type Token struct {
	Type string // "comment", "string", "keyword", "number", "plain"
	Text string
}

// codeKeywords is a cross-language intersection of common keywords. Not
// exhaustive for any single language; approximate across many. This is the
// "poor-man's" part — no per-language config, no grammar.
var codeKeywords = map[string]bool{
	"if": true, "else": true, "elif": true, "for": true, "while": true,
	"do": true, "return": true, "break": true, "continue": true,
	"switch": true, "case": true, "default": true, "goto": true,
	"func": true, "def": true, "fn": true, "function": true,
	"class": true, "struct": true, "enum": true, "interface": true,
	"type": true, "typedef": true, "union": true, "trait": true,
	"impl": true, "extends": true, "implements": true,
	"const": true, "var": true, "let": true, "static": true,
	"final": true, "abstract": true, "public": true, "private": true,
	"protected": true, "internal": true, "export": true,
	"import": true, "include": true, "require": true, "from": true,
	"package": true, "module": true, "use": true, "namespace": true,
	"void": true, "nil": true, "null": true, "none": true,
	"true": true, "false": true, "bool": true, "boolean": true,
	"int": true, "float": true, "double": true, "string": true,
	"byte": true, "char": true, "uint": true, "uintptr": true,
	"error": true, "err": true, "panic": true,
	"go": true, "async": true, "await": true, "yield": true,
	"chan": true, "select": true, "defer": true,
	"new": true, "delete": true, "sizeof": true, "in": true,
	"is": true, "as": true, "with": true, "lambda": true,
	"end": true, "then": true, "and": true, "or": true, "not": true,
	"try": true, "catch": true, "throw": true, "throws": true,
	"except": true, "finally": true, "raise": true,
	"match": true, "when": true, "loop": true, "next": true,
	"self": true, "this": true, "super": true, "base": true,
}

// tokenizeLine classifies a single line into tokens. The concatenation of all
// token texts equals the original line — this invariant is critical for the
// guardrail property (stripANSI(colorized) == original).
//
// The tokenizer is language-agnostic: no file extension lookup, no per-language
// grammar. It uses universal patterns — quoted strings, line-comment prefixes,
// a cross-language keyword set, and numeric literals.
func tokenizeLine(line string) []Token {
	if line == "" {
		return nil
	}

	var tokens []Token
	plainStart := 0
	flushPlain := func(end int) {
		if end > plainStart {
			tokens = append(tokens, Token{Type: "plain", Text: line[plainStart:end]})
		}
	}

	i := 0
	n := len(line)

	for i < n {
		c := line[i]

		// --- String detection ---
		if c == '"' || c == '\'' || c == '`' {
			flushPlain(i)
			quote := c
			start := i
			i++
			for i < n {
				if line[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if line[i] == quote {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Type: "string", Text: line[start:i]})
			plainStart = i
			continue
		}

		// --- Comment detection (word-boundary aware) ---
		if isCommentStart(line, i) {
			flushPlain(i)
			tokens = append(tokens, Token{Type: "comment", Text: line[i:]})
			plainStart = n
			i = n
			continue
		}

		// --- Word/keyword detection ---
		if isWordStart(c) {
			flushPlain(i)
			start := i
			for i < n && isWordChar(line[i]) {
				i++
			}
			word := line[start:i]
			if codeKeywords[word] {
				tokens = append(tokens, Token{Type: "keyword", Text: word})
			} else {
				tokens = append(tokens, Token{Type: "plain", Text: word})
			}
			plainStart = i
			continue
		}

		// --- Number detection ---
		if c >= '0' && c <= '9' {
			flushPlain(i)
			start := i
			i++
			sawDot := false
			for i < n {
				if line[i] >= '0' && line[i] <= '9' {
					i++
				} else if line[i] == '.' && !sawDot && i+1 < n && line[i+1] >= '0' && line[i+1] <= '9' {
					sawDot = true
					i++
				} else {
					break
				}
			}
			tokens = append(tokens, Token{Type: "number", Text: line[start:i]})
			plainStart = i
			continue
		}

		// Plain character — just advance
		i++
	}

	flushPlain(n)
	return tokens
}

// isCommentStart checks whether position i in line begins a comment marker
// (//, #, or --) at a word boundary. Word boundary means the preceding
// character is whitespace, start-of-line, or semicolon — this avoids false
// positives on URLs (http://), CSS colors without space, and decrement
// operators (x--).
func isCommentStart(line string, i int) bool {
	atBoundary := i == 0
	if i > 0 {
		prev := line[i-1]
		atBoundary = prev == ' ' || prev == '\t' || prev == ';'
	}
	if !atBoundary {
		return false
	}
	if i+1 < len(line) && line[i] == '/' && line[i+1] == '/' {
		return true
	}
	if line[i] == '#' {
		return true
	}
	if i+1 < len(line) && line[i] == '-' && line[i+1] == '-' {
		return true
	}
	return false
}

func isWordStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isWordChar(c byte) bool {
	return isWordStart(c) || (c >= '0' && c <= '9')
}

// tokenizeBlock splits content by newlines and tokenizes each line. Used by
// colorizeCodeBlock to apply syntax highlighting across multi-line content.
func tokenizeBlock(content string) [][]Token {
	lines := strings.Split(content, "\n")
	result := make([][]Token, len(lines))
	for i, line := range lines {
		result[i] = tokenizeLine(line)
	}
	return result
}

// D! id=otok range-end
