package herdr

import (
	"context"
	"encoding/json"
	"os/exec"
	"sync"
	"time"
)

const (
	paneTitle      = "reviewr"
	commandTimeout = 2 * time.Second
)

type commandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// Runtime owns effects that are available only inside Herdr. It is safe to
// construct and start in standalone mode, where every capability is a no-op.
type Runtime struct {
	context Context
	runner  commandRunner
	timeout time.Duration

	mu          sync.Mutex
	started     bool
	closing     bool
	ownsTitle   bool
	startCancel context.CancelFunc
	startDone   chan struct{}
}

// NewRuntime creates the process-wide Herdr runtime from a startup snapshot.
func NewRuntime(host Context) *Runtime {
	return newRuntime(host, execRunner{}, commandTimeout)
}

func newRuntime(host Context, runner commandRunner, timeout time.Duration) *Runtime {
	return &Runtime{context: host, runner: runner, timeout: timeout}
}

// Context returns the immutable startup snapshot shared with application services.
func (r *Runtime) Context() Context { return r.context }

// Start activates available hosted capabilities without delaying first paint.
func (r *Runtime) Start() {
	if !r.context.canLabelPane() {
		return
	}

	r.mu.Lock()
	if r.started || r.closing {
		r.mu.Unlock()
		return
	}
	r.started = true
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	r.startCancel = cancel
	r.startDone = make(chan struct{})
	done := r.startDone
	r.mu.Unlock()

	go func() {
		defer close(done)
		defer cancel()

		label, err := r.currentPaneLabel(ctx)
		if err != nil || label != "" {
			return
		}
		if _, err := r.run(ctx, "pane", "rename", r.context.PaneID(), paneTitle); err != nil {
			return
		}
		r.mu.Lock()
		r.ownsTitle = true
		r.mu.Unlock()
	}()
}

// Close releases hosted resources on normal shutdown. It never clears a label
// that existed before reviewr or one another actor changed while reviewr ran.
func (r *Runtime) Close() {
	r.mu.Lock()
	if r.closing {
		r.mu.Unlock()
		return
	}
	r.closing = true
	done := r.startDone
	cancel := r.startCancel
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		timer := time.NewTimer(r.timeout)
		select {
		case <-done:
			timer.Stop()
		case <-timer.C:
			return
		}
	}

	r.mu.Lock()
	owned := r.ownsTitle
	r.mu.Unlock()
	if !owned {
		return
	}

	ctx, cancelClear := context.WithTimeout(context.Background(), r.timeout)
	defer cancelClear()
	label, err := r.currentPaneLabel(ctx)
	if err != nil || label != paneTitle {
		return
	}
	_, _ = r.run(ctx, "pane", "rename", r.context.PaneID(), "--clear")
}

func (r *Runtime) currentPaneLabel(ctx context.Context) (string, error) {
	output, err := r.run(ctx, "pane", "get", r.context.PaneID())
	if err != nil {
		return "", err
	}
	var response struct {
		Result struct {
			Pane struct {
				Label string `json:"label"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", err
	}
	return response.Result.Pane.Label, nil
}

func (r *Runtime) run(ctx context.Context, args ...string) ([]byte, error) {
	return r.runner.Run(ctx, r.context.BinPath(), args...)
}
