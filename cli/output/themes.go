package output

// D! id=othms range-start

// AllThemes maps theme names to Theme values. lookupTheme looks up by name
// (case-sensitive). Unknown names return (Theme{}, false); callers fall back
// to DefaultTheme.
var AllThemes = map[string]Theme{
	"default":         DefaultTheme,
	"minimal":         MinimalTheme,
	"monochrome":      MonochromeTheme,
	"high-contrast":   HighContrastTheme,
	"dark":            DarkTheme,
	"light":           LightTheme,
	"protanopia":      ProtanopiaTheme,
	"solarized-dark":  SolarizedDarkTheme,
	"solarized-light": SolarizedLightTheme,
	"gruvbox":         GruvboxTheme,
	"nord":            NordTheme,
	"dracula":         DraculaTheme,
}

func lookupTheme(name string) (Theme, bool) {
	t, ok := AllThemes[name]
	return t, ok
}

// DefaultTheme is the vibrant bright-ANSI theme. Bright colors (91-97) for
// maximum saturation. Marker/spec IDs in blue/magenta bold for distinct
// visual identity. Metadata (filepaths, hashes, line numbers) dimmed.
var DefaultTheme = Theme{
	Name:          "default",
	MarkerID:      Style{Color: "94", Bold: true},
	SpecID:        Style{Color: "95", Bold: true},
	Filepath:      Style{Dim: true},
	LineNumber:    Style{Dim: true},
	Hash:          Style{Dim: true},
	StatusOK:      Style{Color: "92"},
	StatusWarn:    Style{Color: "93"},
	StatusError:   Style{Color: "91"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "92"},
	Hint:          Style{Color: "96"},
	DiffAdd:       Style{Color: "92"},
	DiffRemove:    Style{Color: "91"},
	DiffHunk:      Style{Color: "96", Bold: true},
}

// MinimalTheme uses only basic status colors (green/yellow/red from the
// 31-37 range). Everything else — IDs, filepaths, hashes — is unstyled.
var MinimalTheme = Theme{
	Name:          "minimal",
	StatusOK:      Style{Color: "32"},
	StatusWarn:    Style{Color: "33"},
	StatusError:   Style{Color: "31"},
	SectionHeader: Style{Bold: true},
	DiffAdd:       Style{Color: "32"},
	DiffRemove:    Style{Color: "31"},
	DiffHunk:      Style{Color: "36"},
}

// MonochromeTheme uses zero color — only bold and dim for hierarchy.
var MonochromeTheme = Theme{
	Name:          "monochrome",
	Filepath:      Style{Dim: true},
	LineNumber:    Style{Dim: true},
	Hash:          Style{Dim: true},
	SectionHeader: Style{Bold: true},
	DiffHunk:      Style{Bold: true},
}

// HighContrastTheme is like Default but with no dimming — metadata gets
// bright cyan instead of dim, for maximum visibility.
var HighContrastTheme = Theme{
	Name:          "high-contrast",
	MarkerID:      Style{Color: "94", Bold: true},
	SpecID:        Style{Color: "95", Bold: true},
	Filepath:      Style{Color: "96"},
	LineNumber:    Style{Color: "96"},
	Hash:          Style{Color: "96"},
	StatusOK:      Style{Color: "92"},
	StatusWarn:    Style{Color: "93"},
	StatusError:   Style{Color: "91"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "92"},
	Hint:          Style{Color: "96"},
	DiffAdd:       Style{Color: "92"},
	DiffRemove:    Style{Color: "91"},
	DiffHunk:      Style{Color: "96", Bold: true},
}

// DarkTheme is tuned for dark terminal backgrounds — bright colors that
// pop against dark surfaces.
var DarkTheme = Theme{
	Name:          "dark",
	MarkerID:      Style{Color: "94", Bold: true},
	SpecID:        Style{Color: "95", Bold: true},
	Filepath:      Style{Color: "96"},
	LineNumber:    Style{Color: "96"},
	Hash:          Style{Color: "96"},
	StatusOK:      Style{Color: "92"},
	StatusWarn:    Style{Color: "93"},
	StatusError:   Style{Color: "91"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "92"},
	Hint:          Style{Color: "96"},
	DiffAdd:       Style{Color: "92"},
	DiffRemove:    Style{Color: "91"},
	DiffHunk:      Style{Color: "96", Bold: true},
}

// LightTheme is tuned for light terminal backgrounds — basic colors (31-37)
// that read well on white/light surfaces, no dimming (dim is invisible on
// light backgrounds).
var LightTheme = Theme{
	Name:          "light",
	MarkerID:      Style{Color: "34", Bold: true},
	SpecID:        Style{Color: "35", Bold: true},
	Filepath:      Style{Color: "36"},
	LineNumber:    Style{Color: "36"},
	Hash:          Style{Color: "36"},
	StatusOK:      Style{Color: "32"},
	StatusWarn:    Style{Color: "33"},
	StatusError:   Style{Color: "31"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "32"},
	Hint:          Style{Color: "36"},
	DiffAdd:       Style{Color: "32"},
	DiffRemove:    Style{Color: "31"},
	DiffHunk:      Style{Color: "36", Bold: true},
}

// ProtanopiaTheme avoids red/green entirely — uses blue/yellow/cyan which
// protanopes (red-green color blind) distinguish reliably.
// status_ok = blue (not green), status_error = yellow (not red),
// diff_add = cyan, diff_remove = yellow.
var ProtanopiaTheme = Theme{
	Name:          "protanopia",
	MarkerID:      Style{Color: "94", Bold: true},
	SpecID:        Style{Color: "95", Bold: true},
	Filepath:      Style{Dim: true},
	LineNumber:    Style{Dim: true},
	Hash:          Style{Dim: true},
	StatusOK:      Style{Color: "94"},
	StatusWarn:    Style{Color: "93"},
	StatusError:   Style{Color: "93"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "94"},
	Hint:          Style{Color: "96"},
	DiffAdd:       Style{Color: "96"},
	DiffRemove:    Style{Color: "93"},
	DiffHunk:      Style{Color: "96", Bold: true},
}

