package app

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/josephembrey/reviewr/internal/notes"
	"github.com/josephembrey/reviewr/internal/repository"
	"github.com/josephembrey/reviewr/internal/review"
	"github.com/josephembrey/reviewr/internal/session"
	"github.com/josephembrey/reviewr/internal/ui"
)

type effectKind uint8

const (
	effectNone effectKind = iota
	effectLoadSnapshot
	effectLoadFile
	effectLoadDiff
	effectLoadCommits
	effectLoadCommit
	effectLoadReviewState
	effectLoadReviewDocument
	effectLoadReviewFile
	effectVerifyReview
	effectPersistReview
	effectLoadRefSources
	effectLoadRefCommits
	effectLoadStashes
	effectLoadStashFiles
	effectLoadStashFile
	effectLoadNotes
	effectDebounceNotes
	effectSaveNotes
	effectSaveSession
	effectAnimateReaderContext
	effectOpenEditor
	effectQuit
)

// effect is the shared transition value returned by the existing domain
// states. Command construction below immediately narrows it by owning domain.
type effect struct {
	kind               effectKind
	generation         uint64
	identity           string
	entry              repository.Entry
	fileComparison     repository.Comparison
	query              repository.CommitQuery
	refSource          repository.RefSource
	stashSource        repository.ChangeSource
	changedFile        repository.ChangedFile
	text               string
	session            session.State
	background         bool
	activity           uint64
	notesScope         notes.Scope
	readerContextOwner readerContextOwner
	path               string
	line               uint64

	scope            string
	reviewGeneration uint64
	comparison       review.FileComparison
	bounds           review.Bounds
	retained         *string
	delta            review.Delta
	store            *review.Store
}

// repositoryPollResult tags asynchronous repository work without making
// every result type reimplement the same background-completion contract.
type repositoryPollResult struct {
	background bool
	activity   uint64
}

func (result repositoryPollResult) repositoryPollContext() (bool, uint64) {
	return result.background, result.activity
}

type snapshotLoadedMsg struct {
	repositoryPollResult
	generation       uint64
	snapshot         repository.Snapshot
	err              error
	reviewGeneration uint64
	reviewSnapshot   review.Snapshot
	reviewErr        error
	reviewCapable    bool
}

type reviewStateLoadedMsg struct {
	ledger  review.Ledger
	store   *review.Store
	warning string
	err     error
}

type reviewDocumentLoadedMsg struct {
	repositoryPollResult
	generation   uint64
	entry        repository.Entry
	comparison   review.FileComparison
	bounds       review.Bounds
	document     review.Document
	presentation ui.ReaderDocument
}

type reviewFileLoadedMsg struct {
	repositoryPollResult
	generation   uint64
	entry        repository.Entry
	comparison   review.FileComparison
	content      review.Content
	document     review.Document
	presentation ui.ReaderDocument
}

type reviewVerifiedMsg struct {
	generation uint64
	entry      repository.Entry
	comparison review.FileComparison
	delta      review.Delta
	content    review.Content
}

type reviewPersistedMsg struct {
	delta  review.Delta
	ledger review.Ledger
	err    error
}

type fileLoadedMsg struct {
	repositoryPollResult
	generation   uint64
	entry        repository.Entry
	file         repository.File
	presentation ui.ReaderDocument
}

type diffLoadedMsg struct {
	repositoryPollResult
	generation   uint64
	entry        repository.Entry
	diff         repository.Diff
	presentation ui.ReaderDocument
}

type commitsLoadedMsg struct {
	repositoryPollResult
	generation uint64
	commits    []repository.Commit
	err        error
	query      repository.CommitQuery
}

type commitLoadedMsg struct {
	repositoryPollResult
	generation uint64
	oid        string
	summary    repository.CommitSummary
	err        error
}

type notesLoadedMsg struct {
	scope      notes.Scope
	generation uint64
	text       string
	readOnly   bool
	err        error
}

type notesSaveDueMsg struct {
	scope      notes.Scope
	generation uint64
}

type notesSavedMsg struct {
	scope      notes.Scope
	generation uint64
	err        error
}

type sessionSaveDueMsg struct {
	generation uint64
}

type sessionSavedMsg struct {
	generation uint64
	err        error
}

type refSourcesLoadedMsg struct {
	repositoryPollResult
	generation uint64
	sources    []repository.RefSource
	err        error
}

type refCommitsLoadedMsg struct {
	repositoryPollResult
	generation uint64
	sourceID   repository.RefSourceID
	commits    []repository.RefCommit
	err        error
}

type stashesLoadedMsg struct {
	repositoryPollResult
	generation uint64
	stashes    []repository.Stash
	err        error
}

type stashFilesLoadedMsg struct {
	repositoryPollResult
	generation uint64
	oid        string
	files      []repository.ChangedFile
	err        error
}

type stashFileLoadedMsg struct {
	repositoryPollResult
	generation   uint64
	oid          string
	fileIdentity string
	document     repository.ChangeDocument
	presentation ui.ReaderDocument
}

