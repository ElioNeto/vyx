//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

// ─── API Gateway REST API ───────────────────────────────────────────────

func planAPIGateway(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"name", "description", "version"}
	for _, f := range fields {
		if !valuesEqual(desired.Properties[f], current.Properties[f]) {
			diff[f] = infra.DiffValue{Old: current.Properties[f], New: desired.Properties[f]}
		}
	}
	if len(diff) == 0 {
		return &infra.ResourceChange{ResourceID: desired.ID, ChangeType: infra.ChangeNoop, ResourceType: desired.Type, ProviderName: "aws"}, nil
	}
	return &infra.ResourceChange{ResourceID: desired.ID, ChangeType: infra.ChangeUpdate, ResourceType: desired.Type, ProviderName: "aws", Diff: diff}, nil
}

func createAPIGateway(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	apiName := getStringProp(r.Properties, "name", string(r.ID))
	desc := getStringProp(r.Properties, "description", "")

	result, err := client.APIGateway.CreateRestApi(ctx, &apigateway.CreateRestApiInput{
		Name:        aws.String(apiName),
		Description: aws.String(desc),
		Version:     aws.String(getStringProp(r.Properties, "version", "v1")),
	})
	if err != nil {
		return nil, fmt.Errorf("create API Gateway %q: %w", apiName, err)
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":               aws.ToString(result.Id),
		"arn":              fmt.Sprintf("arn:aws:apigateway:%s::/restapis/%s", client.Region(), aws.ToString(result.Id)),
		"root_resource_id": aws.ToString(result.RootResourceId),
	}
	return created, nil
}

func readAPIGateway(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	apiID := r.Outputs["id"]
	if apiID == "" {
		return nil, fmt.Errorf("read API Gateway %q: no API ID in state", r.ID)
	}
	result, err := client.APIGateway.GetRestApi(ctx, &apigateway.GetRestApiInput{
		RestApiId: aws.String(apiID),
	})
	if err != nil {
		return nil, fmt.Errorf("read API Gateway %q: %w", r.ID, err)
	}
	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	existing.Outputs = map[string]string{
		"id":  aws.ToString(result.Id),
		"arn": fmt.Sprintf("arn:aws:apigateway:%s::/restapis/%s", client.Region(), aws.ToString(result.Id)),
	}
	return existing, nil
}

func updateAPIGateway(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	apiID := current.Outputs["id"]
	if apiID == "" {
		return nil, fmt.Errorf("update API Gateway %q: no API ID", desired.ID)
	}
	_, err := client.APIGateway.UpdateRestApi(ctx, &apigateway.UpdateRestApiInput{
		RestApiId: aws.String(apiID),
		PatchOperations: []apigatewaytypes.PatchOperation{
			{
				Op:    apigatewaytypes.OpReplace,
				Path:  aws.String("/description"),
				Value: aws.String(getStringProp(desired.Properties, "description", "")),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("update API Gateway %q: %w", desired.ID, err)
	}
	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteAPIGateway(ctx context.Context, client *Client, r *infra.Resource) error {
	apiID := r.Outputs["id"]
	if apiID == "" {
		return fmt.Errorf("delete API Gateway %q: no API ID", r.ID)
	}
	_, err := client.APIGateway.DeleteRestApi(ctx, &apigateway.DeleteRestApiInput{
		RestApiId: aws.String(apiID),
	})
	if err != nil {
		return fmt.Errorf("delete API Gateway %q: %w", r.ID, err)
	}
	return nil
}
