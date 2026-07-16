package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewState(t *testing.T) {
	s := NewState("production")

	assert.Equal(t, uint64(1), s.Serial)
	assert.Equal(t, "production", s.Metadata.StackName)
	assert.Empty(t, s.Resources)
	assert.NotZero(t, s.CreatedAt)
	assert.NotZero(t, s.UpdatedAt)
	assert.Equal(t, s.CreatedAt, s.UpdatedAt)
}

func TestStateWithResources(t *testing.T) {
	s := NewState("test")

	r := NewResource("bucket-1", "aws_s3_bucket", "aws")
	s.Resources = append(s.Resources, r)
	s.ProviderIDs = append(s.ProviderIDs, "aws")

	assert.Len(t, s.Resources, 1)
	assert.Len(t, s.ProviderIDs, 1)
}