// SolarizedDarkTheme uses the Solarized Dark palette by Ethan Schoonover (MIT).
// Color values: blue=#268bd2 green=#859900 yellow=#b58900 red=#dc322f
// magenta=#d33682 cyan=#2aa198.
var SolarizedDarkTheme = Theme{
	Name:          "solarized-dark",
	MarkerID:      Style{Color: "38;5;33", Bold: true},
	SpecID:        Style{Color: "38;5;162", Bold: true},
	Filepath:      Style{Dim: true},
	LineNumber:    Style{Dim: true},
	Hash:          Style{Dim: true},
	StatusOK:      Style{Color: "38;5;100"},
	StatusWarn:    Style{Color: "38;5;136"},
	StatusError:   Style{Color: "38;5;124"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "38;5;100"},
	Hint:          Style{Color: "38;5;37"},
	DiffAdd:       Style{Color: "38;5;100"},
	DiffRemove:    Style{Color: "38;5;124"},
	DiffHunk:      Style{Color: "38;5;37", Bold: true},
}

// SolarizedLightTheme uses the Solarized Light palette by Ethan Schoonover (MIT).
// Darker shades for readability on light backgrounds.
var SolarizedLightTheme = Theme{
	Name:          "solarized-light",
	MarkerID:      Style{Color: "38;5;26", Bold: true},
	SpecID:        Style{Color: "38;5;96", Bold: true},
	Filepath:      Style{Color: "38;5;244"},
	LineNumber:    Style{Color: "38;5;244"},
	Hash:          Style{Color: "38;5;244"},
	StatusOK:      Style{Color: "38;5;64"},
	StatusWarn:    Style{Color: "38;5;100"},
	StatusError:   Style{Color: "38;5;88"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "38;5;64"},
	Hint:          Style{Color: "38;5;30"},
	DiffAdd:       Style{Color: "38;5;64"},
	DiffRemove:    Style{Color: "38;5;88"},
	DiffHunk:      Style{Color: "38;5;30", Bold: true},
}

// GruvboxTheme uses the Gruvbox Dark palette by Pavel Pertsev (MIT).
// Color values: red=#fb4934 green=#b8bb26 yellow=#fabd2f blue=#83a598
// purple=#d3869b aqua=#8ec07c orange=#fe8019.
var GruvboxTheme = Theme{
	Name:          "gruvbox",
	MarkerID:      Style{Color: "38;5;109", Bold: true},
	SpecID:        Style{Color: "38;5;175", Bold: true},
	Filepath:      Style{Dim: true},
	LineNumber:    Style{Dim: true},
	Hash:          Style{Dim: true},
	StatusOK:      Style{Color: "38;5;142"},
	StatusWarn:    Style{Color: "38;5;214"},
	StatusError:   Style{Color: "38;5;203"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "38;5;142"},
	Hint:          Style{Color: "38;5;108"},
	DiffAdd:       Style{Color: "38;5;142"},
	DiffRemove:    Style{Color: "38;5;203"},
	DiffHunk:      Style{Color: "38;5;108", Bold: true},
}

// NordTheme uses the Nord palette by Sven Greb / arcticicestudio (MIT).
// Color values: red=#bf616a green=#a3be8c yellow=#ebcb8b blue=#81a1c1
// magenta=#b48ead cyan=#88c0d0.
var NordTheme = Theme{
	Name:          "nord",
	MarkerID:      Style{Color: "38;5;110", Bold: true},
	SpecID:        Style{Color: "38;5;139", Bold: true},
	Filepath:      Style{Dim: true},
	LineNumber:    Style{Dim: true},
	Hash:          Style{Dim: true},
	StatusOK:      Style{Color: "38;5;108"},
	StatusWarn:    Style{Color: "38;5;179"},
	StatusError:   Style{Color: "38;5;174"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "38;5;108"},
	Hint:          Style{Color: "38;5;38"},
	DiffAdd:       Style{Color: "38;5;108"},
	DiffRemove:    Style{Color: "38;5;174"},
	DiffHunk:      Style{Color: "38;5;38", Bold: true},
}

// DraculaTheme uses the Dracula palette by Zeno Rocha (MIT).
// Color values: cyan=#8be9fd green=#50fa7b orange=#ffb86c pink=#ff79c6
// purple=#bd93f9 red=#ff5555 yellow=#f1fa8c.
var DraculaTheme = Theme{
	Name:          "dracula",
	MarkerID:      Style{Color: "38;5;117", Bold: true},
	SpecID:        Style{Color: "38;5;212", Bold: true},
	Filepath:      Style{Dim: true},
	LineNumber:    Style{Dim: true},
	Hash:          Style{Dim: true},
	StatusOK:      Style{Color: "38;5;84"},
	StatusWarn:    Style{Color: "38;5;215"},
	StatusError:   Style{Color: "38;5;203"},
	SectionHeader: Style{Bold: true},
	Command:       Style{Color: "38;5;84"},
	Hint:          Style{Color: "38;5;141"},
	DiffAdd:       Style{Color: "38;5;84"},
	DiffRemove:    Style{Color: "38;5;203"},
	DiffHunk:      Style{Color: "38;5;141", Bold: true},
}

// D! id=othms range-end
