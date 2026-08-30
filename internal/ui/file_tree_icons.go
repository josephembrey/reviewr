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

var fileIconsByName = map[string]fileTreeIcon{
	"Makefile":            {glyph: configFileIcon, tone: fileIconYellow},
	"makefile":            {glyph: configFileIcon, tone: fileIconYellow},
	"CMakeLists.txt":      {glyph: configFileIcon, tone: fileIconYellow},
	"justfile":            {glyph: configFileIcon, tone: fileIconYellow},
	"Justfile":            {glyph: configFileIcon, tone: fileIconYellow},
	".editorconfig":       {glyph: configFileIcon, tone: fileIconYellow},
	".prettierrc":         {glyph: configFileIcon, tone: fileIconYellow},
	".eslintrc":           {glyph: configFileIcon, tone: fileIconYellow},
	".eslintrc.js":        {glyph: configFileIcon, tone: fileIconYellow},
	".eslintrc.json":      {glyph: configFileIcon, tone: fileIconYellow},
	"Dockerfile":          {glyph: "", tone: fileIconCyan},
	"dockerfile":          {glyph: "", tone: fileIconCyan},
	"docker-compose.yml":  {glyph: "", tone: fileIconCyan},
	"docker-compose.yaml": {glyph: "", tone: fileIconCyan},
	".gitignore":          {glyph: "", tone: fileIconRed},
	".gitmodules":         {glyph: "", tone: fileIconRed},
	".gitattributes":      {glyph: "", tone: fileIconRed},
	"go.mod":              {glyph: "", tone: fileIconCyan},
	"go.sum":              {glyph: "", tone: fileIconCyan},
	"Cargo.toml":          {glyph: "", tone: fileIconOrange},
	"Cargo.lock":          {glyph: "", tone: fileIconOrange},
	"package.json":        {glyph: "", tone: fileIconGreen},
	"package-lock.json":   {glyph: "", tone: fileIconGreen},
	"tsconfig.json":       {glyph: "", tone: fileIconBlue},
	"webpack.config.js":   {glyph: "", tone: fileIconYellow},
	"LICENSE":             {glyph: "", tone: fileIconNeutral},
	"license":             {glyph: "", tone: fileIconNeutral},
	"README.md":           {glyph: "", tone: fileIconBlue},
	"readme.md":           {glyph: "", tone: fileIconBlue},
	".env":                {glyph: "", tone: fileIconYellow},
	".env.local":          {glyph: "", tone: fileIconYellow},
	".env.development":    {glyph: "", tone: fileIconYellow},
	".env.production":     {glyph: "", tone: fileIconYellow},
	".envrc":              {glyph: "", tone: fileIconGreen},
	"Gemfile":             {glyph: "", tone: fileIconRed},
	"Rakefile":            {glyph: "", tone: fileIconRed},
	"requirements.txt":    {glyph: "", tone: fileIconYellow},
	"setup.py":            {glyph: "", tone: fileIconYellow},
	"Pipfile":             {glyph: "", tone: fileIconYellow},
	"flake.nix":           {glyph: "", tone: fileIconNix},
	"flake.lock":          {glyph: "", tone: fileIconNix},
	"default.nix":         {glyph: "", tone: fileIconNix},
	"shell.nix":           {glyph: "", tone: fileIconNix},
}

