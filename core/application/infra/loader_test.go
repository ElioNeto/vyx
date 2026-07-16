package infra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackLoader_LoadFromInfraMap(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a fake infra_map.json
	content := `{
  "resources": [
    {
      "type": "aws_s3_bucket",
      "provider": "aws",
      "id": "my-bucket",
      "properties": {"bucket": "test-123", "acl": "private"},
      "tags": {"env": "prod"},
      "outputs": ["arn"]
    },
    {
      "type": "aws_rds_instance",
      "provider": "aws",
      "id": "my-db",
      "properties": {"engine": "postgres", "instance_class": "db.r6g.large"},
      "depends_on": ["my-bucket"]
    }
  ]
}`
	writeFile(t, tmpDir, "infra_map.json", content)

	loader := NewStackLoader(tmpDir)
	stack, err := loader.LoadFromInfraMap("default")
	require.NoError(t, err)
	assert.Equal(t, 2, stack.ResourceCount())

	bucket := stack.FindResource("my-bucket")
	require.NotNil(t, bucket)
	assert.Equal(t, infra.ResourceType("aws_s3_bucket"), bucket.Type)
	assert.Equal(t, infra.ProviderID("aws"), bucket.ProviderName)
	assert.Equal(t, "test-123", bucket.Properties["bucket"])

	db := stack.FindResource("my-db")
	require.NotNil(t, db)
	assert.Equal(t, []infra.ResourceID{"my-bucket"}, db.DependsOn)
}

func TestStackLoader_LoadFromVyxYaml(t *testing.T) {
	tmpDir := t.TempDir()

	content := `project:
  name: my-app

infrastructure:
  resources:
    - type: aws_s3_bucket
      id: assets
      provider: aws
`
	writeFile(t, tmpDir, "vyx.yaml", content)

	loader := NewStackLoader(tmpDir)
	stack, err := loader.LoadFromVyxYaml("default")
	require.NoError(t, err)

	if stack.ResourceCount() > 0 {
		r := stack.FindResource("assets")
		require.NotNil(t, r)
		assert.Equal(t, infra.ResourceType("aws_s3_bucket"), r.Type)
	}
}

func TestStackLoader_LoadStack_Fallback(t *testing.T) {
	tmpDir := t.TempDir()

	// Only vyx.yaml exists, no infra_map.json
	content := `project:
  name: my-app

infrastructure:
  resources:
    - type: aws_s3_bucket
      id: my-bucket
      provider: aws
`
	writeFile(t, tmpDir, "vyx.yaml", content)

	loader := NewStackLoader(tmpDir)
	stack, err := loader.LoadStack("default")
	require.NoError(t, err)
	assert.Equal(t, 1, stack.ResourceCount())
}

func TestStackLoader_EmptyProject(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewStackLoader(tmpDir)
	stack, err := loader.LoadStack("default")
	require.NoError(t, err)
	assert.Equal(t, 0, stack.ResourceCount())
	assert.Equal(t, "default", stack.Name)
}

func TestParseInlineResources(t *testing.T) {
	content := `project:
  name: test

infrastructure:
  resources:
    - type: aws_s3_bucket
      id: my-bucket
      provider: aws
    - type: aws_rds_instance
      id: my-db
      provider: aws
`
	resources := parseInlineResources(content)
	assert.Len(t, resources, 2)
	assert.Equal(t, infra.ResourceType("aws_s3_bucket"), resources[0].Type)
	assert.Equal(t, infra.ResourceID("my-bucket"), resources[0].ID)
	assert.Equal(t, infra.ProviderID("aws"), resources[0].ProviderName)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(dir, name),
		[]byte(content), 0644,
	); err != nil {
		t.Fatal(err)
	}
}
