package herdr

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type commandCall struct {
	name string
	args []string
}

type fakeRunner struct {
	mu        sync.Mutex
	responses [][]byte
	errors    []error
	calls     []commandCall
}

type blockingRunner struct {
	entered chan struct{}
	once    sync.Once
}

func (r *blockingRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	r.once.Do(func() { close(r.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, commandCall{name: name, args: append([]string(nil), args...)})
	index := len(f.calls) - 1
	var output []byte
	if index < len(f.responses) {
		output = f.responses[index]
	}
	if index < len(f.errors) {
		return output, f.errors[index]
	}
	return output, nil
}

func (f *fakeRunner) Calls() []commandCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]commandCall(nil), f.calls...)
}

func TestRuntimeOwnsUnlabeledPaneTitleForProcessLifetime(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{responses: [][]byte{
		[]byte(`{"result":{"pane":{"pane_id":"pane"}}}`),
		nil,
		[]byte(`{"result":{"pane":{"pane_id":"pane","label":"reviewr"}}}`),
	}}
	runtime := newRuntime(hostedContext(), runner, time.Second)

	runtime.Start()
	waitForStart(t, runtime)
	runtime.Close()

	want := []commandCall{
		{name: "/bin/herdr", args: []string{"pane", "get", "pane"}},
		{name: "/bin/herdr", args: []string{"pane", "rename", "pane", "reviewr"}},
		{name: "/bin/herdr", args: []string{"pane", "get", "pane"}},
		{name: "/bin/herdr", args: []string{"pane", "rename", "pane", "--clear"}},
	}
	if got := runner.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestRuntimeStartDoesNotWaitForHerdr(t *testing.T) {
	t.Parallel()
	runner := &blockingRunner{entered: make(chan struct{})}
	runtime := newRuntime(hostedContext(), runner, time.Second)
	returned := make(chan struct{})

	go func() {
		runtime.Start()
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Start waited for the Herdr command")
	}
	select {
	case <-runner.entered:
	case <-time.After(time.Second):
		t.Fatal("Herdr command did not begin")
	}
	runtime.Close()
}

func TestRuntimePreservesExistingPaneTitle(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{responses: [][]byte{
		[]byte(`{"result":{"pane":{"label":"my workspace"}}}`),
	}}
	runtime := newRuntime(hostedContext(), runner, time.Second)

	runtime.Start()
	waitForStart(t, runtime)
	runtime.Close()

	want := []commandCall{{name: "/bin/herdr", args: []string{"pane", "get", "pane"}}}
	if got := runner.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestRuntimePreservesReplacedOwnedTitle(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{responses: [][]byte{
		[]byte(`{"result":{"pane":{}}}`),
		nil,
		[]byte(`{"result":{"pane":{"label":"new owner"}}}`),
	}}
	runtime := newRuntime(hostedContext(), runner, time.Second)

	runtime.Start()
	waitForStart(t, runtime)
	runtime.Close()

	want := []commandCall{
		{name: "/bin/herdr", args: []string{"pane", "get", "pane"}},
		{name: "/bin/herdr", args: []string{"pane", "rename", "pane", "reviewr"}},
		{name: "/bin/herdr", args: []string{"pane", "get", "pane"}},
	}
	if got := runner.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestRuntimeDoesNothingWithoutPaneTitleCapability(t *testing.T) {
	t.Parallel()
	contexts := []Context{
		{},
		{hosted: true, paneID: "pane"},
		{hosted: true, binPath: "/bin/herdr"},
	}
	for _, host := range contexts {
		runner := &fakeRunner{}
		runtime := newRuntime(host, runner, time.Second)
		runtime.Start()
		runtime.Close()
		if got := runner.Calls(); len(got) != 0 {
			t.Fatalf("context %#v made calls %#v", host, got)
		}
	}
}

func TestRuntimeDoesNotClaimTitleWhenInspectionFails(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{errors: []error{errors.New("unavailable")}}
	runtime := newRuntime(hostedContext(), runner, time.Second)
	runtime.Start()
	waitForStart(t, runtime)
	runtime.Close()

	if got := len(runner.Calls()); got != 1 {
		t.Fatalf("call count = %d, want 1", got)
	}
}

func hostedContext() Context {
	return Context{hosted: true, paneID: "pane", binPath: "/bin/herdr"}
}

func waitForStart(t *testing.T, runtime *Runtime) {
	t.Helper()
	runtime.mu.Lock()
	done := runtime.startDone
	runtime.mu.Unlock()
	if done == nil {
		t.Fatal("runtime did not start")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runtime start did not finish")
	}
}
