package app

import (
	"testing"

	"github.com/josephembrey/reviewr/internal/herdr"
)

func newTestModel(source Source) Model {
	return New(source, herdr.Context{})
}

func TestNewRetainsHerdrStartupSnapshot(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"HERDR_ENV":      "1",
		"HERDR_PANE_ID":  "first",
		"HERDR_BIN_PATH": "/bin/herdr",
	}
	lookup := func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
	host := herdr.Detect(lookup)

	model := New(&fakeSource{}, host)
	env["HERDR_PANE_ID"] = "second"

	if !model.host.Hosted() || model.host.PaneID() != "first" {
		t.Fatalf("model host = (hosted %v, pane %q), want captured hosted pane", model.host.Hosted(), model.host.PaneID())
	}
}
