package ui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFileTreeIconResolver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want fileTreeIcon
	}{
		{
			name: "filename override precedes extension",
			path: "nested/Cargo.toml",
			want: fileTreeIcon{glyph: "", tone: fileIconOrange},
		},
		{
			name: "filename override precedes neutral lock extension",
			path: "flake.lock",
			want: fileTreeIcon{glyph: "", tone: fileIconNix},
		},
		{
			name: "common root filename",
			path: "package.json",
			want: fileTreeIcon{glyph: "", tone: fileIconGreen},
		},
		{
			name: "source extension",
			path: "internal/ui/render.go",
			want: fileTreeIcon{glyph: "", tone: fileIconCyan},
		},
		{
			name: "extension ignores ASCII case",
			path: "assets/logo.PNG",
			want: fileTreeIcon{glyph: "", tone: fileIconPurple},
		},
		{
			name: "hidden file fallback",
			path: ".toolrc",
			want: fileTreeIcon{glyph: configFileIcon, tone: fileIconYellow},
		},
		{
			name: "unknown extension fallback",
			path: "notes.unknown",
			want: fileTreeIcon{glyph: fileIcon, tone: fileIconNeutral},
		},
		{
			name: "extensionless fallback",
			path: "NOTICE",
			want: fileTreeIcon{glyph: fileIcon, tone: fileIconNeutral},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := treeFileIcon(test.path); got != test.want {
				t.Fatalf("treeFileIcon(%q) = %#v, want %#v", test.path, got, test.want)
			}
		})
	}
}

func TestDirectoryIconResolver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expanded bool
		glyph    string
	}{
		{name: "closed", glyph: closedFolderIcon},
		{name: "open", expanded: true, glyph: openFolderIcon},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := treeDirectoryIcon(test.expanded)
			if got.glyph != test.glyph || got.tone != fileIconDirectory {
				t.Fatalf("treeDirectoryIcon(%v) = %#v, want glyph %q and directory tone", test.expanded, got, test.glyph)
			}
		})
	}
}

func TestNixIconUsesCanonicalTruecolorBlue(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"flake.nix", "flake.lock", "shell.nix", "packages/example.nix"} {
		icon := treeFileIcon(name)
		if icon.glyph != "" || icon.tone != fileIconNix {
			t.Fatalf("treeFileIcon(%q) = %#v, want Nix snowflake", name, icon)
		}
		assertSameColor(t, fileTreeIconColor(icon.tone), color.RGBA{R: 0x7e, G: 0xba, B: 0xe4, A: 0xff})
	}
}

func TestFileTypeIconTruecolorCatalog(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tone fileIconTone
		want color.RGBA
	}{
		{name: "red", tone: fileIconRed, want: color.RGBA{R: 0xe0, G: 0x6c, B: 0x75, A: 0xff}},
		{name: "green", tone: fileIconGreen, want: color.RGBA{R: 0x98, G: 0xc3, B: 0x79, A: 0xff}},
		{name: "yellow", tone: fileIconYellow, want: color.RGBA{R: 0xe5, G: 0xc0, B: 0x7b, A: 0xff}},
		{name: "orange", tone: fileIconOrange, want: color.RGBA{R: 0xd1, G: 0x9a, B: 0x66, A: 0xff}},
		{name: "purple", tone: fileIconPurple, want: color.RGBA{R: 0xc6, G: 0x78, B: 0xdd, A: 0xff}},
		{name: "blue", tone: fileIconBlue, want: color.RGBA{R: 0x61, G: 0xaf, B: 0xef, A: 0xff}},
		{name: "cyan", tone: fileIconCyan, want: color.RGBA{R: 0x56, G: 0xb6, B: 0xc2, A: 0xff}},
		{name: "nix blue", tone: fileIconNix, want: color.RGBA{R: 0x7e, G: 0xba, B: 0xe4, A: 0xff}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := fileTreeIconColor(test.tone)
			if _, isANSI := got.(ansi.BasicColor); isANSI {
				t.Fatalf("file icon %s unexpectedly uses ANSI slot %v", test.name, got)
			}
			assertSameColor(t, got, test.want)
		})
	}
}

func TestNeutralFileIconUsesReadableSecondaryANSI(t *testing.T) {
	t.Parallel()
	icon := treeFileIcon("NOTICE")
	if icon.tone != fileIconNeutral {
		t.Fatalf("neutral icon tone = %v, want neutral", icon.tone)
	}
	assertSameColor(t, fileTreeIconColor(icon.tone), secondaryColor)
}

