// Package state implements infrastructure state backends.
//
// S3 backend is compiled conditionally with the build tag "with_aws".
// Without the tag, this file is excluded and a helpful error is returned
// from the factory function.
//
//go:build with_aws
// +build with_aws

package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	// defaultLockTable is the default DynamoDB table for state locking.
	defaultLockTable = "vyx-infra-locks"

	// lockTTLSeconds is the TTL for lock items in DynamoDB.
	lockTTLSeconds = 120
)

// S3Backend stores infrastructure state in S3 with DynamoDB-based locking.
// This is analogous to Terraform's S3 backend pattern.
type S3Backend struct {
	bucket          string
	key             string
	region          string
	lockTable       string
	s3Client        *s3.Client
	dynamoDBClient  *dynamodb.Client
}

// S3BackendConfig holds configuration for the S3 state backend.
type S3BackendConfig struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	Region    string `json:"region"`
	LockTable string `json:"lock_table,omitempty"`
}

// NewS3Backend creates a new S3 state backend.
func NewS3Backend(cfg S3BackendConfig) (*S3Backend, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 backend: bucket is required")
	}
	if cfg.Key == "" {
		return nil, fmt.Errorf("s3 backend: key is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.LockTable == "" {
		cfg.LockTable = defaultLockTable
	}

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("s3 backend: load AWS config: %w", err)
	}

	return &S3Backend{
		bucket:         cfg.Bucket,
		key:            cfg.Key,
		region:         cfg.Region,
		lockTable:      cfg.LockTable,
		s3Client:       s3.NewFromConfig(awsCfg),
		dynamoDBClient: dynamodb.NewFromConfig(awsCfg),
	}, nil
}

// Init creates the S3 bucket and DynamoDB table if they don't exist.
func (b *S3Backend) Init(ctx context.Context) error {
	// Create S3 bucket if it doesn't exist.
	_, err := b.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(b.bucket),
	})
	if err != nil {
		// Ignore BucketAlreadyOwnedByYou and BucketAlreadyExists.
		if !isBucketExistsError(err) {
			return fmt.Errorf("s3 backend: create bucket %q: %w", b.bucket, err)
		}
	}

	// Enable versioning on the state bucket for history.
	_, err = b.s3Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(b.bucket),
		VersioningConfiguration: &s3types.VersioningConfiguration{
			Status: s3types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return fmt.Errorf("s3 backend: enable versioning on %q: %w", b.bucket, err)
	}

	// Create DynamoDB lock table if it doesn't exist.
	_, err = b.dynamoDBClient.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(b.lockTable),
		AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
			{
				AttributeName: aws.String("LockID"),
				AttributeType: dynamodbtypes.ScalarAttributeTypeS,
			},
		},
		KeySchema: []dynamodbtypes.KeySchemaElement{
			{
				AttributeName: aws.String("LockID"),
				KeyType:       dynamodbtypes.KeyTypeHash,
			},
		},
		BillingMode: dynamodbtypes.BillingModePayPerRequest,
	})
	if err != nil {
		// Ignore TableAlreadyExists.
		if !isTableExistsError(err) {
			return fmt.Errorf("s3 backend: create lock table %q: %w", b.lockTable, err)
		}
	}

	return nil
}

// Lock acquires a lock via DynamoDB conditional write (TTL-based).
func (b *S3Backend) Lock(ctx context.Context, info infra.LockInfo) error {
	now := time.Now()
	info.CreatedAt = now
	if info.TTL == "" {
		info.TTL = "120s"
	}
	ttl := now.Add(lockTTLSeconds * time.Second).Unix()

	item := map[string]dynamodbtypes.AttributeValue{
		"LockID":    &dynamodbtypes.AttributeValueMemberS{Value: "vyx-lock"},
		"Operation": &dynamodbtypes.AttributeValueMemberS{Value: info.Operation},
		"Who":       &dynamodbtypes.AttributeValueMemberS{Value: info.Who},
		"CreatedAt": &dynamodbtypes.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		"TTL":       &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttl)},
	}

	// Conditional write: only succeed if no lock exists.
	_, err := b.dynamoDBClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(b.lockTable),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(LockID)"),
	})
	if err != nil {
		return &infra.ErrLockAcquisition{
			Info: info,
			Err:  fmt.Errorf("dynamodb lock: %w", err),
		}
	}

	return nil
}

// Unlock releases a lock by deleting the DynamoDB item.
func (b *S3Backend) Unlock(ctx context.Context, info infra.LockInfo) error {
	_, err := b.dynamoDBClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(b.lockTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"LockID": &dynamodbtypes.AttributeValueMemberS{Value: "vyx-lock"},
		},
	})
	if err != nil {
		return &infra.ErrLockAcquisition{
			Info: info,
			Err:  fmt.Errorf("dynamodb unlock: %w", err),
		}
	}
	return nil
}

// Get retrieves the state from S3.
func (b *S3Backend) Get(ctx context.Context) (*infra.State, error) {
	result, err := b.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.key),
	})
	if err != nil {
		if isObjectNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("s3 backend: get state %s/%s: %w", b.bucket, b.key, err)
	}
	defer result.Body.Close()

	var state infra.State
	if err := json.NewDecoder(result.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("s3 backend: parse state %s/%s: %w", b.bucket, b.key, err)
	}

	return &state, nil
}

// Put persists the state to S3.
func (b *S3Backend) Put(ctx context.Context, state *infra.State) error {
	state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("s3 backend: marshal state: %w", err)
	}

	_, err = b.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(b.key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("s3 backend: put state %s/%s: %w", b.bucket, b.key, err)
	}

	return nil
}

// Delete removes the state from S3.
func (b *S3Backend) Delete(ctx context.Context) error {
	_, err := b.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.key),
	})
	if err != nil {
		return fmt.Errorf("s3 backend: delete state %s/%s: %w", b.bucket, b.key, err)
	}
	return nil
}

// ─── Error helpers ──────────────────────────────────────────────────────

func isBucketExistsError(err error) bool {
	var bae *s3types.BucketAlreadyExists
	var baoby *s3types.BucketAlreadyOwnedByYou
	return errors.As(err, &bae) || errors.As(err, &baoby)
}

func isTableExistsError(err error) bool {
	var tee *dynamodbtypes.TableAlreadyExistsException
	return errors.As(err, &tee)
}

func isObjectNotFoundError(err error) bool {
	var nf *s3types.NoSuchKey
	return errors.As(err, &nf)
}
