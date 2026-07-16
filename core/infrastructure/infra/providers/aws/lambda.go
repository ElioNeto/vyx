//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func planLambdaFunction(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"function_name", "runtime", "handler", "memory_size", "timeout"}
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

func createLambdaFunction(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	funcName := getStringProp(r.Properties, "function_name", string(r.ID))
	runtime := getStringProp(r.Properties, "runtime", "nodejs20.x")
	handler := getStringProp(r.Properties, "handler", "index.handler")
	roleARN := getStringProp(r.Properties, "role_arn", "")
	memory := int32(getIntProp(r.Properties, "memory_size", 128))
	timeout := int32(getIntProp(r.Properties, "timeout", 30))

	if roleARN == "" {
		return nil, fmt.Errorf("lambda function %q: role_arn is required", r.ID)
	}

	// Note: actual code would need to provide a zip file via S3 or local path.
	// For now, this is a scaffold that requires a pre-uploaded package.
	result, err := client.Lambda.CreateFunction(ctx, &lambda.CreateFunctionInput{
		FunctionName: aws.String(funcName),
		Runtime:      lambdatypes.Runtime(runtime),
		Handler:      aws.String(handler),
		Role:         aws.String(roleARN),
		MemorySize:   aws.Int32(memory),
		Timeout:      aws.Int32(timeout),
		Code:         &lambdatypes.FunctionCode{}, // must provide S3Bucket/S3Key or ZipFile
	})
	if err != nil {
		return nil, fmt.Errorf("create Lambda function %q: %w", funcName, err)
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"function_arn":  aws.ToString(result.FunctionArn),
		"function_name": aws.ToString(result.FunctionName),
		"version":       aws.ToString(result.Version),
	}
	return created, nil
}

func readLambdaFunction(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	funcName := getStringProp(r.Properties, "function_name", string(r.ID))

	result, err := client.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(funcName),
	})
	if err != nil {
		return nil, fmt.Errorf("read Lambda function %q: %w", funcName, err)
	}

	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	if result.Configuration != nil {
		existing.Outputs = map[string]string{
			"function_arn":  aws.ToString(result.Configuration.FunctionArn),
			"function_name": aws.ToString(result.Configuration.FunctionName),
			"version":       aws.ToString(result.Configuration.Version),
		}
	}
	return existing, nil
}

func updateLambdaFunction(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	funcName := getStringProp(desired.Properties, "function_name", string(desired.ID))

	_, err := client.Lambda.UpdateFunctionConfiguration(ctx, &lambda.UpdateFunctionConfigurationInput{
		FunctionName: aws.String(funcName),
		MemorySize:   aws.Int32(int32(getIntProp(desired.Properties, "memory_size", 128))),
		Timeout:      aws.Int32(int32(getIntProp(desired.Properties, "timeout", 30))),
	})
	if err != nil {
		return nil, fmt.Errorf("update Lambda function %q: %w", funcName, err)
	}

	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteLambdaFunction(ctx context.Context, client *Client, r *infra.Resource) error {
	funcName := getStringProp(r.Properties, "function_name", string(r.ID))
	_, err := client.Lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: aws.String(funcName),
	})
	if err != nil {
		return fmt.Errorf("delete Lambda function %q: %w", funcName, err)
	}
	return nil
}
