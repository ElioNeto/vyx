//go:build with_aws
// +build with_aws

package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// This file imports the lambda package for compile-time resolution
// of types used in lambda.go.
