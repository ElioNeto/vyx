package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeInfraFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestParseInfraGoFile_BasicResource(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package infra

// @Resource(type: "aws_s3_bucket", id: "assets")
// @Provider(aws)
// @Tags(env: "production", team: "platform")
// @Output(bucket_arn)
// @Output(bucket_domain)
func defineStorage() {}
`
	writeInfraFile(t, tmpDir, "storage.go", src)

	resources, errs := ParseInfraFiles(tmpDir)
	require.Empty(t, errs)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "aws_s3_bucket", r.Type)
	assert.Equal(t, "assets", r.ID)
	assert.Equal(t, "aws", r.ProviderName)
	assert.Equal(t, "production", r.Tags["env"])
	assert.Equal(t, "platform", r.Tags["team"])
	assert.Equal(t, []string{"bucket_arn", "bucket_domain"}, r.Outputs)
}

func TestParseInfraGoFile_MultipleResources(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package infra

// @Resource(type: "aws_s3_bucket", id: "assets")
// @Provider(aws)
func defineStorage() {}

// @Resource(type: "aws_rds_instance", id: "database")
// @Provider(aws)
// @DependsOn(assets)
func defineDatabase() {}
`
	writeInfraFile(t, tmpDir, "resources.go", src)

	resources, errs := ParseInfraFiles(tmpDir)
	require.Empty(t, errs)
	require.Len(t, resources, 2)

	assert.Equal(t, "aws_s3_bucket", resources[0].Type)
	assert.Equal(t, "assets", resources[0].ID)

	assert.Equal(t, "aws_rds_instance", resources[1].Type)
	assert.Equal(t, "database", resources[1].ID)
	assert.Equal(t, []string{"assets"}, resources[1].DependsOn)
}

func TestParseInfraGoFile_DefaultID(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package infra

// @Resource(type: "aws_s3_bucket")
// @Provider(aws)
func defineStorage() {}
`
	writeInfraFile(t, tmpDir, "storage.go", src)

	resources, errs := ParseInfraFiles(tmpDir)
	require.Empty(t, errs)
	require.Len(t, resources, 1)

	// Default ID should be filename_base + _ + line
	assert.Equal(t, "storage_3", resources[0].ID)
}

func TestParseInfraPyFile_BasicResource(t *testing.T) {
	tmpDir := t.TempDir()
	src := `# infra/database.py
# @Resource(type: "aws_rds_instance", id: "main-db")
# @Provider(aws)
# @Tags(env: "staging")
# @Output(host)
# @Output(port)

def create_database():
    pass
`
	writeInfraFile(t, tmpDir, "database.py", src)

	resources, errs := ParseInfraFiles(tmpDir)
	require.Empty(t, errs)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "aws_rds_instance", r.Type)
	assert.Equal(t, "main-db", r.ID)
	assert.Equal(t, "aws", r.ProviderName)
	assert.Equal(t, "staging", r.Tags["env"])
	assert.Equal(t, []string{"host", "port"}, r.Outputs)
}

func TestParseInfraYAMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := `# infrastructure resources
# @Resource(type: "aws_sqs_queue", id: "orders-queue")
# @Provider(aws)
# @Tags(env: "production")
# @Output(queue_arn)
# @Output(queue_url)

resources:
  - type: aws_sqs_queue
    id: orders-queue
`
	writeInfraFile(t, tmpDir, "infra.yaml", src)

	resources, errs := ParseInfraFiles(tmpDir)
	require.Empty(t, errs)
	require.Len(t, resources, 1)

	r := resources[0]
	assert.Equal(t, "aws_sqs_queue", r.Type)
	assert.Equal(t, "orders-queue", r.ID)
	assert.Equal(t, "aws", r.ProviderName)
	assert.Equal(t, "production", r.Tags["env"])
	assert.Equal(t, []string{"queue_arn", "queue_url"}, r.Outputs)
}

func TestParseInfraFiles_FileOpenError(t *testing.T) {
	routes, errs := parseInfraFile("/nonexistent/path/file.go", "//")
	assert.Empty(t, routes)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, "cannot open file")
}

func TestParseInfraFiles_NonExistentDir(t *testing.T) {
	resources, errs := ParseInfraFiles("/nonexistent/path")
	assert.Empty(t, resources)
	assert.Empty(t, errs)
}

func TestParseInfraFile_MissingProvider(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package infra

// @Resource(type: "aws_s3_bucket", id: "test")
// No @Provider annotation
func defineStorage() {}
`
	writeInfraFile(t, tmpDir, "storage.go", src)

	resources, errs := ParseInfraFiles(tmpDir)
	require.Empty(t, errs)
	require.Len(t, resources, 1)

	// Should default to "unknown"
	assert.Equal(t, "unknown", resources[0].ProviderName)
}

func TestParseInfraFile_DependsOn(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package infra

// @Resource(type: "aws_lambda_function", id: "my-func")
// @Provider(aws)
// @DependsOn(role-1, bucket-1)
// @DependsOn(queue-1)
func defineFunction() {}
`
	writeInfraFile(t, tmpDir, "function.go", src)

	resources, errs := ParseInfraFiles(tmpDir)
	require.Empty(t, errs)
	require.Len(t, resources, 1)

	expectedDeps := []string{"role-1", "bucket-1", "queue-1"}
	assert.Equal(t, expectedDeps, resources[0].DependsOn)
}

func TestParseTagList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]string
	}{
		{
			name: "single tag",
			raw:  `env: "production"`,
			want: map[string]string{"env": "production"},
		},
		{
			name: "multiple tags",
			raw:  `env: "production", team: "platform"`,
			want: map[string]string{"env": "production", "team": "platform"},
		},
		{
			name: "tags with single quotes",
			raw:  `env: 'staging', owner: 'devops'`,
			want: map[string]string{"env": "staging", "owner": "devops"},
		},
		{
			name: "empty tags",
			raw:  ``,
			want: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTagList(tc.raw)
			assert.Equal(t, tc.want, got)
		})
	}
}
