//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

func planIAMRole(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"name", "assume_role_policy", "description"}
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

func createIAMRole(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	roleName := getStringProp(r.Properties, "name", string(r.ID))
	policyDoc := getStringProp(r.Properties, "assume_role_policy", `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`)

	result, err := client.IAM.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(policyDoc),
		Description:              aws.String(getStringProp(r.Properties, "description", "")),
	})
	if err != nil {
		return nil, fmt.Errorf("create IAM role %q: %w", roleName, err)
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"arn":       aws.ToString(result.Role.Arn),
		"name":      aws.ToString(result.Role.RoleName),
		"unique_id": aws.ToString(result.Role.RoleId),
	}
	return created, nil
}

func readIAMRole(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	roleName := getStringProp(r.Properties, "name", string(r.ID))

	result, err := client.IAM.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return nil, fmt.Errorf("read IAM role %q: %w", roleName, err)
	}

	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	existing.Outputs = map[string]string{
		"arn":  aws.ToString(result.Role.Arn),
		"name": aws.ToString(result.Role.RoleName),
	}
	return existing, nil
}

func updateIAMRole(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	roleName := getStringProp(desired.Properties, "name", string(desired.ID))
	policyDoc := getStringProp(desired.Properties, "assume_role_policy", "")

	if policyDoc != "" {
		_, err := client.IAM.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
			RoleName:       aws.String(roleName),
			PolicyDocument: aws.String(policyDoc),
		})
		if err != nil {
			return nil, fmt.Errorf("update IAM role %q policy: %w", roleName, err)
		}
	}

	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteIAMRole(ctx context.Context, client *Client, r *infra.Resource) error {
	roleName := getStringProp(r.Properties, "name", string(r.ID))

	_, err := client.IAM.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("delete IAM role %q: %w", roleName, err)
	}
	return nil
}
