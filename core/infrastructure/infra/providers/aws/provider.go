// Package aws implements the AWS cloud provider for the vyx IaC module.
//
// This provider is compiled conditionally with the build tag "with_aws".
// To include it:
//
//	go build -tags with_aws ./cmd/vyx
//
// Without the tag, the AWS provider and its SDK dependency are excluded
// from the binary.
//
//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// ProviderID is the unique identifier for the AWS provider.
const ProviderID infra.ProviderID = "aws"

// Provider implements the infra.Provider interface for Amazon Web Services.
type Provider struct {
	id       infra.ProviderID
	region   string
	client   *Client
}

// New creates a new AWS provider.
func New(config map[string]any) (*Provider, error) {
	region := "us-east-1"
	if r, ok := config["region"].(string); ok && r != "" {
		region = r
	}

	client, err := NewClient(region)
	if err != nil {
		return nil, fmt.Errorf("aws provider: create client: %w", err)
	}

	return &Provider{
		id:     ProviderID,
		region: region,
		client: client,
	}, nil
}

// ID returns the provider identifier.
func (p *Provider) ID() infra.ProviderID {
	return p.id
}

// Validate checks that the resource has required properties.
func (p *Provider) Validate(ctx context.Context, r *infra.Resource) error {
	if r.Properties == nil {
		return &infra.ErrValidation{
			ResourceID: r.ID,
			Message:    "properties is required",
		}
	}
	return nil
}

// Plan determines what changes are needed for a resource.
func (p *Provider) Plan(ctx context.Context, desired, current *infra.Resource) (*infra.ResourceChange, error) {
	if current == nil || current.State == infra.ResourceStateDeleted || current.State == infra.ResourceStatePending {
		return &infra.ResourceChange{
			ResourceID:   desired.ID,
			ChangeType:   infra.ChangeCreate,
			ResourceType: desired.Type,
			ProviderName: p.id,
		}, nil
	}

	// Delegate to resource-specific planner
	return p.planResource(ctx, desired, current)
}

// Create provisions a new resource in AWS.
func (p *Provider) Create(ctx context.Context, r *infra.Resource) (*infra.Resource, error) {
	return p.createResource(ctx, r)
}

// Read queries the current state of an existing AWS resource.
func (p *Provider) Read(ctx context.Context, r *infra.Resource) (*infra.Resource, error) {
	return p.readResource(ctx, r)
}

// Update modifies an existing resource to match the desired state.
func (p *Provider) Update(ctx context.Context, desired, current *infra.Resource) (*infra.Resource, error) {
	return p.updateResource(ctx, desired, current)
}

// Delete destroys a resource in AWS.
func (p *Provider) Delete(ctx context.Context, r *infra.Resource) error {
	return p.deleteResource(ctx, r)
}

