package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStack(t *testing.T) {
	s := NewStack("prod")
	assert.Equal(t, "prod", s.Name)
	assert.Empty(t, s.Resources)
	assert.Empty(t, s.ProviderRefs)
}

func TestStackAddResource(t *testing.T) {
	s := NewStack("test")

	r1 := NewResource("bucket", "aws_s3_bucket", "aws")
	r2 := NewResource("db", "aws_rds_instance", "aws")

	s.AddResource(r1)
	s.AddResource(r2)

	assert.Len(t, s.Resources, 2)
	assert.Len(t, s.ProviderRefs, 1) // only one unique provider
	assert.Equal(t, ProviderID("aws"), s.ProviderRefs[0])
}

func TestStackFindResource(t *testing.T) {
	s := NewStack("test")
	r := NewResource("my-res", "mock", "mock")
	s.AddResource(r)

	found := s.FindResource("my-res")
	assert.Equal(t, r, found)

	notFound := s.FindResource("nonexistent")
	assert.Nil(t, notFound)
}

func TestTopologicalSort(t *testing.T) {
	s := NewStack("test")

	a := NewResource("a", "mock", "mock")
	b := NewResource("b", "mock", "mock")
	b.DependsOn = []ResourceID{"a"}
	c := NewResource("c", "mock", "mock")
	c.DependsOn = []ResourceID{"b"}

	s.AddResource(a)
	s.AddResource(b)
	s.AddResource(c)

	sorted, err := s.TopologicalSort()
	require.NoError(t, err)
	require.Len(t, sorted, 3)

	// a must come before b, b before c
	indexA := indexOf(sorted, "a")
	indexB := indexOf(sorted, "b")
	indexC := indexOf(sorted, "c")
	assert.Less(t, indexA, indexB)
	assert.Less(t, indexB, indexC)
}

func TestTopologicalSortReverse(t *testing.T) {
	s := NewStack("test")

	a := NewResource("a", "mock", "mock")
	b := NewResource("b", "mock", "mock")
	b.DependsOn = []ResourceID{"a"}

	s.AddResource(a)
	s.AddResource(b)

	sorted, err := s.TopologicalSortReverse()
	require.NoError(t, err)
	require.Len(t, sorted, 2)

	// b (dependant) must come first in reverse order
	assert.Equal(t, ResourceID("b"), sorted[0].ID)
	assert.Equal(t, ResourceID("a"), sorted[1].ID)
}

func TestTopologicalSortCycleDetection(t *testing.T) {
	s := NewStack("test")

	a := NewResource("a", "mock", "mock")
	a.DependsOn = []ResourceID{"c"}
	b := NewResource("b", "mock", "mock")
	b.DependsOn = []ResourceID{"a"}
	c := NewResource("c", "mock", "mock")
	c.DependsOn = []ResourceID{"b"}

	s.AddResource(a)
	s.AddResource(b)
	s.AddResource(c)

	_, err := s.TopologicalSort()
	assert.Error(t, err)
	assert.IsType(t, &ErrCycleDetected{}, err)
}

func indexOf(resources []*Resource, id ResourceID) int {
	for i, r := range resources {
		if r.ID == id {
			return i
		}
	}
	return -1
}
