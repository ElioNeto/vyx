//go:build with_aws
// +build with_aws

package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// This file imports the iam package so that iam.go can reference
// iam.CreateRoleInput, iam.GetRoleInput, etc. without importing
// the full path in every function.
