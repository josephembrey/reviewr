package review

import "sort"

// Receipts returns a defensive copy of the ledger receipts.
func (l Ledger) Receipts() []Receipt {
	return cloneReceipts(l.ReceiptData)
}

// Clone returns an independent ledger copy.
func (l Ledger) Clone() Ledger {
	l.ReceiptData = cloneReceipts(l.ReceiptData)
	return l
}

// Mark adds exactly the represented bounds. This is the only coverage-creating operation.
func (l *Ledger) Mark(comparison FileComparison, bounds Bounds, retained *string) bool {
	if !bounds.Old.Exact() || !bounds.New.Exact() {
		return false
	}
	receipt := l.newReceipt(comparison, bounds, retained)
	if index := l.receiptIndex(receipt); index >= 0 {
		l.ReceiptData[index] = receipt
	} else {
		l.ReceiptData = append(l.ReceiptData, receipt)
	}
	l.Compact()
	return true
}

func (l *Ledger) newReceipt(comparison FileComparison, bounds Bounds, retained *string) Receipt {
	l.NextSequence++
	if l.NextSequence == 0 {
		l.NextSequence = 1
	}
	return Receipt{
		Comparison: comparison.Identity,
		OldSource:  l.oldSource(comparison, bounds.Old),
		NewSource:  comparison.NewSource,
		Action:     actionForBounds(comparison, bounds),
		Old:        bounds.Old,
		New:        bounds.New,
		Retained:   boundedRetained(retained),
		Sequence:   l.NextSequence,
	}
}

func (l Ledger) oldSource(comparison FileComparison, old Endpoint) EndpointSource {
	if old == comparison.Old {
		return comparison.OldSource
	}
	source := comparison.OldSource
	var sequence uint64
	for _, receipt := range l.ReceiptData {
		if receipt.New == old && receipt.Sequence >= sequence {
			source = receipt.NewSource
			sequence = receipt.Sequence
		}
	}
	return source
}

func (l Ledger) receiptIndex(want Receipt) int {
	for index, receipt := range l.ReceiptData {
		if receipt.Action == want.Action && receipt.Old == want.Old && receipt.New == want.New {
			return index
		}
	}
	return -1
}

// Clear removes the newest checkpoint proving the exact active current result.
func (l *Ledger) Clear(comparison FileComparison) bool {
	remove := l.newestDirectReceipt(comparison)
	if remove < 0 {
		remove = l.newestComposingReceipt(comparison)
	}
	if remove < 0 {
		return false
	}
	l.ReceiptData = append(l.ReceiptData[:remove], l.ReceiptData[remove+1:]...)
	return true
}

func (l Ledger) newestDirectReceipt(comparison FileComparison) int {
	remove := -1
	var sequence uint64
	for index, receipt := range l.ReceiptData {
		if receipt.Action == comparison.Action && receipt.Old == comparison.Old && receipt.New == comparison.New && receipt.Sequence >= sequence {
			remove, sequence = index, receipt.Sequence
		}
	}
	return remove
}

func (l Ledger) newestComposingReceipt(comparison FileComparison) int {
	reachable := l.reachableFrom(comparison.Old, comparison)
	remove := -1
	var sequence uint64
	for index, receipt := range l.ReceiptData {
		_, oldReached := reachable[receipt.Old]
		if receipt.New == comparison.New && oldReached && actionComposes(receipt, comparison) && receipt.Sequence >= sequence {
			remove, sequence = index, receipt.Sequence
		}
	}
	return remove
}

// Compact deduplicates exact edges and bounds history and retained payloads.
func (l *Ledger) Compact() {
	l.deduplicate()
	l.trimHistory()
	l.trimRetained()
	l.reconcileSequence()
}

func (l *Ledger) deduplicate() {
	sort.SliceStable(l.ReceiptData, func(i, j int) bool {
		return l.ReceiptData[i].Sequence > l.ReceiptData[j].Sequence
	})
	seen := make(map[edgeKey]struct{}, len(l.ReceiptData))
	unique := l.ReceiptData[:0]
	for _, receipt := range l.ReceiptData {
		key := edgeKey{Action: receipt.Action, Old: receipt.Old, New: receipt.New}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, receipt)
	}
	l.ReceiptData = unique
	sort.SliceStable(l.ReceiptData, func(i, j int) bool {
		return l.ReceiptData[i].Sequence < l.ReceiptData[j].Sequence
	})
}

func (l *Ledger) trimHistory() {
	if len(l.ReceiptData) <= MaxReceipts {
		return
	}
	drop := len(l.ReceiptData) - MaxReceipts
	l.ReceiptData = append([]Receipt(nil), l.ReceiptData[drop:]...)
}

func (l *Ledger) trimRetained() {
	total := 0
	for _, receipt := range l.ReceiptData {
		if receipt.Retained != nil {
			total += len(*receipt.Retained)
		}
	}
	for index := range l.ReceiptData {
		if total <= MaxRetainedTotal {
			return
		}
		if retained := l.ReceiptData[index].Retained; retained != nil {
			total -= len(*retained)
			l.ReceiptData[index].Retained = nil
		}
	}
}

func (l *Ledger) reconcileSequence() {
	for _, receipt := range l.ReceiptData {
		if receipt.Sequence > l.NextSequence {
			l.NextSequence = receipt.Sequence
		}
	}
}

type edgeKey struct {
	Action FileAction
	Old    Endpoint
	New    Endpoint
}

func actionForBounds(comparison FileComparison, bounds Bounds) FileAction {
	if bounds.Old == comparison.Old && bounds.New == comparison.New {
		return comparison.Action
	}
	switch {
	case bounds.Old.Kind == Absent:
		return Added
	case bounds.New.Kind == Absent:
		return Deleted
	case bounds.Old.Path != bounds.New.Path && comparison.Action == Copied:
		return Copied
	case bounds.Old.Path != bounds.New.Path:
		return Renamed
	default:
		return Modified
	}
}

func boundedRetained(retained *string) *string {
	if retained == nil || len(*retained) > MaxRetainedBytes {
		return nil
	}
	return cloneString(retained)
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneReceipts(receipts []Receipt) []Receipt {
	result := append([]Receipt(nil), receipts...)
	for index := range result {
		result[index].Retained = cloneString(result[index].Retained)
	}
	return result
}

// DeltaKind identifies one user-authored persistent ledger mutation.
type DeltaKind string

const (
	MarkDelta  DeltaKind = "mark"
	ClearDelta DeltaKind = "clear"
)

// Delta is replayed against locked current state instead of replacing a stale ledger.
type Delta struct {
	Kind       DeltaKind
	Comparison FileComparison
	Bounds     Bounds
	Retained   *string
}

// Apply mutates a ledger exactly as the authored action requested.
func (d Delta) Apply(ledger *Ledger) bool {
	if d.Kind == ClearDelta {
		return ledger.Clear(d.Comparison)
	}
	return ledger.Mark(d.Comparison, d.Bounds, d.Retained)
}
