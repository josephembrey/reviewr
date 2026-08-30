package app

import (
	"time"

	"github.com/josephembrey/reviewr/internal/workspace"
)

const (
	readerContextAnimationSteps = 8
	readerContextFrameDelay     = 20 * time.Millisecond
)

type readerContextOwner uint8

const (
	readerContextFiles readerContextOwner = iota + 1
	readerContextStashes
)

type readerContextFrameMsg struct {
	owner      readerContextOwner
	generation uint64
}

func readerContextTarget(expanded bool) int {
	if expanded {
		return readerContextAnimationSteps
	}
	return 0
}

func stepReaderContext(progress int, expanded bool) int {
	target := readerContextTarget(expanded)
	if progress < target {
		return progress + 1
	}
	if progress > target {
		return progress - 1
	}
	return progress
}

func readerContextAnimating(progress int, expanded bool) bool {
	return progress != readerContextTarget(expanded)
}

func readerContextAnimationEffect(owner readerContextOwner, generation uint64, animating bool) effect {
	if !animating {
		return effect{}
	}
	return effect{kind: effectAnimateReaderContext, generation: generation, readerContextOwner: owner}
}

func (m *Model) setFilesReaderContext(expanded bool) effect {
	changed, animating := m.files.setReaderContextExpanded(expanded)
	if !changed {
		return effect{}
	}
	m.clampDocumentReader(&m.files.place, m.files.readerDocument())
	return readerContextAnimationEffect(readerContextFiles, m.files.readerContext.generation, animating)
}

func (m *Model) toggleActiveReaderContextFold(identity string) effect {
	return m.changeActiveReaderContextFold(identity, nil)
}

func (m *Model) setActiveReaderContextFold(expanded bool) effect {
	document, ok := m.activeReaderDocument()
	if !ok {
		return effect{}
	}
	cursor := m.activePlace().ReaderCursor
	if cursor < 0 || cursor >= len(document.Rows) {
		return effect{}
	}
	identity, ok := document.Rows[cursor].ContextFoldIdentity()
	if !ok {
		return effect{}
	}
	return m.changeActiveReaderContextFold(identity, &expanded)
}

func (m *Model) changeActiveReaderContextFold(identity string, expanded *bool) effect {
	var changed, animating bool
	var owner readerContextOwner
	switch {
	case m.gitStashesActive():
		if expanded == nil {
			changed, animating = m.stashes.toggleReaderContextFold(identity)
		} else {
			changed, animating = m.stashes.setReaderContextFold(identity, *expanded)
		}
		owner = readerContextStashes
	case m.active == workspace.Files:
		if expanded == nil {
			changed, animating = m.files.toggleReaderContextFold(identity)
		} else {
			changed, animating = m.files.setReaderContextFold(identity, *expanded)
		}
		owner = readerContextFiles
	}
	if !changed {
		return effect{}
	}
	document, _ := m.activeReaderDocument()
	m.clampDocumentReader(m.activePlace(), document)
	generation := m.files.readerContext.generation
	if owner == readerContextStashes {
		generation = m.stashes.readerContext.generation
	}
	return readerContextAnimationEffect(owner, generation, animating)
}

func (m *Model) landReaderContextFrame(msg readerContextFrameMsg) effect {
	var advanced, animating bool
	switch msg.owner {
	case readerContextFiles:
		advanced, animating = m.files.advanceReaderContext(msg.generation)
		if advanced {
			m.clampDocumentReader(&m.files.place, m.files.readerDocument())
		}
	case readerContextStashes:
		advanced, animating = m.stashes.advanceReaderContext(msg.generation)
		if advanced {
			m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
		}
	}
	if !advanced {
		return effect{}
	}
	return readerContextAnimationEffect(msg.owner, msg.generation, animating)
}
