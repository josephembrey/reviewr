package turn

import (
	"errors"
	"reflect"
	"testing"

	"github.com/josephembrey/reviewr/internal/herdr"
)

type fakeRepository struct {
	root        string
	worktrees   map[string]string
	snapshots   []string
	snapshotErr error
	writes      []string
	writeErr    error
}

func (repo *fakeRepository) Root() string { return repo.root }

func (repo *fakeRepository) ResolveWorktree(path string) (string, bool, error) {
	root, found := repo.worktrees[path]
	return root, found, nil
}

func (repo *fakeRepository) SnapshotTurnWorktree() (string, error) {
	if repo.snapshotErr != nil {
		return "", repo.snapshotErr
	}
	if len(repo.snapshots) == 0 {
		return "", errors.New("unexpected snapshot")
	}
	result := repo.snapshots[0]
	repo.snapshots = repo.snapshots[1:]
	return result, nil
}

func (repo *fakeRepository) WriteTurnBaseline(tree string) error {
	if repo.writeErr != nil {
		return repo.writeErr
	}
	repo.writes = append(repo.writes, tree)
	return nil
}

type fakeAgents struct {
	samples [][]herdr.AgentSample
	errors  []error
	calls   int
}

func (agents *fakeAgents) Samples() ([]herdr.AgentSample, error) {
	index := agents.calls
	agents.calls++
	if index < len(agents.errors) && agents.errors[index] != nil {
		return nil, agents.errors[index]
	}
	if index >= len(agents.samples) {
		return nil, nil
	}
	return agents.samples[index], nil
}

func TestHostPromotesTheTurnStartSnapshotAfterTheFirstChange(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{
		root: "/work/reviewed", worktrees: map[string]string{"/work/reviewed/sub": "/work/reviewed"},
		snapshots: []string{"turn-start", "turn-start", "turn-changed"},
	}
	agents := &fakeAgents{samples: [][]herdr.AgentSample{
		{{Status: "idle", CWD: "/work/reviewed/sub"}},
		{{Status: "working", CWD: "/work/reviewed/sub"}},
		{{Status: "blocked", CWD: "/work/reviewed/sub"}},
		{{Status: "working", CWD: "/work/reviewed/sub"}},
	}}
	host := NewHost(repo, agents)

	if report := host.Sample(); report.BaselineChanged {
		t.Fatal("resting sample changed the baseline")
	}
	if report := host.Sample(); report.BaselineChanged {
		t.Fatal("turn start changed the baseline before an edit")
	}
	if report := host.Sample(); report.BaselineChanged {
		t.Fatal("unchanged permission pause changed the baseline")
	}
	if report := host.Sample(); !report.BaselineChanged {
		t.Fatal("first changed snapshot did not promote the baseline")
	}
	if !reflect.DeepEqual(repo.writes, []string{"turn-start"}) {
		t.Fatalf("baseline writes = %#v", repo.writes)
	}
}

func TestHostTracksTheReviewedWorktreeAsOneTurn(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{
		root: "/work/reviewed",
		worktrees: map[string]string{
			"/work/reviewed/a": "/work/reviewed",
			"/work/reviewed/b": "/work/reviewed",
			"/work/other":      "/work/other",
		},
		snapshots: []string{"before", "after"},
	}
	agents := &fakeAgents{samples: [][]herdr.AgentSample{
		{{Status: "idle", CWD: "/work/reviewed/a"}, {Status: "working", CWD: "/work/other"}},
		{{Status: "working", CWD: "/work/reviewed/a"}, {Status: "idle", CWD: "/work/reviewed/b"}},
		{{Status: "working", CWD: "/work/reviewed/a"}, {Status: "working", CWD: "/work/reviewed/b"}},
	}}
	host := NewHost(repo, agents)

	host.Sample()
	host.Sample()
	if report := host.Sample(); !report.BaselineChanged {
		t.Fatal("two-agent worktree did not promote its one turn baseline")
	}
	if !reflect.DeepEqual(repo.writes, []string{"before"}) {
		t.Fatalf("baseline writes = %#v", repo.writes)
	}
}

func TestHostHoldsStateAcrossAgentAndSnapshotFailures(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{
		root: "/work/reviewed", worktrees: map[string]string{"/work/reviewed": "/work/reviewed"},
		snapshots: []string{"before", "after"},
	}
	agents := &fakeAgents{
		samples: [][]herdr.AgentSample{
			{{Status: "idle", CWD: "/work/reviewed"}},
			{{Status: "working", CWD: "/work/reviewed"}},
			nil,
			{{Status: "working", CWD: "/work/reviewed"}},
		},
		errors: []error{nil, nil, errors.New("herdr unavailable")},
	}
	host := NewHost(repo, agents)

	host.Sample()
	host.Sample()
	if report := host.Sample(); report.BaselineChanged {
		t.Fatal("failed enumeration changed the baseline")
	}
	if report := host.Sample(); !report.BaselineChanged {
		t.Fatal("candidate was lost across failed enumeration")
	}
}

func TestUnknownAndRelativeSamplesCannotFabricateATurnEdge(t *testing.T) {
	t.Parallel()
	repo := &fakeRepository{
		root: "/work/reviewed", worktrees: map[string]string{"/work/reviewed": "/work/reviewed"},
		snapshots: []string{"before"},
	}
	agents := &fakeAgents{samples: [][]herdr.AgentSample{
		nil,
		{{Status: "compacting", CWD: "/work/reviewed"}},
		{{Status: "working", CWD: "/work/reviewed"}, {Status: "idle", CWD: "relative"}},
	}}
	host := NewHost(repo, agents)

	host.Sample()
	host.Sample()
	host.Sample()
	if len(repo.snapshots) != 1 || len(repo.writes) != 0 {
		t.Fatalf("unknown transition captured a candidate: snapshots %#v writes %#v", repo.snapshots, repo.writes)
	}
}