type backgroundRepositoryResult interface {
	repositoryPollContext() (bool, uint64)
}

// command is the sole asynchronous effect router. Each domain command helper
// captures immutable request data before Bubble Tea executes its closure.
func (m Model) command(pending effect) tea.Cmd {
	switch pending.kind {
	case effectLoadSnapshot, effectLoadFile, effectLoadDiff:
		return m.repositoryCommand(pending)
	case effectLoadReviewState,
		effectLoadReviewDocument,
		effectLoadReviewFile,
		effectVerifyReview,
		effectPersistReview:
		return m.reviewCommand(pending)
	case effectLoadCommits,
		effectLoadCommit,
		effectLoadRefSources,
		effectLoadRefCommits,
		effectLoadStashes,
		effectLoadStashFiles,
		effectLoadStashFile:
		return m.gitCommand(pending)
	case effectLoadNotes, effectDebounceNotes, effectSaveNotes:
		return m.notesCommand(pending)
	case effectSaveSession, effectAnimateReaderContext, effectOpenEditor, effectQuit:
		return m.rootCommand(pending)
	default:
		return nil
	}
}

func (m Model) repositoryCommand(pending effect) tea.Cmd {
	switch pending.kind {
	case effectLoadSnapshot:
		source := m.source
		generation := pending.generation
		reviewGeneration := pending.reviewGeneration
		scope := pending.scope
		background := pending.background
		activity := pending.activity
		return func() tea.Msg {
			snapshot, err := source.Snapshot(scope)
			message := snapshotLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, snapshot: snapshot, err: err, reviewGeneration: reviewGeneration,
			}
			provider, ok := source.(review.Provider)
			comparison := snapshot.Comparison()
			if err == nil && ok && comparison.Available() {
				message.reviewCapable = true
				message.reviewSnapshot, message.reviewErr = provider.ReviewComparisons(scope, comparison.Basis, reviewCandidates(snapshot.Changed()))
			} else if err == nil && ok && comparison.Reason != "" {
				message.reviewCapable = true
				message.reviewSnapshot = review.Snapshot{Scope: scope, Comparisons: make(map[string]review.FileComparison)}
				message.reviewErr = errors.New(comparison.Reason)
			}
			return message
		}
	case effectLoadFile:
		source := m.source
		generation := pending.generation
		entry := pending.entry
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			file := source.ReadFile(entry)
			return fileLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, entry: entry, file: file, presentation: fileReaderDocument(file, entry),
			}
		}
	case effectLoadDiff:
		source := m.source
		generation := pending.generation
		entry := pending.entry
		comparison := pending.fileComparison
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			diff := source.ReadDiff(comparison, entry)
			return diffLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, entry: entry, diff: diff, presentation: diffReaderDocument(diff),
			}
		}
	default:
		return nil
	}
}

func (m Model) reviewCommand(pending effect) tea.Cmd {
	if pending.kind == effectPersistReview {
		store, delta := pending.store, pending.delta
		return func() tea.Msg {
			if store == nil {
				return reviewPersistedMsg{delta: delta, err: errors.New("review state store unavailable")}
			}
			ledger, err := store.Replay(delta)
			return reviewPersistedMsg{delta: delta, ledger: ledger, err: err}
		}
	}
	provider, capable := m.source.(review.Provider)
	if !capable {
		return nil
	}
	switch pending.kind {
	case effectLoadReviewState:
		root := m.reviewStateRoot
		return func() tea.Msg {
			identity, err := provider.ReviewRepositoryID()
			if err != nil {
				return reviewStateLoadedMsg{err: err, warning: "review state unavailable; marks will not survive restart"}
			}
			ledger, store, warning := review.OpenStore(identity, root)
			return reviewStateLoadedMsg{ledger: ledger, store: store, warning: warning}
		}
	case effectLoadReviewDocument:
		generation, entry := pending.generation, pending.entry
		comparison, bounds, retained := pending.comparison, pending.bounds, pending.retained
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			var oldContent review.Content
			if retained != nil && bounds.Old != comparison.Old {
				oldContent = review.Content{Endpoint: bounds.Old, State: review.ContentText, Text: *retained, Size: int64(len(*retained))}
			} else {
				oldContent = provider.ReadReviewContent(comparison.OldSource, bounds.Old)
			}
			newContent := provider.ReadReviewContent(comparison.NewSource, bounds.New)
			document := review.BuildDocument(bounds, oldContent, newContent)
			return reviewDocumentLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, entry: entry, comparison: comparison, bounds: bounds,
				document: document, presentation: reviewReaderDocument(entry.Path, document),
			}
		}
	case effectLoadReviewFile:
		generation, entry, comparison := pending.generation, pending.entry, pending.comparison
		background, activity := pending.background, pending.activity
		return func() tea.Msg {
			bounds := review.Bounds{Old: comparison.Old, New: comparison.New}
			oldContent := provider.ReadReviewContent(comparison.OldSource, comparison.Old)
			content := provider.ReadReviewContent(comparison.NewSource, comparison.New)
			document := review.BuildDocument(bounds, oldContent, content)
			return reviewFileLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, entry: entry, comparison: comparison, content: content, document: document,
				presentation: annotatedReviewFileReaderDocument(content, entry, comparison, document),
			}
		}
	case effectVerifyReview:
		generation, entry := pending.generation, pending.entry
		comparison, delta := pending.comparison, pending.delta
		return func() tea.Msg {
			content := provider.ReadReviewContent(comparison.NewSource, comparison.New)
			return reviewVerifiedMsg{generation: generation, entry: entry, comparison: comparison, delta: delta, content: content}
		}
	default:
		return nil
	}
}

