// Package turn tracks Herdr work cycles and advances the private last-turn
// baseline without mutating a worktree, index, or public ref.
package turn

import (
	"path/filepath"
	"sync"

	"github.com/josephembrey/reviewr/internal/herdr"
)

type Repository interface {
	Root() string
	ResolveWorktree(path string) (string, bool, error)
	SnapshotTurnWorktree() (string, error)
	WriteTurnBaseline(tree string) error
}

type AgentSource interface {
	Samples() ([]herdr.AgentSample, error)
}

// Report describes externally visible state advanced by one sample.
type Report struct {
	BaselineChanged bool
}

// Host serializes agent sampling, worktree snapshots, and baseline promotion.
type Host struct {
	mu       sync.Mutex
	repo     Repository
	agents   AgentSource
	tracker  tracker
	resolved map[string]bool
}

// NewHost creates one tracker for a reviewed worktree.
func NewHost(repo Repository, agents AgentSource) *Host {
	return &Host{repo: repo, agents: agents, resolved: make(map[string]bool)}
}

// Sample observes one complete Herdr agent listing. Failures conservatively
// hold the previous state and never move the baseline.
func (host *Host) Sample() Report {
	if host == nil || host.repo == nil || host.agents == nil {
		return Report{}
	}
	host.mu.Lock()
	defer host.mu.Unlock()

	samples, err := host.agents.Samples()
	if err != nil {
		return Report{}
	}
	state, complete := host.worktreeState(samples)
	if !complete {
		return Report{}
	}
	transition := host.tracker.observe(state)
	if transition.started {
		if tree, snapshotErr := host.repo.SnapshotTurnWorktree(); snapshotErr == nil {
			host.tracker.candidate = tree
		}
		return Report{}
	}
	if host.tracker.candidate == "" {
		return Report{}
	}
	current, err := host.repo.SnapshotTurnWorktree()
	if err != nil || current == host.tracker.candidate {
		return Report{}
	}
	if err := host.repo.WriteTurnBaseline(host.tracker.candidate); err != nil {
		return Report{}
	}
	host.tracker.candidate = ""
	return Report{BaselineChanged: true}
}

func (host *Host) worktreeState(samples []herdr.AgentSample) (workState, bool) {
	statuses := make([]agentStatus, 0, len(samples))
	for _, sample := range samples {
		if !filepath.IsAbs(sample.CWD) {
			continue
		}
		member, complete := host.member(sample.CWD)
		if !complete {
			return resting, false
		}
		if member {
			statuses = append(statuses, parseStatus(sample.Status))
		}
	}
	return fold(statuses), true
}

func (host *Host) member(cwd string) (bool, bool) {
	if member, known := host.resolved[cwd]; known {
		return member, true
	}
	root, found, err := host.repo.ResolveWorktree(cwd)
	if err != nil {
		return false, false
	}
	if !found {
		return false, true
	}
	member := root == host.repo.Root()
	host.resolved[cwd] = member
	return member, true
}

type agentStatus uint8

const (
	statusUnknown agentStatus = iota
	statusIdle
	statusWorking
	statusBlocked
	statusDone
)

func parseStatus(value string) agentStatus {
	switch value {
	case "idle":
		return statusIdle
	case "working":
		return statusWorking
	case "blocked":
		return statusBlocked
	case "done":
		return statusDone
	default:
		return statusUnknown
	}
}

type workState uint8

const (
	resting workState = iota
	working
	neither
)

func fold(statuses []agentStatus) workState {
	held := false
	for _, status := range statuses {
		if status == statusWorking {
			return working
		}
		held = held || (status != statusIdle && status != statusDone)
	}
	if held {
		return neither
	}
	return resting
}

type tracker struct {
	previousResting bool
	candidate       string
}

type transition struct {
	started bool
}

func (tracker *tracker) observe(state workState) transition {
	result := transition{started: state == working && tracker.previousResting}
	tracker.previousResting = state == resting
	return result
}
