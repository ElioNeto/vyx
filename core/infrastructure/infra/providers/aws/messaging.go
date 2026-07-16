//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"strings"
)

// ─── SQS Queue ──────────────────────────────────────────────────────────

func planSQSQueue(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"name", "fifo_queue", "visibility_timeout", "delay_seconds", "message_retention"}
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

func createSQSQueue(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	queueName := getStringProp(r.Properties, "name", string(r.ID))
	isFIFO := getBoolProp(r.Properties, "fifo_queue", false)

	if isFIFO && !strings.HasSuffix(queueName, ".fifo") {
		queueName += ".fifo"
	}

	input := &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"VisibilityTimeout":     fmt.Sprintf("%d", getIntProp(r.Properties, "visibility_timeout", 30)),
			"DelaySeconds":          fmt.Sprintf("%d", getIntProp(r.Properties, "delay_seconds", 0)),
			"MessageRetentionPeriod": fmt.Sprintf("%d", getIntProp(r.Properties, "message_retention", 345600)),
		},
	}
	if isFIFO {
		input.Attributes["FifoQueue"] = "true"
	}

	result, err := client.SQS.CreateQueue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create SQS queue %q: %w", queueName, err)
	}

	getAttrs, err := client.SQS.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: result.QueueUrl,
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	})
	queueARN := ""
	if err == nil {
		queueARN = getAttrs.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{"arn": queueARN, "url": aws.ToString(result.QueueUrl)}
	return created, nil
}

func readSQSQueue(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	queueURL := r.Outputs["url"]
	if queueURL == "" {
		return nil, fmt.Errorf("read SQS queue %q: no queue URL in state", r.ID)
	}
	_, err := client.SQS.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
	})
	if err != nil {
		return nil, fmt.Errorf("read SQS queue %q: %w", r.ID, err)
	}
	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	return existing, nil
}

func updateSQSQueue(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	queueURL := current.Outputs["url"]
	if queueURL == "" {
		return nil, fmt.Errorf("update SQS queue %q: no queue URL", desired.ID)
	}
	attrs := map[string]string{
		"VisibilityTimeout":     fmt.Sprintf("%d", getIntProp(desired.Properties, "visibility_timeout", 30)),
		"DelaySeconds":          fmt.Sprintf("%d", getIntProp(desired.Properties, "delay_seconds", 0)),
		"MessageRetentionPeriod": fmt.Sprintf("%d", getIntProp(desired.Properties, "message_retention", 345600)),
	}
	_, err := client.SQS.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl:   aws.String(queueURL),
		Attributes: attrs,
	})
	if err != nil {
		return nil, fmt.Errorf("update SQS queue %q: %w", desired.ID, err)
	}
	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteSQSQueue(ctx context.Context, client *Client, r *infra.Resource) error {
	queueURL := r.Outputs["url"]
	if queueURL == "" {
		return fmt.Errorf("delete SQS queue %q: no queue URL", r.ID)
	}
	_, err := client.SQS.DeleteQueue(ctx, &sqs.DeleteQueueInput{QueueUrl: aws.String(queueURL)})
	if err != nil {
		return fmt.Errorf("delete SQS queue %q: %w", r.ID, err)
	}
	return nil
}

// ─── SNS Topic ───────────────────────────────────────────────────────────

func planSNSTopic(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	if !valuesEqual(desired.Properties["name"], current.Properties["name"]) {
		diff["name"] = infra.DiffValue{Old: current.Properties["name"], New: desired.Properties["name"]}
	}
	if len(diff) == 0 {
		return &infra.ResourceChange{ResourceID: desired.ID, ChangeType: infra.ChangeNoop, ResourceType: desired.Type, ProviderName: "aws"}, nil
	}
	return &infra.ResourceChange{ResourceID: desired.ID, ChangeType: infra.ChangeUpdate, ResourceType: desired.Type, ProviderName: "aws", Diff: diff}, nil
}

func createSNSTopic(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	topicName := getStringProp(r.Properties, "name", string(r.ID))
	isFIFO := getBoolProp(r.Properties, "fifo_topic", false)

	input := &sns.CreateTopicInput{Name: aws.String(topicName)}
	if isFIFO {
		input.Attributes = map[string]string{"FifoTopic": "true"}
		if !strings.HasSuffix(topicName, ".fifo") {
			input.Name = aws.String(topicName + ".fifo")
		}
	}

	result, err := client.SNS.CreateTopic(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create SNS topic %q: %w", topicName, err)
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{"arn": aws.ToString(result.TopicArn)}
	return created, nil
}

func readSNSTopic(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	topicARN := r.Outputs["arn"]
	if topicARN == "" {
		return nil, fmt.Errorf("read SNS topic %q: no ARN in state", r.ID)
	}
	_, err := client.SNS.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: aws.String(topicARN)})
	if err != nil {
		return nil, fmt.Errorf("read SNS topic %q: %w", r.ID, err)
	}
	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	return existing, nil
}

func updateSNSTopic(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteSNSTopic(ctx context.Context, client *Client, r *infra.Resource) error {
	topicARN := r.Outputs["arn"]
	if topicARN == "" {
		return fmt.Errorf("delete SNS topic %q: no ARN", r.ID)
	}
	_, err := client.SNS.DeleteTopic(ctx, &sns.DeleteTopicInput{TopicArn: aws.String(topicARN)})
	if err != nil {
		return fmt.Errorf("delete SNS topic %q: %w", r.ID, err)
	}
	return nil
}
