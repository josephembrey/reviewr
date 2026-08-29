package main

import (
	"strings"
	"testing"
)

func TestRunRejectsExtraArguments(t *testing.T) {
	t.Parallel()
	err := run([]string{"one", "two"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("run() error = %v, want usage error", err)
	}
}
