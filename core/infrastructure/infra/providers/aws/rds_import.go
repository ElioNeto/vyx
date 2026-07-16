//go:build with_aws
// +build with_aws

package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// This file exists to make the s3 and rds imports used by the resource files
// compile correctly when the build tag is active.
// The actual s3 and rds packages are imported here so that rds.go and s3.go
// can use the short-form `s3.CreateBucketInput` and `rds.CreateDBInstanceInput`
// without importing the full path each time.
