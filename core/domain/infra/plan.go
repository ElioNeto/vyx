package infra

import "time"

// PlanResult is the complete output of a plan operation.
type PlanResult struct {
	StackName string            `json:"stack_name"`
	Serial    uint64            `json:"serial"`
	Changes   []*ResourceChange `json:"changes"`
	Summary   PlanSummary       `json:"summary"`
	CreatedAt time.Time         `json:"created_at"`
}

// PlanSummary holds aggregate counts of planned changes.
type PlanSummary struct {
	ToCreate int `json:"to_create"`
	ToUpdate int `json:"to_update"`
	ToDelete int `json:"to_delete"`
	Noop     int `json:"noop"`
	Errors   int `json:"errors"`
}

// NewPlanResult creates a new PlanResult with the given parameters.
func NewPlanResult(stackName string, serial uint64, changes []*ResourceChange) *PlanResult {
	summary := PlanSummary{}
	for _, c := range changes {
		switch c.ChangeType {
		case ChangeCreate:
			summary.ToCreate++
		case ChangeUpdate:
			summary.ToUpdate++
		case ChangeDelete:
			summary.ToDelete++
		case ChangeNoop:
			summary.Noop++
		}
	}

	return &PlanResult{
		StackName: stackName,
		Serial:    serial,
		Changes:   changes,
		Summary:   summary,
		CreatedAt: time.Now(),
	}
}

// HasChanges returns true if the plan contains any non-noop changes.
func (p *PlanResult) HasChanges() bool {
	return p.Summary.ToCreate > 0 || p.Summary.ToUpdate > 0 || p.Summary.ToDelete > 0
}

// String implements fmt.Stringer for human-readable output.
func (p *PlanResult) String() string {
	s := p.Summary
	return formatSummary(s)
}

// formatSummary produces a human-readable summary string.
func formatSummary(s PlanSummary) string {
	result := "Plan: "
	parts := make([]string, 0, 4)
	if s.ToCreate > 0 {
		parts = append(parts, intStr(s.ToCreate, "to create"))
	}
	if s.ToUpdate > 0 {
		parts = append(parts, intStr(s.ToUpdate, "to update"))
	}
	if s.ToDelete > 0 {
		parts = append(parts, intStr(s.ToDelete, "to delete"))
	}
	if s.Noop > 0 {
		parts = append(parts, intStr(s.Noop, "noop"))
	}

	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}

	if len(parts) == 0 {
		result += "no changes"
	}

	if s.Errors > 0 {
		result += "; " + intStr(s.Errors, "error")
	}

	return result
}

func intStr(n int, label string) string {
	if n == 1 {
		return "1 " + label
	}
	return itoa(n) + " " + label
}

// itoa is a small integer-to-string helper (avoids strconv import in domain).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
