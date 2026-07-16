package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlanResultSummary(t *testing.T) {
	changes := []*ResourceChange{
		{ResourceID: "a", ChangeType: ChangeCreate},
		{ResourceID: "b", ChangeType: ChangeCreate},
		{ResourceID: "c", ChangeType: ChangeUpdate},
		{ResourceID: "d", ChangeType: ChangeDelete},
		{ResourceID: "e", ChangeType: ChangeNoop},
	}

	pr := NewPlanResult("test", 1, changes)
	assert.Equal(t, 2, pr.Summary.ToCreate)
	assert.Equal(t, 1, pr.Summary.ToUpdate)
	assert.Equal(t, 1, pr.Summary.ToDelete)
	assert.Equal(t, 1, pr.Summary.Noop)
	assert.True(t, pr.HasChanges())
}

func TestPlanResultNoChanges(t *testing.T) {
	changes := []*ResourceChange{
		{ResourceID: "a", ChangeType: ChangeNoop},
	}
	pr := NewPlanResult("test", 1, changes)
	assert.False(t, pr.HasChanges())
}

func TestPlanResultString(t *testing.T) {
	tests := []struct {
		name    string
		changes []*ResourceChange
		want    string
	}{
		{
			name:    "all creates",
			changes: []*ResourceChange{{ResourceID: "a", ChangeType: ChangeCreate}},
			want:    "Plan: 1 to create",
		},
		{
			name: "mixed",
			changes: []*ResourceChange{
				{ResourceID: "a", ChangeType: ChangeCreate},
				{ResourceID: "b", ChangeType: ChangeUpdate},
				{ResourceID: "c", ChangeType: ChangeDelete},
			},
			want: "Plan: 1 to create, 1 to update, 1 to delete",
		},
		{
			name:    "no changes",
			changes: []*ResourceChange{{ResourceID: "a", ChangeType: ChangeNoop}},
			want:    "Plan: 1 noop",
		},
		{
			name:    "empty",
			changes: []*ResourceChange{},
			want:    "Plan: no changes",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr := NewPlanResult("test", 1, tc.changes)
			assert.Contains(t, pr.String(), tc.want)
		})
	}
}
