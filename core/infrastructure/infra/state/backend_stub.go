// Package state implements infrastructure state backends.
//
// This file provides a stub implementation that returns an error.
// When the "with_aws" build tag is active, backend_s3.go replaces it
// with a real S3 backend.
//
//go:build !with_aws
// +build !with_aws

package state

import (
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// newS3Backend is a build-tag-resolved function.
// Without the "with_aws" tag, it returns a "not implemented" error.
// With the tag (see backend_s3.go), it creates the real S3 backend.
func newS3Backend(cfg infra.BackendConfig) (infra.Backend, error) {
	return nil, fmt.Errorf("s3 backend requires build with -tags with_aws")
}
