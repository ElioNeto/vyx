package infra

import (
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphGeneratorMermaid(t *testing.T) {
	stack := buildTestStack()
	gen := NewGraphGenerator()

	result, err := gen.Generate(stack, GraphFormatMermaid)
	require.NoError(t, err)

	assert.Contains(t, result, "graph TD")
	assert.Contains(t, result, "Infrastructure: test")
	assert.Contains(t, result, "Provider: mock")
	assert.Contains(t, result, "subgraph")
	assert.Contains(t, result, "-->")
}

func TestGraphGeneratorDot(t *testing.T) {
	stack := buildTestStack()
	gen := NewGraphGenerator()

	result, err := gen.Generate(stack, GraphFormatDot)
	require.NoError(t, err)

	assert.Contains(t, result, "digraph infra")
	assert.Contains(t, result, "rankdir=LR")
	assert.Contains(t, result, "->")
}

func TestGraphGeneratorInvalidFormat(t *testing.T) {
	stack := buildTestStack()
	gen := NewGraphGenerator()

	_, err := gen.Generate(stack, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported graph format")
}

func TestGraphGeneratorEmptyStack(t *testing.T) {
	stack := infra.NewStack("empty")
	gen := NewGraphGenerator()

	mermaid, err := gen.Generate(stack, GraphFormatMermaid)
	require.NoError(t, err)
	assert.Contains(t, mermaid, "graph TD")
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-bucket", "my_bucket"},
		{"my.bucket", "my_bucket"},
		{"my:bucket", "my_bucket"},
		{"my/bucket", "my_bucket"},
		{"123bucket", "n_123bucket"},
		{"simple", "simple"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeID(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func buildTestStack() *infra.Stack {
	stack := infra.NewStack("test")
	a := infra.NewResource("db", "aws_rds_instance", "mock")
	b := infra.NewResource("app", "aws_ecs_service", "mock")
	b.DependsOn = []infra.ResourceID{"db"}
	c := infra.NewResource("dns", "aws_route53_record", "mock")
	c.DependsOn = []infra.ResourceID{"app"}
	stack.AddResource(a)
	stack.AddResource(b)
	stack.AddResource(c)
	return stack
}
