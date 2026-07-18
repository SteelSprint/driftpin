package output

import "testing"

// TestStyleApply verifies that Style.Apply composes the correct ANSI escape
// sequence from its Color, Bold, and Dim fields. This is the foundational
// unit test for the theming system — if Apply is wrong, every theme is wrong.
func TestStyleApply(t *testing.T) {
	tests := []struct {
		name string
		style Style
		input string
		want  string
	}{
		{
			name:  "zero_value_returns_input_unchanged",
			style: Style{},
			input: "hello",
			want:  "hello",
		},
		{
			name:  "color_only",
			style: Style{Color: "92"},
			input: "green text",
			want:  "\x1b[92mgreen text\x1b[0m",
		},
		{
			name:  "bold_only",
			style: Style{Bold: true},
			input: "bold text",
			want:  "\x1b[1mbold text\x1b[0m",
		},
		{
			name:  "dim_only",
			style: Style{Dim: true},
			input: "dim text",
			want:  "\x1b[2mdim text\x1b[0m",
		},
		{
			name:  "bold_and_color",
			style: Style{Color: "94", Bold: true},
			input: "cdisp",
			want:  "\x1b[1;94mcdisp\x1b[0m",
		},
		{
			name:  "dim_and_color",
			style: Style{Color: "96", Dim: true},
			input: "path/to/file",
			want:  "\x1b[2;96mpath/to/file\x1b[0m",
		},
		{
			name:  "all_three",
			style: Style{Color: "91", Bold: true, Dim: true},
			input: "error",
			want:  "\x1b[2;1;91merror\x1b[0m",
		},
		{
			name:  "256_color",
			style: Style{Color: "38;5;37"},
			input: "solarized green",
			want:  "\x1b[38;5;37msolarized green\x1b[0m",
		},
		{
			name:  "empty_string_input",
			style: Style{Color: "92"},
			input: "",
			want:  "\x1b[92m\x1b[0m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.style.Apply(tc.input)
			if got != tc.want {
				t.Errorf("Style%+v.Apply(%q) = %q, want %q", tc.style, tc.input, got, tc.want)
			}
		})
	}
}

// TestThemeZeroValue verifies that a zero-value Theme (all Styles empty)
// produces no ANSI codes — this is the foundation of the guardrail property.
func TestThemeZeroValue(t *testing.T) {
	theme := Theme{}
	if theme.StatusOK.Apply("text") != "text" {
		t.Error("zero-value Theme.StatusOK should not modify text")
	}
	if theme.MarkerID.Apply("cdisp") != "cdisp" {
		t.Error("zero-value Theme.From should not modify text")
	}
}
