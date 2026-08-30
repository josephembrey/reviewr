package ui

import (
	"image/color"
	"path"
	"strings"

	"charm.land/lipgloss/v2"
)

// The icon lookup order and glyphs are adapted from Bontree's icons package. Keep this resolver
// deliberately small and semantic: exact project filenames win, then case-insensitive extensions,
// then stable hidden-file and ordinary-file fallbacks.
const (
	closedFolderIcon = ""
	openFolderIcon   = ""
	fileIcon         = ""
	configFileIcon   = ""
)

type fileIconTone uint8

const (
	fileIconNeutral fileIconTone = iota
	fileIconRed
	fileIconGreen
	fileIconYellow
	fileIconOrange
	fileIconPurple
	fileIconBlue
	fileIconCyan
	fileIconNix
	fileIconDirectory
)

type fileTreeIcon struct {
	glyph string
	tone  fileIconTone
}

func treeDirectoryIcon(expanded bool) fileTreeIcon {
	if expanded {
		return fileTreeIcon{glyph: openFolderIcon, tone: fileIconDirectory}
	}
	return fileTreeIcon{glyph: closedFolderIcon, tone: fileIconDirectory}
}

func treeFileIcon(name string) fileTreeIcon {
	name = path.Base(name)
	if icon, ok := fileIconByName(name); ok {
		return icon
	}
	if extension := strings.TrimPrefix(path.Ext(name), "."); extension != "" {
		if icon, ok := fileIconByExtension(strings.ToLower(extension)); ok {
			return icon
		}
	}
	if strings.HasPrefix(name, ".") {
		return fileTreeIcon{glyph: configFileIcon, tone: fileIconYellow}
	}
	return fileTreeIcon{glyph: fileIcon, tone: fileIconNeutral}
}

func fileIconByName(name string) (fileTreeIcon, bool) {
	var icon fileTreeIcon
	switch name {
	case "Makefile", "makefile", "CMakeLists.txt", "justfile", "Justfile", ".editorconfig",
		".prettierrc", ".eslintrc", ".eslintrc.js", ".eslintrc.json":
		icon = fileTreeIcon{glyph: configFileIcon, tone: fileIconYellow}
	case "Dockerfile", "dockerfile", "docker-compose.yml", "docker-compose.yaml":
		icon = fileTreeIcon{glyph: "", tone: fileIconCyan}
	case ".gitignore", ".gitmodules", ".gitattributes":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "go.mod", "go.sum":
		icon = fileTreeIcon{glyph: "", tone: fileIconCyan}
	case "Cargo.toml", "Cargo.lock":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "package.json", "package-lock.json":
		icon = fileTreeIcon{glyph: "", tone: fileIconGreen}
	case "tsconfig.json":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "webpack.config.js":
		icon = fileTreeIcon{glyph: "", tone: fileIconYellow}
	case "LICENSE", "license":
		icon = fileTreeIcon{glyph: "", tone: fileIconNeutral}
	case "README.md", "readme.md":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case ".env", ".env.local", ".env.development", ".env.production":
		icon = fileTreeIcon{glyph: "", tone: fileIconYellow}
	case ".envrc":
		icon = fileTreeIcon{glyph: "", tone: fileIconGreen}
	case "Gemfile", "Rakefile":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "requirements.txt", "setup.py", "Pipfile":
		icon = fileTreeIcon{glyph: "", tone: fileIconYellow}
	case "flake.nix", "flake.lock", "default.nix", "shell.nix":
		icon = fileTreeIcon{glyph: "", tone: fileIconNix}
	default:
		return fileTreeIcon{}, false
	}
	return icon, true
}

func fileIconByExtension(extension string) (fileTreeIcon, bool) {
	var icon fileTreeIcon
	switch extension {
	case "go":
		icon = fileTreeIcon{glyph: "", tone: fileIconCyan}
	case "nix":
		icon = fileTreeIcon{glyph: "", tone: fileIconNix}
	case "py":
		icon = fileTreeIcon{glyph: "", tone: fileIconYellow}
	case "js":
		icon = fileTreeIcon{glyph: "", tone: fileIconYellow}
	case "ts":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "tsx", "jsx":
		icon = fileTreeIcon{glyph: "", tone: fileIconCyan}
	case "rs":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "rb":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "java":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "c", "h":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "cpp", "cc", "hpp":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "cs":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "swift":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "kt":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "lua":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "php":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "zig":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "hs":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "ex", "exs":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "sh", "bash", "zsh", "fish", "ps1", "bat":
		icon = fileTreeIcon{glyph: "", tone: fileIconGreen}
	case "html", "htm":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "css", "scss", "sass", "less":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "vue":
		icon = fileTreeIcon{glyph: "", tone: fileIconGreen}
	case "svelte":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "json":
		icon = fileTreeIcon{glyph: "", tone: fileIconYellow}
	case "yaml", "yml":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "toml":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "xml":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "csv":
		icon = fileTreeIcon{glyph: "", tone: fileIconGreen}
	case "sql":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "graphql":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "md":
		icon = fileTreeIcon{glyph: "", tone: fileIconBlue}
	case "txt", "rst", "tex":
		icon = fileTreeIcon{glyph: "", tone: fileIconNeutral}
	case "pdf":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "png", "jpg", "jpeg", "gif", "svg", "ico", "webp", "bmp":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "zip", "tar", "gz", "bz2", "xz", "rar", "7z":
		icon = fileTreeIcon{glyph: "", tone: fileIconOrange}
	case "lock", "sum":
		icon = fileTreeIcon{glyph: "", tone: fileIconNeutral}
	case "env":
		icon = fileTreeIcon{glyph: "", tone: fileIconYellow}
	case "ini", "cfg", "conf":
		icon = fileTreeIcon{glyph: configFileIcon, tone: fileIconYellow}
	case "o", "so", "dll", "exe":
		icon = fileTreeIcon{glyph: "", tone: fileIconRed}
	case "wasm":
		icon = fileTreeIcon{glyph: "", tone: fileIconPurple}
	case "log":
		icon = fileTreeIcon{glyph: "", tone: fileIconNeutral}
	default:
		return fileTreeIcon{}, false
	}
	return icon, true
}

