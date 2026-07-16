package templater

import (
	"strings"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportTerraform_BasicS3(t *testing.T) {
	stack := infra.NewStack("test")
	r := infra.NewResource("my-bucket", "aws_s3_bucket", "aws")
	r.Properties = map[string]any{
		"bucket":     "my-app-production",
		"acl":        "private",
		"versioning": "true",
	}
	stack.AddResource(r)

	result, err := ExportTerraform(stack, "us-east-1")
	require.NoError(t, err)

	assert.Contains(t, result, `terraform`)
	assert.Contains(t, result, `resource "aws_s3_bucket" "my-bucket"`)
	assert.Contains(t, result, `"my-app-production"`)
	assert.Contains(t, result, `region = "us-east-1"`)
}

func TestExportTerraform_WithDependencies(t *testing.T) {
	stack := infra.NewStack("prod")
	db := infra.NewResource("database", "aws_db_instance", "aws")
	db.Properties = map[string]any{"engine": "postgres", "instance_class": "db.r6g.large"}

	app := infra.NewResource("app", "aws_instance", "aws")
	app.Properties = map[string]any{"ami": "ami-123", "instance_type": "t3.micro"}
	app.DependsOn = []infra.ResourceID{"database"}

	stack.AddResource(db)
	stack.AddResource(app)

	result, err := ExportTerraform(stack, "us-west-2")
	require.NoError(t, err)

	assert.Contains(t, result, `resource "aws_db_instance" "database"`)
	assert.Contains(t, result, `resource "aws_instance" "app"`)
	assert.Contains(t, result, `depends_on`)
	assert.Contains(t, result, `region = "us-west-2"`)
}

func TestExportTerraform_UnknownType(t *testing.T) {
	stack := infra.NewStack("test")
	r := infra.NewResource("x", "unknown_type", "aws")
	stack.AddResource(r)

	_, err := ExportTerraform(stack, "us-east-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Terraform mapping")
}

func TestExportTerraform_EmptyStack(t *testing.T) {
	stack := infra.NewStack("empty")
	result, err := ExportTerraform(stack, "us-east-1")
	require.NoError(t, err)
	assert.Contains(t, result, `terraform`)
}

func TestExportCloudFormation_Basic(t *testing.T) {
	stack := infra.NewStack("test")
	r := infra.NewResource("my-bucket", "aws_s3_bucket", "aws")
	r.Properties = map[string]any{"bucket": "my-app"}
	stack.AddResource(r)

	result, err := ExportCloudFormation(stack)
	require.NoError(t, err)

	assert.Contains(t, result, `"AWSTemplateFormatVersion": "2010-09-09"`)
	assert.Contains(t, result, `"my-bucket"`)
	assert.Contains(t, result, `"AWS::S3::Bucket"`)
}

func TestExportCloudFormation_WithDependencies(t *testing.T) {
	stack := infra.NewStack("prod")
	db := infra.NewResource("database", "aws_db_instance", "aws")
	db.Properties = map[string]any{"engine": "postgres"}

	app := infra.NewResource("app", "aws_instance", "aws")
	app.Properties = map[string]any{"ami": "ami-123"}
	app.DependsOn = []infra.ResourceID{"database"}

	stack.AddResource(db)
	stack.AddResource(app)

	result, err := ExportCloudFormation(stack)
	require.NoError(t, err)

	assert.Contains(t, result, `"AWS::RDS::DBInstance"`)
	assert.Contains(t, result, `"AWS::EC2::Instance"`)
	assert.Contains(t, result, `"DependsOn": ["database"]`)
}

func TestExportCloudFormation_UnknownType(t *testing.T) {
	stack := infra.NewStack("test")
	r := infra.NewResource("x", "unknown_type", "aws")
	stack.AddResource(r)

	_, err := ExportCloudFormation(stack)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no CloudFormation mapping")
}

func TestExportTerraform_DefaultRegion(t *testing.T) {
	stack := infra.NewStack("test")
	r := infra.NewResource("b", "aws_s3_bucket", "aws")
	r.Properties = map[string]any{"bucket": "x"}
	stack.AddResource(r)

	result, err := ExportTerraform(stack, "")
	require.NoError(t, err)
	assert.Contains(t, result, `region = "us-east-1"`)
}

func TestTerraformResource_TerraformBlock(t *testing.T) {
	r := &TerraformResource{
		ID:         "my-bucket",
		Type:       "aws_s3_bucket",
		Properties: "  bucket = \"my-app\"\n",
		DependsOn:  `["dependency"]`,
	}

	block := r.TerraformBlock()
	assert.True(t, strings.Contains(block, `resource "aws_s3_bucket" "my-bucket"`))
	assert.True(t, strings.Contains(block, `depends_on`))
}