func (m Model) gitCommand(pending effect) tea.Cmd {
	source := m.source
	background, activity := pending.background, pending.activity
	switch pending.kind {
	case effectLoadCommits:
		generation, query := pending.generation, pending.query
		return func() tea.Msg {
			commits, err := source.ListCommits(query)
			return commitsLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, commits: commits, err: err, query: query,
			}
		}
	case effectLoadCommit:
		generation, oid := pending.generation, pending.identity
		return func() tea.Msg {
			summary, err := source.ReadCommit(oid)
			return commitLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, oid: oid, summary: summary, err: err,
			}
		}
	case effectLoadRefSources:
		generation := pending.generation
		return func() tea.Msg {
			sources, err := source.ListRefSources()
			return refSourcesLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, sources: sources, err: err,
			}
		}
	case effectLoadRefCommits:
		generation, refSource := pending.generation, pending.refSource
		return func() tea.Msg {
			commits, err := source.ListRefCommits(refSource)
			return refCommitsLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, sourceID: refSource.ID, commits: commits, err: err,
			}
		}
	case effectLoadStashes:
		generation := pending.generation
		return func() tea.Msg {
			stashes, err := source.ListStashes()
			return stashesLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, stashes: stashes, err: err,
			}
		}
	case effectLoadStashFiles:
		generation, oid, stashSource := pending.generation, pending.identity, pending.stashSource
		return func() tea.Msg {
			files, err := source.ListStashFiles(stashSource)
			return stashFilesLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, oid: oid, files: files, err: err,
			}
		}
	case effectLoadStashFile:
		generation, oid := pending.generation, pending.identity
		stashSource, file := pending.stashSource, pending.changedFile
		return func() tea.Msg {
			document := source.ReadStashFile(stashSource, file)
			return stashFileLoadedMsg{
				repositoryPollResult: repositoryPollResult{background: background, activity: activity},
				generation:           generation, oid: oid, fileIdentity: file.Identity(),
				document: document, presentation: changeDiffDocument(document),
			}
		}
	default:
		return nil
	}
}

func (m Model) notesCommand(pending effect) tea.Cmd {
	scope := pending.notesScope
	generation := pending.generation
	switch pending.kind {
	case effectLoadNotes:
		store := m.note.forScope(scope).store
		return func() tea.Msg {
			text, readOnly, err := store.Load()
			return notesLoadedMsg{scope: scope, generation: generation, text: text, readOnly: readOnly, err: err}
		}
	case effectDebounceNotes:
		return tea.Tick(notesSaveDebounce, func(time.Time) tea.Msg {
			return notesSaveDueMsg{scope: scope, generation: generation}
		})
	case effectSaveNotes:
		store := m.note.forScope(scope).store
		text := pending.text
		return func() tea.Msg {
			return notesSavedMsg{scope: scope, generation: generation, err: store.Save(text)}
		}
	default:
		return nil
	}
}

func (m Model) rootCommand(pending effect) tea.Cmd {
	switch pending.kind {
	case effectSaveSession:
		store := m.sessionStore
		generation, state := pending.generation, pending.session
		if store == nil {
			return nil
		}
		return func() tea.Msg {
			return sessionSavedMsg{generation: generation, err: store.Save(generation, state)}
		}
	case effectAnimateReaderContext:
		owner, generation := pending.readerContextOwner, pending.generation
		return tea.Tick(readerContextFrameDelay, func(time.Time) tea.Msg {
			return readerContextFrameMsg{owner: owner, generation: generation}
		})
	case effectOpenEditor:
		return m.editorCommand(pending)
	case effectQuit:
		return tea.Quit
	default:
		return nil
	}
}

// Shutdown synchronously saves final place, flushes edited notes, and releases
// their OS-backed locks. The final generation supersedes delayed save commands.
func (m *Model) Shutdown() error {
	var sessionErr error
	if m.sessionStore != nil {
		m.sessionSave++
		sessionErr = m.sessionStore.Save(m.sessionSave, m.sessionState())
	}
	return errors.Join(sessionErr, m.note.shutdown())
}

func batchCommands(commands ...tea.Cmd) tea.Cmd {
	filtered := commands[:0]
	for _, command := range commands {
		if command != nil {
			filtered = append(filtered, command)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}
