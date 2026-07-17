package output

import (
	"fmt"
	"testing"
)

func TestTokenizeLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		check func([]Token) error
	}{
		{
			name: "plain_text_no_tokens",
			line: "hello world some random text",
			check: func(toks []Token) error {
				for _, tk := range toks {
					if tk.Type != "plain" {
						return fmt.Errorf("expected all plain tokens, got %+v", toks)
					}
				}
				return nil
			},
		},
		{
			name: "double_quoted_string",
			line: `x := "hello world"`,
			check: func(toks []Token) error {
				hasString := false
				for _, tk := range toks {
					if tk.Type == "string" && tk.Text == `"hello world"` {
						hasString = true
					}
				}
				if !hasString {
					return fmt.Errorf("expected string token %q, got %+v", `"hello world"`, toks)
				}
				return nil
			},
		},
		{
			name: "single_quoted_string",
			line: `x := 'hello'`,
			check: func(toks []Token) error {
				hasString := false
				for _, tk := range toks {
					if tk.Type == "string" && tk.Text == `'hello'` {
						hasString = true
					}
				}
				if !hasString {
					return fmt.Errorf("expected string token %q, got %+v", `'hello'`, toks)
				}
				return nil
			},
		},
		{
			name: "backtick_string",
			line: "x := `hello`",
			check: func(toks []Token) error {
				hasString := false
				for _, tk := range toks {
					if tk.Type == "string" && tk.Text == "`hello`" {
						hasString = true
					}
				}
				if !hasString {
					return fmt.Errorf("expected string token %q, got %+v", "`hello`", toks)
				}
				return nil
			},
		},
		{
			name: "line_comment_double_slash",
			line: "code // this is a comment",
			check: func(toks []Token) error {
				hasComment := false
				for _, tk := range toks {
					if tk.Type == "comment" {
						hasComment = true
					}
				}
				if !hasComment {
					return fmt.Errorf("expected comment token, got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "line_comment_hash",
			line: "# python comment",
			check: func(toks []Token) error {
				if len(toks) != 1 || toks[0].Type != "comment" {
					return fmt.Errorf("expected single comment token, got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "line_comment_double_dash",
			line: "-- lua comment",
			check: func(toks []Token) error {
				if len(toks) != 1 || toks[0].Type != "comment" {
					return fmt.Errorf("expected single comment token, got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "url_not_comment",
			line: `url := "http://example.com/path"`,
			check: func(toks []Token) error {
				for _, tk := range toks {
					if tk.Type == "comment" {
						return fmt.Errorf("URL // should not trigger comment, got %+v", toks)
					}
				}
				return nil
			},
		},
		{
			name: "decrement_not_comment",
			line: "x--",
			check: func(toks []Token) error {
				for _, tk := range toks {
					if tk.Type == "comment" {
						return fmt.Errorf("x-- should not trigger comment, got %+v", toks)
					}
				}
				return nil
			},
		},
		{
			name: "keyword_if",
			line: "if x > 0",
			check: func(toks []Token) error {
				hasKeyword := false
				for _, tk := range toks {
					if tk.Type == "keyword" && tk.Text == "if" {
						hasKeyword = true
					}
				}
				if !hasKeyword {
					return fmt.Errorf("expected keyword 'if', got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "keyword_func",
			line: "func main() {",
			check: func(toks []Token) error {
				hasFunc := false
				for _, tk := range toks {
					if tk.Type == "keyword" && tk.Text == "func" {
						hasFunc = true
					}
				}
				if !hasFunc {
					return fmt.Errorf("expected keyword 'func', got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "keyword_return",
			line: "return nil",
			check: func(toks []Token) error {
				hasReturn := false
				hasNil := false
				for _, tk := range toks {
					if tk.Type == "keyword" && tk.Text == "return" {
						hasReturn = true
					}
					if tk.Type == "keyword" && tk.Text == "nil" {
						hasNil = true
					}
				}
				if !hasReturn || !hasNil {
					return fmt.Errorf("expected keywords 'return' and 'nil', got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "number_literal",
			line: "x := 42",
			check: func(toks []Token) error {
				hasNum := false
				for _, tk := range toks {
					if tk.Type == "number" && tk.Text == "42" {
						hasNum = true
					}
				}
				if !hasNum {
					return fmt.Errorf("expected number '42', got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "float_literal",
			line: "pi := 3.14",
			check: func(toks []Token) error {
				hasNum := false
				for _, tk := range toks {
					if tk.Type == "number" && tk.Text == "3.14" {
						hasNum = true
					}
				}
				if !hasNum {
					return fmt.Errorf("expected number '3.14', got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "mixed_line",
			line: `func test() { return "hello" } // comment`,
			check: func(toks []Token) error {
				types := map[string]bool{}
				for _, tk := range toks {
					types[tk.Type] = true
				}
				for _, want := range []string{"keyword", "string", "comment"} {
					if !types[want] {
						return fmt.Errorf("expected token type %q in mixed line, got types: %+v", want, types)
					}
				}
				return nil
			},
		},
		{
			name: "escaped_quote_in_string",
			line: `x := "he said \"hello\""`,
			check: func(toks []Token) error {
				// The escaped quotes should NOT end the string
				hasString := false
				for _, tk := range toks {
					if tk.Type == "string" {
						hasString = true
					}
				}
				if !hasString {
					return fmt.Errorf("expected string token with escaped quotes, got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "empty_line",
			line: "",
			check: func(toks []Token) error {
				if len(toks) != 0 {
					return fmt.Errorf("expected 0 tokens for empty line, got %+v", toks)
				}
				return nil
			},
		},
		{
			name: "hash_in_string_not_comment",
			line: `color := "#ff0000"`,
			check: func(toks []Token) error {
				for _, tk := range toks {
					if tk.Type == "comment" {
						return fmt.Errorf("# inside string should not be comment, got %+v", toks)
					}
				}
				return nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			toks := tokenizeLine(tc.line)
			if err := tc.check(toks); err != nil {
				t.Errorf("tokenizeLine(%q): %s\n  tokens: %+v", tc.line, err, toks)
			}
		})
	}
}

func TestTokenizeLineReconstructs(t *testing.T) {
	// The concatenation of all token texts must equal the original line.
	// This is a critical invariant for the guardrail property.
	lines := []string{
		`func main() {`,
		`    // comment line`,
		`    x := "string with spaces"`,
		`    return 42 + 3.14`,
		`    # python comment`,
		`    fmt.Println("hello \"world\"")`,
		`    url := "http://example.com"`,
		`    count--`,
		``,
		`-- lua/sql comment`,
		`    if err != nil { return err }`,
	}
	for _, line := range lines {
		toks := tokenizeLine(line)
		var reconstructed string
		for _, tk := range toks {
			reconstructed += tk.Text
		}
		if reconstructed != line {
			t.Errorf("reconstruction mismatch\n  input:    %q\n  recon:    %q\n  tokens:   %+v", line, reconstructed, toks)
		}
	}
}
