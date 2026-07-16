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

func planEC2Instance(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"ami", "instance_type", "subnet_id", "key_name", "ebs_optimized"}
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

func createEC2Instance(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	ami := getStringProp(r.Properties, "ami", "")
	if ami == "" {
		return nil, fmt.Errorf("ec2 instance %q: ami is required", r.ID)
	}
	instanceType := getStringProp(r.Properties, "instance_type", "t3.micro")

	input := &ec2.RunInstancesInput{
		ImageId:      aws.String(ami),
		InstanceType: ec2types.InstanceType(instanceType),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	}

	if subnetID := getStringProp(r.Properties, "subnet_id", ""); subnetID != "" {
		input.SubnetId = aws.String(subnetID)
	}
	if keyName := getStringProp(r.Properties, "key_name", ""); keyName != "" {
		input.KeyName = aws.String(keyName)
	}

	result, err := client.EC2.RunInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create EC2 instance %q: %w", r.ID, err)
	}
	if len(result.Instances) == 0 {
		return nil, fmt.Errorf("create EC2 instance %q: no instances returned", r.ID)
	}

	instance := result.Instances[0]
	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":                aws.ToString(instance.InstanceId),
		"arn":               fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", client.Region(), "", aws.ToString(instance.InstanceId)),
		"public_ip":         aws.ToString(instance.PublicIpAddress),
		"private_ip":        aws.ToString(instance.PrivateIpAddress),
		"availability_zone": aws.ToString(instance.Placement.AvailabilityZone),
	}
	return created, nil
}

func readEC2Instance(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	instanceID := getStringProp(r.Properties, "instance_id", "")
	if instanceID == "" {
		instanceID = r.Outputs["id"]
	}
	if instanceID == "" {
		return nil, fmt.Errorf("read EC2 instance %q: no instance ID available", r.ID)
	}

	result, err := client.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("read EC2 instance %q: %w", r.ID, err)
	}

	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	if len(result.Reservations) > 0 && len(result.Reservations[0].Instances) > 0 {
		inst := result.Reservations[0].Instances[0]
		existing.Outputs = map[string]string{
			"id":                aws.ToString(inst.InstanceId),
			"public_ip":         aws.ToString(inst.PublicIpAddress),
			"private_ip":        aws.ToString(inst.PrivateIpAddress),
			"availability_zone": aws.ToString(inst.Placement.AvailabilityZone),
		}
	}
	return existing, nil
}

func updateEC2Instance(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	// EC2 updates are limited (instance type, security groups, etc.)
	// For Phase 2, we implement basic updates.
	instanceID := current.Outputs["id"]
	if instanceID == "" {
		return nil, fmt.Errorf("update EC2 instance %q: no instance ID in state", desired.ID)
	}

	// Update instance type if changed
	newType := getStringProp(desired.Properties, "instance_type", "")
	oldType := getStringProp(current.Properties, "instance_type", "")
	if newType != "" && newType != oldType {
		_, err := client.EC2.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
			InstanceId:   aws.String(instanceID),
			InstanceType: &ec2types.AttributeValue{Value: aws.String(newType)},
		})
		if err != nil {
			return nil, fmt.Errorf("update EC2 instance %q type: %w", desired.ID, err)
		}
	}

	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteEC2Instance(ctx context.Context, client *Client, r *infra.Resource) error {
	instanceID := r.Outputs["id"]
	if instanceID == "" {
		instanceID = getStringProp(r.Properties, "instance_id", "")
	}
	if instanceID == "" {
		return fmt.Errorf("delete EC2 instance %q: no instance ID", r.ID)
	}

	_, err := client.EC2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("delete EC2 instance %q: %w", r.ID, err)
	}
	return nil
}
