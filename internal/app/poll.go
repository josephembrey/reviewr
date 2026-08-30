package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/repository"
)

const repositoryPollInterval = time.Second

type repositoryPoller interface {
	PollState() (repository.StateFingerprint, error)
}

type repositoryPollTickMsg struct{}

type repositoryPolledMsg struct {
	generation uint64
	activity   uint64
	state      repository.StateFingerprint
	err        error
}

type repositoryPollState struct {
	generation  uint64
	running     bool
	ready       bool
	deferNext   bool
	activity    uint64
	fingerprint repository.StateFingerprint
}

func scheduleRepositoryPoll() tea.Cmd {
	return tea.Tick(repositoryPollInterval, func(time.Time) tea.Msg {
		return repositoryPollTickMsg{}
	})
}

func (m *Model) beginRepositoryPoll() tea.Cmd {
	provider, ok := m.source.(repositoryPoller)
	if !ok || m.poll.running {
		return nil
	}
	if m.poll.deferNext {
		m.poll.deferNext = false
		return scheduleRepositoryPoll()
	}
	m.poll.generation++
	m.poll.running = true
	generation := m.poll.generation
	activity := m.poll.activity
	return func() tea.Msg {
		state, err := provider.PollState()
		return repositoryPolledMsg{generation: generation, activity: activity, state: state, err: err}
	}
}

func (m *Model) landRepositoryPoll(msg repositoryPolledMsg) tea.Cmd {
	if msg.generation != m.poll.generation {
		return nil
	}
	m.poll.running = false
	commands := []tea.Cmd{scheduleRepositoryPoll()}
	if msg.activity != m.poll.activity {
		return batchCommands(commands...)
	}
	if msg.err != nil {
		return batchCommands(commands...)
	}
	worktreeChanged := !m.poll.ready || msg.state.Worktree != m.poll.fingerprint.Worktree
	refsChanged := !m.poll.ready || msg.state.Refs != m.poll.fingerprint.Refs
	m.poll.ready = true
	m.poll.fingerprint = msg.state
	if worktreeChanged {
		commands = append(commands,
			m.command(tagRepositoryPoll(m.files.poll(m.controls.Comparison.Label()), msg.activity)),
			m.command(tagRepositoryPoll(m.summary.poll(), msg.activity)),
		)
	}
	if refsChanged {
		commands = append(commands, m.command(tagRepositoryPoll(m.history.poll(m.controls.Traversal, m.selectedHistoryOID()), msg.activity)))
		if m.refs.loaded {
			commands = append(commands, m.command(tagRepositoryPoll(m.refs.poll(), msg.activity)))
		}
		if m.stashes.loaded {
			commands = append(commands, m.command(tagRepositoryPoll(m.stashes.poll(), msg.activity)))
		}
	}
	return batchCommands(commands...)
}

func (m *Model) noteRepositoryActivity() {
	m.poll.activity++
	m.poll.deferNext = true
}

func (m Model) acceptsRepositoryPoll(background bool, activity uint64) bool {
	return !background || activity == m.poll.activity
}

func tagRepositoryPoll(pending effect, activity uint64) effect {
	if pending.kind != effectNone {
		pending.background = true
		pending.activity = activity
	}
	return pending
}
