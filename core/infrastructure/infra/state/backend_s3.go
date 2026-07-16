//go:build with_aws
// +build with_aws

package state

import (
	"github.com/ElioNeto/vyx/core/domain/infra"
)

// newS3Backend creates a real S3-backed state backend.
// This file is only compiled when the "with_aws" build tag is active.
func newS3Backend(cfg infra.BackendConfig) (infra.Backend, error) {
	s3Cfg := S3BackendConfig{
		Bucket: "vyx-state",
		Key:    "infra.tfstate",
		Region: "us-east-1",
	}

	if v, ok := cfg.Config["bucket"].(string); ok && v != "" {
		s3Cfg.Bucket = v
	}
	if v, ok := cfg.Config["key"].(string); ok && v != "" {
		s3Cfg.Key = v
	}
	if v, ok := cfg.Config["region"].(string); ok && v != "" {
		s3Cfg.Region = v
	}
	if v, ok := cfg.Config["lock_table"].(string); ok && v != "" {
		s3Cfg.LockTable = v
	}

	return NewS3Backend(s3Cfg)
}