var fileIconsByExtension = map[string]fileTreeIcon{
	"go":      {glyph: "", tone: fileIconCyan},
	"nix":     {glyph: "", tone: fileIconNix},
	"py":      {glyph: "", tone: fileIconYellow},
	"js":      {glyph: "", tone: fileIconYellow},
	"ts":      {glyph: "", tone: fileIconBlue},
	"tsx":     {glyph: "", tone: fileIconCyan},
	"jsx":     {glyph: "", tone: fileIconCyan},
	"rs":      {glyph: "", tone: fileIconOrange},
	"rb":      {glyph: "", tone: fileIconRed},
	"java":    {glyph: "", tone: fileIconRed},
	"c":       {glyph: "", tone: fileIconBlue},
	"h":       {glyph: "", tone: fileIconBlue},
	"cpp":     {glyph: "", tone: fileIconBlue},
	"cc":      {glyph: "", tone: fileIconBlue},
	"hpp":     {glyph: "", tone: fileIconBlue},
	"cs":      {glyph: "", tone: fileIconPurple},
	"swift":   {glyph: "", tone: fileIconOrange},
	"kt":      {glyph: "", tone: fileIconPurple},
	"lua":     {glyph: "", tone: fileIconBlue},
	"php":     {glyph: "", tone: fileIconPurple},
	"zig":     {glyph: "", tone: fileIconOrange},
	"hs":      {glyph: "", tone: fileIconPurple},
	"ex":      {glyph: "", tone: fileIconPurple},
	"exs":     {glyph: "", tone: fileIconPurple},
	"sh":      {glyph: "", tone: fileIconGreen},
	"bash":    {glyph: "", tone: fileIconGreen},
	"zsh":     {glyph: "", tone: fileIconGreen},
	"fish":    {glyph: "", tone: fileIconGreen},
	"ps1":     {glyph: "", tone: fileIconGreen},
	"bat":     {glyph: "", tone: fileIconGreen},
	"html":    {glyph: "", tone: fileIconOrange},
	"htm":     {glyph: "", tone: fileIconOrange},
	"css":     {glyph: "", tone: fileIconBlue},
	"scss":    {glyph: "", tone: fileIconBlue},
	"sass":    {glyph: "", tone: fileIconBlue},
	"less":    {glyph: "", tone: fileIconBlue},
	"vue":     {glyph: "", tone: fileIconGreen},
	"svelte":  {glyph: "", tone: fileIconRed},
	"json":    {glyph: "", tone: fileIconYellow},
	"yaml":    {glyph: "", tone: fileIconRed},
	"yml":     {glyph: "", tone: fileIconRed},
	"toml":    {glyph: "", tone: fileIconOrange},
	"xml":     {glyph: "", tone: fileIconOrange},
	"csv":     {glyph: "", tone: fileIconGreen},
	"sql":     {glyph: "", tone: fileIconBlue},
	"graphql": {glyph: "", tone: fileIconPurple},
	"md":      {glyph: "", tone: fileIconBlue},
	"txt":     {glyph: "", tone: fileIconNeutral},
	"rst":     {glyph: "", tone: fileIconNeutral},
	"tex":     {glyph: "", tone: fileIconNeutral},
	"pdf":     {glyph: "", tone: fileIconRed},
	"png":     {glyph: "", tone: fileIconPurple},
	"jpg":     {glyph: "", tone: fileIconPurple},
	"jpeg":    {glyph: "", tone: fileIconPurple},
	"gif":     {glyph: "", tone: fileIconPurple},
	"svg":     {glyph: "", tone: fileIconPurple},
	"ico":     {glyph: "", tone: fileIconPurple},
	"webp":    {glyph: "", tone: fileIconPurple},
	"bmp":     {glyph: "", tone: fileIconPurple},
	"zip":     {glyph: "", tone: fileIconOrange},
	"tar":     {glyph: "", tone: fileIconOrange},
	"gz":      {glyph: "", tone: fileIconOrange},
	"bz2":     {glyph: "", tone: fileIconOrange},
	"xz":      {glyph: "", tone: fileIconOrange},
	"rar":     {glyph: "", tone: fileIconOrange},
	"7z":      {glyph: "", tone: fileIconOrange},
	"lock":    {glyph: "", tone: fileIconNeutral},
	"sum":     {glyph: "", tone: fileIconNeutral},
	"env":     {glyph: "", tone: fileIconYellow},
	"ini":     {glyph: configFileIcon, tone: fileIconYellow},
	"cfg":     {glyph: configFileIcon, tone: fileIconYellow},
	"conf":    {glyph: configFileIcon, tone: fileIconYellow},
	"o":       {glyph: "", tone: fileIconRed},
	"so":      {glyph: "", tone: fileIconRed},
	"dll":     {glyph: "", tone: fileIconRed},
	"exe":     {glyph: "", tone: fileIconRed},
	"wasm":    {glyph: "", tone: fileIconPurple},
	"log":     {glyph: "", tone: fileIconNeutral},
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

var fileIconColors = [...]color.Color{
	fileIconNeutral:   secondaryColor,
	fileIconRed:       fileIconRedColor,
	fileIconGreen:     fileIconGreenColor,
	fileIconYellow:    fileIconYellowColor,
	fileIconOrange:    fileIconOrangeColor,
	fileIconPurple:    fileIconPurpleColor,
	fileIconBlue:      fileIconBlueColor,
	fileIconCyan:      fileIconCyanColor,
	fileIconNix:       nixIconBlueColor,
	fileIconDirectory: directoryTreeColor,
}

func treeDirectoryIcon(expanded bool) fileTreeIcon {
	if expanded {
		return fileTreeIcon{glyph: openFolderIcon, tone: fileIconDirectory}
	}
	return fileTreeIcon{glyph: closedFolderIcon, tone: fileIconDirectory}
}

func treeFileIcon(name string) fileTreeIcon {
	name = path.Base(name)
	if icon, ok := fileIconsByName[name]; ok {
		return icon
	}
	extension := strings.TrimPrefix(path.Ext(name), ".")
	if icon, ok := fileIconsByExtension[strings.ToLower(extension)]; ok {
		return icon
	}
	if strings.HasPrefix(name, ".") {
		return fileTreeIcon{glyph: configFileIcon, tone: fileIconYellow}
	}
	return fileTreeIcon{glyph: fileIcon, tone: fileIconNeutral}
}

func fileTreeIconColor(tone fileIconTone) color.Color {
	if int(tone) >= len(fileIconColors) {
		return secondaryColor
	}
	return fileIconColors[tone]
}