// Capabilities returns the resource types this provider can manage.
func (p *Provider) Capabilities() []infra.ResourceCapability {
	return []infra.ResourceCapability{
		{
			Type:        "aws_s3_bucket",
			Description: "Amazon S3 bucket for object storage",
			InputSchema: map[string]infra.SchemaField{
				"bucket":       {Type: "string", Required: true, Description: "Bucket name"},
				"acl":          {Type: "string", Required: false, Default: "private", Description: "Bucket ACL"},
				"versioning":   {Type: "bool", Required: false, Default: false, Description: "Enable versioning"},
				"encryption":   {Type: "string", Required: false, Default: "AES256", Description: "Server-side encryption"},
				"force_destroy": {Type: "bool", Required: false, Default: false, Description: "Force destroy even if non-empty"},
			},
			OutputFields: []string{"arn", "bucket_domain_name", "region"},
		},
		{
			Type:        "aws_instance",
			Description: "Amazon EC2 instance",
			InputSchema: map[string]infra.SchemaField{
				"ami":              {Type: "string", Required: true, Description: "AMI ID"},
				"instance_type":    {Type: "string", Required: true, Description: "Instance type (e.g. t3.micro)"},
				"subnet_id":        {Type: "string", Required: false, Description: "VPC subnet ID"},
				"key_name":         {Type: "string", Required: false, Description: "SSH key pair name"},
				"security_groups":  {Type: "list(string)", Required: false, Description: "Security group IDs"},
				"user_data":        {Type: "string", Required: false, Description: "User data script"},
				"ebs_optimized":    {Type: "bool", Required: false, Default: false, Description: "EBS optimized"},
			},
			OutputFields: []string{"id", "arn", "public_ip", "private_ip", "public_dns", "availability_zone"},
		},
		{
			Type:        "aws_db_instance",
			Description: "Amazon RDS database instance",
			InputSchema: map[string]infra.SchemaField{
				"engine":             {Type: "string", Required: true, Description: "Database engine (postgres, mysql, etc.)"},
				"engine_version":     {Type: "string", Required: true, Description: "Engine version"},
				"instance_class":     {Type: "string", Required: true, Description: "Instance class (e.g. db.r6g.large)"},
				"allocated_storage":  {Type: "number", Required: true, Description: "Storage in GB"},
				"db_name":            {Type: "string", Required: false, Description: "Database name"},
				"username":           {Type: "string", Required: true, Description: "Master username"},
				"multi_az":           {Type: "bool", Required: false, Default: false, Description: "Multi-AZ deployment"},
				"publicly_accessible": {Type: "bool", Required: false, Default: false, Description: "Public accessibility"},
			},
			OutputFields: []string{"id", "arn", "endpoint", "port", "address"},
		},
		{
			Type:        "aws_iam_role",
			Description: "AWS IAM role",
			InputSchema: map[string]infra.SchemaField{
				"name":             {Type: "string", Required: true, Description: "Role name"},
				"assume_role_policy": {Type: "string", Required: true, Description: "Trust policy JSON"},
				"description":      {Type: "string", Required: false, Description: "Role description"},
				"max_session_duration": {Type: "number", Required: false, Default: 3600, Description: "Max session duration in seconds"},
			},
			OutputFields: []string{"arn", "unique_id", "name"},
		},
		{
			Type:        "aws_lambda_function",
			Description: "AWS Lambda function",
			InputSchema: map[string]infra.SchemaField{
				"function_name": {Type: "string", Required: true, Description: "Function name"},
				"runtime":       {Type: "string", Required: true, Description: "Runtime (nodejs20.x, python3.12, etc.)"},
				"handler":       {Type: "string", Required: true, Description: "Function handler"},
				"role_arn":      {Type: "string", Required: true, Description: "IAM role ARN"},
				"memory_size":   {Type: "number", Required: false, Default: 128, Description: "Memory in MB"},
				"timeout":       {Type: "number", Required: false, Default: 30, Description: "Timeout in seconds"},
				"environment":   {Type: "map(string)", Required: false, Description: "Environment variables"},
			},
			OutputFields: []string{"function_arn", "invoke_arn", "qualified_arn"},
		},
		{
			Type:        "aws_vpc",
			Description: "Amazon VPC",
			InputSchema: map[string]infra.SchemaField{
				"cidr_block":       {Type: "string", Required: true, Description: "CIDR block (e.g. 10.0.0.0/16)"},
				"enable_dns_support": {Type: "bool", Required: false, Default: true, Description: "Enable DNS support"},
				"enable_dns_hostnames": {Type: "bool", Required: false, Default: false, Description: "Enable DNS hostnames"},
				"instance_tenancy": {Type: "string", Required: false, Default: "default", Description: "Tenancy (default or dedicated)"},
			},
			OutputFields: []string{"id", "arn", "default_network_acl_id", "default_security_group_id"},
		},
	}
}

// ─── Resource dispatch ──────────────────────────────────────────────────

