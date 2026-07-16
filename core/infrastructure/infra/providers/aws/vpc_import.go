//go:build with_aws
// +build with_aws

package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// This file imports the ec2 and ec2types packages so that vpc.go can
// reference types like ec2.CreateVpcInput, ec2.DescribeVpcsInput,
// and ec2types.AttributeBooleanValue.
