//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func planS3Bucket(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)

	fields := []string{"bucket", "acl", "versioning", "encryption", "force_destroy"}
	for _, field := range fields {
		newVal := desired.Properties[field]
		oldVal := current.Properties[field]
		if !valuesEqual(oldVal, newVal) {
			diff[field] = infra.DiffValue{Old: oldVal, New: newVal}
		}
	}

	if len(diff) == 0 {
		return &infra.ResourceChange{
			ResourceID:   desired.ID,
			ChangeType:   infra.ChangeNoop,
			ResourceType: desired.Type,
			ProviderName: "aws",
		}, nil
	}

	return &infra.ResourceChange{
		ResourceID:   desired.ID,
		ChangeType:   infra.ChangeUpdate,
		ResourceType: desired.Type,
		ProviderName: "aws",
		Diff:         diff,
	}, nil
}

func createS3Bucket(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	bucketName := getStringProp(r.Properties, "bucket", string(r.ID))

	_, err := client.S3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 bucket %q: %w", bucketName, err)
	}

	// Enable versioning if requested
	if getBoolProp(r.Properties, "versioning", false) {
		_, err := client.S3.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
			Bucket: aws.String(bucketName),
			VersioningConfiguration: &types.VersioningConfiguration{
				Status: types.BucketVersioningStatusEnabled,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("enable S3 bucket versioning %q: %w", bucketName, err)
		}
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"arn":               fmt.Sprintf("arn:aws:s3:::%s", bucketName),
		"bucket_domain_name": fmt.Sprintf("%s.s3.amazonaws.com", bucketName),
		"region":            client.Region(),
	}
	return created, nil
}

func readS3Bucket(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	bucketName := getStringProp(r.Properties, "bucket", string(r.ID))

	_, err := client.S3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil, fmt.Errorf("read S3 bucket %q: %w", bucketName, err)
	}

	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	if existing.Outputs == nil {
		existing.Outputs = make(map[string]string)
	}
	existing.Outputs["arn"] = fmt.Sprintf("arn:aws:s3:::%s", bucketName)
	existing.Outputs["region"] = client.Region()
	return existing, nil
}

func updateS3Bucket(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	bucketName := getStringProp(desired.Properties, "bucket", string(desired.ID))

	// Update versioning if changed
	desiredVersioning := getBoolProp(desired.Properties, "versioning", false)
	currentVersioning := getBoolProp(current.Properties, "versioning", false)
	if desiredVersioning != currentVersioning {
		status := types.BucketVersioningStatusSuspended
		if desiredVersioning {
			status = types.BucketVersioningStatusEnabled
		}
		_, err := client.S3.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
			Bucket: aws.String(bucketName),
			VersioningConfiguration: &types.VersioningConfiguration{
				Status: status,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("update S3 bucket versioning %q: %w", bucketName, err)
		}
	}

	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = map[string]string{
		"arn":               fmt.Sprintf("arn:aws:s3:::%s", bucketName),
		"bucket_domain_name": fmt.Sprintf("%s.s3.amazonaws.com", bucketName),
		"region":            client.Region(),
	}
	return updated, nil
}

func deleteS3Bucket(ctx context.Context, client *Client, r *infra.Resource) error {
	bucketName := getStringProp(r.Properties, "bucket", string(r.ID))

	_, err := client.S3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("delete S3 bucket %q: %w", bucketName, err)
	}
	return nil
}
