//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps multiple AWS service clients.
type Client struct {
	Config     aws.Config
	S3         *s3.Client
	EC2        *ec2.Client
	RDS        *rds.Client
	IAM        *iam.Client
	Lambda     *lambda.Client
}

// NewClient creates a new AWS client with the given region.
func NewClient(region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return &Client{
		Config: cfg,
		S3:     s3.NewFromConfig(cfg),
		EC2:    ec2.NewFromConfig(cfg),
		RDS:    rds.NewFromConfig(cfg),
		IAM:    iam.NewFromConfig(cfg),
		Lambda: lambda.NewFromConfig(cfg),
	}, nil
}

// Region returns the configured AWS region.
func (c *Client) Region() string {
	return c.Config.Region
}