func TestTreeRowStyleLayersStayIndependent(t *testing.T) {
	t.Parallel()
	file := NavigatorRow{Tree: true, Label: "main.rs"}
	icon := treeFileIcon(file.Label)
	tests := []struct {
		name             string
		item             NavigatorRow
		icon             fileTreeIcon
		layers           treeRowStyleLayers
		wantMarker       color.Color
		wantIcon         color.Color
		wantFilename     color.Color
		wantSelection    bool
		wantFocusedBold  bool
		filenameHasColor bool
	}{
		{
			name:             "ordinary file keeps default filename",
			item:             file,
			icon:             icon,
			wantMarker:       mutedColor,
			wantIcon:         vividOrangeColor,
			filenameHasColor: false,
		},
		{
			name:             "unselected directory uses legacy bright-blue identity",
			item:             NavigatorRow{Tree: true, Label: "src", Directory: true},
			icon:             treeDirectoryIcon(false),
			wantMarker:       directoryTreeColor,
			wantIcon:         directoryTreeColor,
			wantFilename:     directoryTreeColor,
			filenameHasColor: true,
		},
		{
			name:             "selected directory uses black marker and filename",
			item:             NavigatorRow{Tree: true, Label: "src", Directory: true},
			icon:             treeDirectoryIcon(false),
			layers:           treeRowStyleLayers{selected: true, focused: true},
			wantMarker:       lipgloss.Black,
			wantIcon:         directoryTreeColor,
			wantFilename:     lipgloss.Black,
			wantSelection:    true,
			wantFocusedBold:  true,
			filenameHasColor: true,
		},
		{
			name:             "unselected modified status accents marker and filename only",
			item:             file,
			icon:             icon,
			layers:           treeRowStyleLayers{statusAccent: treeStatusModified},
			wantMarker:       lipgloss.BrightBlue,
			wantIcon:         vividOrangeColor,
			wantFilename:     lipgloss.BrightBlue,
			filenameHasColor: true,
		},
		{
			name:             "selected modified status uses black marker and filename",
			item:             file,
			icon:             icon,
			layers:           treeRowStyleLayers{statusAccent: treeStatusModified, selected: true, focused: true},
			wantMarker:       lipgloss.Black,
			wantIcon:         vividOrangeColor,
			wantFilename:     lipgloss.Black,
			wantSelection:    true,
			wantFocusedBold:  true,
			filenameHasColor: true,
		},
		{
			name:             "unselected added status accents marker and filename only",
			item:             file,
			icon:             icon,
			layers:           treeRowStyleLayers{statusAccent: treeStatusAdded},
			wantMarker:       lipgloss.BrightGreen,
			wantIcon:         vividOrangeColor,
			wantFilename:     lipgloss.BrightGreen,
			filenameHasColor: true,
		},
		{
			name:             "selected added status uses black marker and filename",
			item:             file,
			icon:             icon,
			layers:           treeRowStyleLayers{statusAccent: treeStatusAdded, selected: true, focused: true},
			wantMarker:       lipgloss.Black,
			wantIcon:         vividOrangeColor,
			wantFilename:     lipgloss.Black,
			wantSelection:    true,
			wantFocusedBold:  true,
			filenameHasColor: true,
		},
		{
			name:             "unselected ignored layer is stronger than status and filetype",
			item:             file,
			icon:             icon,
			layers:           treeRowStyleLayers{statusAccent: treeStatusDeleted, ignored: true},
			wantMarker:       ignoredTreeColor,
			wantIcon:         ignoredTreeColor,
			wantFilename:     ignoredTreeColor,
			filenameHasColor: true,
		},
		{
			name:             "selected ignored row uses black marker and filename",
			item:             file,
			icon:             icon,
			layers:           treeRowStyleLayers{statusAccent: treeStatusDeleted, ignored: true, selected: true, focused: true},
			wantMarker:       lipgloss.Black,
			wantIcon:         ignoredTreeColor,
			wantFilename:     lipgloss.Black,
			wantSelection:    true,
			wantFocusedBold:  true,
			filenameHasColor: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			styles := resolveTreeRowStyles(test.item, test.icon, test.layers)
			if test.item.Directory && !test.layers.ignored && !styles.filename.GetBold() {
				t.Fatal("ordinary directory name is not bold")
			}
			assertSameColor(t, styles.marker.GetForeground(), test.wantMarker)
			assertSameColor(t, styles.icon.GetForeground(), test.wantIcon)
			_, filenameHasColor := styles.filename.GetForeground().(lipgloss.NoColor)
			filenameHasColor = !filenameHasColor
			if filenameHasColor != test.filenameHasColor {
				t.Fatalf("filename has foreground = %v, want %v", filenameHasColor, test.filenameHasColor)
			}
			if test.filenameHasColor {
				assertSameColor(t, styles.filename.GetForeground(), test.wantFilename)
			}
			if test.layers.ignored && (styles.marker.GetFaint() || styles.icon.GetFaint() || styles.filename.GetFaint()) {
				t.Fatal("ignored content adds faint rendering on top of BrightBlack")
			}
			if styles.row.GetReverse() || styles.row.GetBold() != test.wantFocusedBold {
				t.Fatalf("row reverse=%v bold=%v, want reverse=false bold=%v", styles.row.GetReverse(), styles.row.GetBold(), test.wantFocusedBold)
			}
			_, hasBackground := styles.row.GetBackground().(lipgloss.NoColor)
			if hasBackground == test.wantSelection {
				t.Fatalf("row has background = %v, want %v", !hasBackground, test.wantSelection)
			}
			if test.wantSelection {
				assertSameColor(t, styles.row.GetBackground(), lipgloss.White)
			}
		})
	}
}

func assertSameColor(t *testing.T, got, want color.Color) {
	t.Helper()
	gotRGBA := color.RGBAModel.Convert(got)
	wantRGBA := color.RGBAModel.Convert(want)
	if gotRGBA != wantRGBA {
		t.Fatalf("color = %v, want %v", gotRGBA, wantRGBA)
	}
}
