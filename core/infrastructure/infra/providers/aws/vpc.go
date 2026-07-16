//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func planVPC(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"cidr_block", "enable_dns_support", "enable_dns_hostnames", "instance_tenancy"}
	for _, field := range fields {
		newVal := desired.Properties[field]
		oldVal := current.Properties[field]
		if !valuesEqual(oldVal, newVal) {
			diff[field] = infra.DiffValue{Old: oldVal, New: newVal}
		}
	}
	if len(diff) == 0 {
		return &infra.ResourceChange{
			ResourceID: desired.ID, ChangeType: infra.ChangeNoop,
			ResourceType: desired.Type, ProviderName: "aws",
		}, nil
	}
	return &infra.ResourceChange{
		ResourceID: desired.ID, ChangeType: infra.ChangeUpdate,
		ResourceType: desired.Type, ProviderName: "aws", Diff: diff,
	}, nil
}

func createVPC(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	cidr := getStringProp(r.Properties, "cidr_block", "10.0.0.0/16")

	result, err := client.EC2.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String(cidr),
	})
	if err != nil {
		return nil, fmt.Errorf("create VPC %q: %w", r.ID, err)
	}

	vpcID := aws.ToString(result.Vpc.VpcId)
	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":         vpcID,
		"arn":        fmt.Sprintf("arn:aws:ec2:%s:%s:vpc/%s", client.Region(), "", vpcID),
		"cidr_block": cidr,
	}
	return created, nil
}

func readVPC(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	vpcID := r.Outputs["id"]
	if vpcID == "" {
		return nil, fmt.Errorf("read VPC %q: no VPC ID in state", r.ID)
	}

	result, err := client.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	if err != nil {
		return nil, fmt.Errorf("read VPC %q: %w", r.ID, err)
	}

	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	if len(result.Vpcs) > 0 {
		vpc := result.Vpcs[0]
		existing.Outputs = map[string]string{
			"id":  aws.ToString(vpc.VpcId),
			"arn": fmt.Sprintf("arn:aws:ec2:%s:%s:vpc/%s", client.Region(), "", aws.ToString(vpc.VpcId)),
		}
	}
	return existing, nil
}

func updateVPC(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	// VPC updates are limited (DNS support, hostnames)
	vpcID := current.Outputs["id"]
	if vpcID == "" {
		return nil, fmt.Errorf("update VPC %q: no VPC ID in state", desired.ID)
	}

	if desired.Properties["enable_dns_support"] != current.Properties["enable_dns_support"] {
		_, err := client.EC2.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
			VpcId: aws.String(vpcID),
			EnableDnsSupport: &ec2types.AttributeBooleanValue{
				Value: aws.Bool(getBoolProp(desired.Properties, "enable_dns_support", true)),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("update VPC %q DNS support: %w", desired.ID, err)
		}
	}

	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteVPC(ctx context.Context, client *Client, r *infra.Resource) error {
	vpcID := r.Outputs["id"]
	if vpcID == "" {
		return fmt.Errorf("delete VPC %q: no VPC ID in state", r.ID)
	}

	_, err := client.EC2.DeleteVpc(ctx, &ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		return fmt.Errorf("delete VPC %q: %w", r.ID, err)
	}
	return nil
}
