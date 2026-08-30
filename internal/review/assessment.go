package review

// Assess derives coverage without mutating the ledger.
func (l Ledger) Assess(comparison FileComparison) Assessment {
	related := l.relatedReceipts(comparison)
	if assessment, done := prerequisiteAssessment(comparison, related); done {
		return assessment
	}

	reachable := l.reachableFrom(comparison.Old, comparison)
	if _, reviewed := reachable[comparison.New]; reviewed {
		return Assessment{State: Reviewed}
	}
	if frontier := l.newestFrontier(reachable, comparison); frontier != nil {
		return frontierAssessment(*frontier)
	}
	return uncoveredAssessment(comparison, related)
}

func (l Ledger) relatedReceipts(comparison FileComparison) []Receipt {
	related := make([]Receipt, 0)
	for _, receipt := range l.ReceiptData {
		if receiptRelated(receipt, comparison) {
			related = append(related, receipt)
		}
	}
	return related
}

func prerequisiteAssessment(comparison FileComparison, related []Receipt) (Assessment, bool) {
	if !comparison.Exact() {
		if len(related) == 0 {
			return Assessment{State: Unreviewed}, true
		}
		return Assessment{State: BasisChanged, Reason: "exact file identity unavailable"}, true
	}
	for _, receipt := range related {
		if receipt.Action == comparison.Action && receipt.Old == comparison.Old && receipt.New == comparison.New {
			return Assessment{State: Reviewed}, true
		}
	}
	if comparison.BasisReason != "" && len(related) > 0 {
		return Assessment{State: BasisChanged, Reason: comparison.BasisReason}, true
	}
	return Assessment{}, false
}

func (l Ledger) newestFrontier(reachable map[Endpoint]struct{}, comparison FileComparison) *Receipt {
	var frontier *Receipt
	for index := range l.ReceiptData {
		receipt := &l.ReceiptData[index]
		if _, reached := reachable[receipt.New]; !reached || !canContinue(receipt.New, comparison) {
			continue
		}
		if frontier == nil || receipt.Sequence > frontier.Sequence {
			frontier = receipt
		}
	}
	return frontier
}

func frontierAssessment(frontier Receipt) Assessment {
	endpoint := frontier.New
	if frontier.Retained != nil {
		return Assessment{State: Updated, Frontier: &endpoint, Retained: cloneString(frontier.Retained)}
	}
	if frontier.New.Kind == Absent {
		empty := ""
		return Assessment{State: Updated, Frontier: &endpoint, Retained: &empty}
	}
	return Assessment{State: BasisChanged, Reason: "reviewed snapshot unavailable for an incremental diff"}
}

func uncoveredAssessment(comparison FileComparison, related []Receipt) Assessment {
	if len(related) == 0 {
		return Assessment{State: Unreviewed}
	}
	for _, receipt := range related {
		if receipt.Comparison.Scope == comparison.Identity.Scope {
			return Assessment{State: BasisChanged, Reason: "comparison basis changed"}
		}
	}
	return Assessment{State: Partial, Reason: "an older comparison gap remains"}
}

// AssessAll derives a comparison snapshot with one path index. Exact endpoint
// composition is unchanged; the index only excludes receipt components that
// cannot connect to a comparison, keeping refresh cost bounded for many files.
func (l Ledger) AssessAll(comparisons map[string]FileComparison) map[string]Assessment {
	byPath := l.receiptIndicesByPath()
	result := make(map[string]Assessment, len(comparisons))
	for path, comparison := range comparisons {
		subset := Ledger{
			ReceiptData:  l.connectedReceipts(byPath, comparison),
			NextSequence: l.NextSequence,
		}
		result[path] = subset.Assess(comparison)
	}
	return result
}

func (l Ledger) receiptIndicesByPath() map[string][]int {
	byPath := make(map[string][]int)
	for index, receipt := range l.ReceiptData {
		byPath[receipt.Old.Path] = append(byPath[receipt.Old.Path], index)
		if receipt.New.Path != receipt.Old.Path {
			byPath[receipt.New.Path] = append(byPath[receipt.New.Path], index)
		}
	}
	return byPath
}

func (l Ledger) connectedReceipts(byPath map[string][]int, comparison FileComparison) []Receipt {
	paths := []string{comparison.Old.Path}
	if comparison.New.Path != comparison.Old.Path {
		paths = append(paths, comparison.New.Path)
	}
	seenPaths := make(map[string]struct{}, len(paths))
	seenReceipts := make(map[int]struct{})
	receipts := make([]Receipt, 0)
	for nextPath := 0; nextPath < len(paths); nextPath++ {
		path := paths[nextPath]
		if _, seen := seenPaths[path]; seen {
			continue
		}
		seenPaths[path] = struct{}{}
		for _, index := range byPath[path] {
			if _, seen := seenReceipts[index]; seen {
				continue
			}
			seenReceipts[index] = struct{}{}
			receipt := l.ReceiptData[index]
			receipts = append(receipts, receipt)
			paths = append(paths, receipt.Old.Path, receipt.New.Path)
		}
	}
	return receipts
}

func (l Ledger) reachableFrom(start Endpoint, comparison FileComparison) map[Endpoint]struct{} {
	reached := map[Endpoint]struct{}{start: {}}
	adjacent := make(map[Endpoint][]Endpoint)
	for _, receipt := range l.ReceiptData {
		if receipt.Old.Exact() && receipt.New.Exact() && actionComposes(receipt, comparison) {
			adjacent[receipt.Old] = append(adjacent[receipt.Old], receipt.New)
		}
	}
	queue := []Endpoint{start}
	for next := 0; next < len(queue); next++ {
		for _, endpoint := range adjacent[queue[next]] {
			if _, seen := reached[endpoint]; seen {
				continue
			}
			reached[endpoint] = struct{}{}
			queue = append(queue, endpoint)
		}
	}
	return reached
}

func actionComposes(receipt Receipt, comparison FileComparison) bool {
	if receipt.Old.Path == receipt.New.Path {
		return true
	}
	switch comparison.Action {
	case Copied:
		return receipt.Action == Copied
	case Renamed:
		return receipt.Action == Renamed
	default:
		return false
	}
}

func receiptRelated(receipt Receipt, comparison FileComparison) bool {
	return receipt.Old.Path == comparison.Old.Path || receipt.Old.Path == comparison.New.Path ||
		receipt.New.Path == comparison.Old.Path || receipt.New.Path == comparison.New.Path
}

func canContinue(frontier Endpoint, comparison FileComparison) bool {
	return frontier != comparison.New && (frontier.Path == comparison.New.Path ||
		frontier.Path == comparison.Old.Path || comparison.Action == Renamed || comparison.Action == Copied)
}