func (p *Provider) planResource(ctx context.Context, desired, current *infra.Resource) (*infra.ResourceChange, error) {
	switch desired.Type {
	case "aws_s3_bucket":
		return planS3Bucket(desired, current)
	case "aws_instance":
		return planEC2Instance(desired, current)
	case "aws_db_instance":
		return planRDSInstance(desired, current)
	case "aws_iam_role":
		return planIAMRole(desired, current)
	case "aws_lambda_function":
		return planLambdaFunction(desired, current)
	case "aws_vpc":
		return planVPC(desired, current)
	default:
		return nil, &infra.ErrValidation{
			ResourceID: desired.ID,
			Message:    fmt.Sprintf("unsupported AWS resource type: %s", desired.Type),
		}
	}
}

func (p *Provider) createResource(ctx context.Context, r *infra.Resource) (*infra.Resource, error) {
	switch r.Type {
	case "aws_s3_bucket":
		return createS3Bucket(ctx, p.client, r)
	case "aws_instance":
		return createEC2Instance(ctx, p.client, r)
	case "aws_db_instance":
		return createRDSInstance(ctx, p.client, r)
	case "aws_iam_role":
		return createIAMRole(ctx, p.client, r)
	case "aws_lambda_function":
		return createLambdaFunction(ctx, p.client, r)
	case "aws_vpc":
		return createVPC(ctx, p.client, r)
	default:
		return nil, &infra.ErrValidation{
			ResourceID: r.ID,
			Message:    fmt.Sprintf("unsupported AWS resource type: %s", r.Type),
		}
	}
}

func (p *Provider) readResource(ctx context.Context, r *infra.Resource) (*infra.Resource, error) {
	switch r.Type {
	case "aws_s3_bucket":
		return readS3Bucket(ctx, p.client, r)
	case "aws_instance":
		return readEC2Instance(ctx, p.client, r)
	case "aws_db_instance":
		return readRDSInstance(ctx, p.client, r)
	case "aws_iam_role":
		return readIAMRole(ctx, p.client, r)
	case "aws_lambda_function":
		return readLambdaFunction(ctx, p.client, r)
	case "aws_vpc":
		return readVPC(ctx, p.client, r)
	default:
		return nil, &infra.ErrValidation{
			ResourceID: r.ID,
			Message:    fmt.Sprintf("unsupported AWS resource type: %s", r.Type),
		}
	}
}

func (p *Provider) updateResource(ctx context.Context, desired, current *infra.Resource) (*infra.Resource, error) {
	switch desired.Type {
	case "aws_s3_bucket":
		return updateS3Bucket(ctx, p.client, desired, current)
	case "aws_instance":
		return updateEC2Instance(ctx, p.client, desired, current)
	case "aws_db_instance":
		return updateRDSInstance(ctx, p.client, desired, current)
	case "aws_iam_role":
		return updateIAMRole(ctx, p.client, desired, current)
	case "aws_lambda_function":
		return updateLambdaFunction(ctx, p.client, desired, current)
	case "aws_vpc":
		return updateVPC(ctx, p.client, desired, current)
	default:
		return nil, &infra.ErrValidation{
			ResourceID: desired.ID,
			Message:    fmt.Sprintf("unsupported AWS resource type: %s", desired.Type),
		}
	}
}

func (p *Provider) deleteResource(ctx context.Context, r *infra.Resource) error {
	switch r.Type {
	case "aws_s3_bucket":
		return deleteS3Bucket(ctx, p.client, r)
	case "aws_instance":
		return deleteEC2Instance(ctx, p.client, r)
	case "aws_db_instance":
		return deleteRDSInstance(ctx, p.client, r)
	case "aws_iam_role":
		return deleteIAMRole(ctx, p.client, r)
	case "aws_lambda_function":
		return deleteLambdaFunction(ctx, p.client, r)
	case "aws_vpc":
		return deleteVPC(ctx, p.client, r)
	default:
		return &infra.ErrValidation{
			ResourceID: r.ID,
			Message:    fmt.Sprintf("unsupported AWS resource type: %s", r.Type),
		}
	}
}
