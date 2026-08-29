package git

import (
	"reflect"
	"testing"
)

func TestParseNUL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data string
		want []string
	}{
		{name: "empty"},
		{name: "terminated", data: "plain.txt\x00dir/with space.go\x00日本語.md\x00", want: []string{"plain.txt", "dir/with space.go", "日本語.md"}},
		{name: "embedded newline", data: "line\nbreak.txt\x00other\x00", want: []string{"line\nbreak.txt", "other"}},
		{name: "missing final delimiter", data: "first\x00last", want: []string{"first", "last"}},
		{name: "empty records ignored", data: "first\x00\x00last\x00", want: []string{"first", "last"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := ParseNUL([]byte(test.data)); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ParseNUL() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestOptionalLocksEnvironment(t *testing.T) {
	t.Parallel()
	got := withOptionalLocksDisabled([]string{"A=1", "GIT_OPTIONAL_LOCKS=1", "B=2"})
	want := []string{"A=1", "B=2", "GIT_OPTIONAL_LOCKS=0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("withOptionalLocksDisabled() = %#v, want %#v", got, want)
	}
}
