package app

import "time"

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
	return readerContextAnimationEffect(readerContextFiles, m.files.readerContextGeneration, animating)
}

func (m *Model) setStashReaderContext(expanded bool) effect {
	changed, animating := m.stashes.setReaderContextExpanded(expanded)
	if !changed {
		return effect{}
	}
	m.clampDocumentReader(&m.stashes.place, m.stashes.readerDocument())
	return readerContextAnimationEffect(readerContextStashes, m.stashes.readerContextGeneration, animating)
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
