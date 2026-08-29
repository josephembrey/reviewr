package herdr

import "testing"

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want Context
	}{
		{
			name: "hosted",
			env: map[string]string{
				"HERDR_ENV":          "1",
				"HERDR_WORKSPACE_ID": "workspace",
				"HERDR_TAB_ID":       "tab",
				"HERDR_PANE_ID":      "pane",
				"HERDR_SOCKET_PATH":  "/run/herdr.sock",
				"HERDR_BIN_PATH":     "/bin/herdr",
			},
			want: Context{
				hosted:      true,
				workspaceID: "workspace",
				tabID:       "tab",
				paneID:      "pane",
				socketPath:  "/run/herdr.sock",
				binPath:     "/bin/herdr",
			},
		},
		{
			name: "standalone",
			env: map[string]string{
				"HERDR_PANE_ID":  "ignored",
				"HERDR_BIN_PATH": "/bin/ignored",
			},
			want: Context{},
		},
		{
			name: "partial hosted context",
			env:  map[string]string{"HERDR_ENV": "1", "HERDR_PANE_ID": "pane"},
			want: Context{hosted: true, paneID: "pane"},
		},
		{
			name: "non-authoritative marker",
			env:  map[string]string{"HERDR_ENV": "true", "HERDR_PANE_ID": "pane"},
			want: Context{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := Detect(lookup(test.env))
			if got != test.want {
				t.Fatalf("Detect() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestDetectReturnsStableSnapshot(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"HERDR_ENV":      "1",
		"HERDR_PANE_ID":  "first",
		"HERDR_BIN_PATH": "/bin/herdr",
	}

	context := Detect(lookup(env))
	env["HERDR_PANE_ID"] = "second"

	if got := context.PaneID(); got != "first" {
		t.Fatalf("PaneID() = %q, want captured value %q", got, "first")
	}
}

func lookup(env map[string]string) LookupEnv {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}