var (
	fileIconRedColor    = lipgloss.Color("#E06C75")
	fileIconGreenColor  = lipgloss.Color("#98C379")
	fileIconYellowColor = lipgloss.Color("#E5C07B")
	fileIconOrangeColor = lipgloss.Color("#D19A66")
	fileIconPurpleColor = lipgloss.Color("#C678DD")
	fileIconBlueColor   = lipgloss.Color("#61AFEF")
	fileIconCyanColor   = lipgloss.Color("#56B6C2")
	nixIconBlueColor    = lipgloss.Color("#7EBAE4")
	// File-type icons above intentionally punch through the terminal palette.
	// Match the legacy terminal palette: directories carry a bright-blue
	// identity accent, neutral icons and ignored rows use readable ANSI white,
	// and only truly quiet metadata falls through to BrightBlack.
	directoryTreeColor = lipgloss.BrightBlue
	ignoredTreeColor   = lipgloss.White
	ignoredTreeStyle   = lipgloss.NewStyle().Foreground(ignoredTreeColor)
)

// treeRowStyleLayers is the narrow merge seam for later status and ignored metadata. Status owns
// only the reserved marker and an optional filename accent; it never replaces the filetype icon.
// Ignored is the stronger outer content layer, while selection remains a terminal background layer.
type treeRowStyleLayers struct {
	statusMarker string
	statusAccent treeStatusAccent
	ignored      bool
	selected     bool
	focused      bool
}

type treeStatusAccent uint8

const (
	treeStatusNone treeStatusAccent = iota
	treeStatusAdded
	treeStatusModified
	treeStatusDeleted
	treeStatusRenamed
	treeStatusUntracked
)

type resolvedTreeRowStyles struct {
	marker   lipgloss.Style
	icon     lipgloss.Style
	filename lipgloss.Style
	row      lipgloss.Style
}

func resolveTreeRowStyles(item NavigatorRow, icon fileTreeIcon, layers treeRowStyleLayers) resolvedTreeRowStyles {
	styles := resolvedTreeRowStyles{
		marker: mutedStyle,
		icon:   lipgloss.NewStyle().Foreground(fileTreeIconColor(icon.tone)),
	}
	if item.Directory {
		styles.marker = lipgloss.NewStyle().Foreground(directoryTreeColor)
		styles.filename = lipgloss.NewStyle().Foreground(directoryTreeColor).Bold(true)
	}
	if color, ok := treeStatusColor(layers.statusAccent); ok {
		styles.marker = lipgloss.NewStyle().Foreground(color)
		styles.filename = lipgloss.NewStyle().Foreground(color)
	}
	if layers.ignored {
		styles.marker = ignoredTreeStyle
		styles.icon = ignoredTreeStyle
		styles.filename = ignoredTreeStyle
	}
	if layers.selected {
		styles.row = treeSelectionStyle(layers.focused)
	}
	return styles
}

func treeSelectionStyle(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Black).
		Background(lipgloss.White).
		Bold(focused)
}

func fileTreeIconColor(tone fileIconTone) color.Color {
	switch tone {
	case fileIconRed:
		return fileIconRedColor
	case fileIconGreen:
		return fileIconGreenColor
	case fileIconYellow:
		return fileIconYellowColor
	case fileIconOrange:
		return fileIconOrangeColor
	case fileIconPurple:
		return fileIconPurpleColor
	case fileIconBlue:
		return fileIconBlueColor
	case fileIconCyan:
		return fileIconCyanColor
	case fileIconNix:
		return nixIconBlueColor
	case fileIconDirectory:
		return directoryTreeColor
	default:
		return secondaryColor
	}
}

func treeStatusColor(accent treeStatusAccent) (color.Color, bool) {
	switch accent {
	case treeStatusAdded:
		return lipgloss.BrightGreen, true
	case treeStatusModified:
		return lipgloss.BrightBlue, true
	case treeStatusDeleted:
		return lipgloss.BrightRed, true
	case treeStatusRenamed:
		return lipgloss.BrightMagenta, true
	case treeStatusUntracked:
		return lipgloss.BrightGreen, true
	default:
		return nil, false
	}
}
