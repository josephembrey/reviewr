package herdr

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestAgentSamplerReturnsOnlyOtherAgentPanes(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{responses: [][]byte{[]byte(`{"result":{"agents":[
		{"agent":"codex","agent_status":"working","pane_id":"agent-a","cwd":"/work/one"},
		{"agent":"codex","agent_status":"idle","pane_id":"pane","cwd":"/work/self"},
		{"agent_status":"unknown","pane_id":"shell","cwd":"/work/shell"},
		{"agent":"claude","agent_status":"blocked","pane_id":"agent-b"}
	]}}`)}}
	sampler := &AgentSampler{context: hostedContext(), runner: runner, timeout: time.Second}

	got, err := sampler.Samples()
	if err != nil {
		t.Fatal(err)
	}
	want := []AgentSample{{Status: "working", CWD: "/work/one"}, {Status: "blocked"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Samples() = %#v, want %#v", got, want)
	}
	if calls := runner.Calls(); !reflect.DeepEqual(calls, []commandCall{{name: "/bin/herdr", args: []string{"agent", "list"}}}) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestAgentSamplerUnavailableAndMalformedResponsesFailClosed(t *testing.T) {
	t.Parallel()
	unavailableRunner := &fakeRunner{}
	unavailable := &AgentSampler{context: Context{}, runner: unavailableRunner, timeout: time.Second}
	if _, err := unavailable.Samples(); !errors.Is(err, ErrUnavailable) || len(unavailableRunner.Calls()) != 0 {
		t.Fatalf("unavailable Samples() = %v, calls %#v", err, unavailableRunner.Calls())
	}

	malformed := &AgentSampler{
		context: hostedContext(), runner: &fakeRunner{responses: [][]byte{[]byte(`{"result":`)}}, timeout: time.Second,
	}
	if _, err := malformed.Samples(); err == nil {
		t.Fatal("malformed agent response was accepted")
	}
	missing := &AgentSampler{
		context: hostedContext(), runner: &fakeRunner{responses: [][]byte{[]byte(`{"result":{}}`)}}, timeout: time.Second,
	}
	if _, err := missing.Samples(); err == nil {
		t.Fatal("agent response without an agents array was accepted")
	}
}
