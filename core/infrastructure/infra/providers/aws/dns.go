//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// ─── Route53 Hosted Zone ────────────────────────────────────────────────

func planRoute53Zone(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	if !valuesEqual(desired.Properties["name"], current.Properties["name"]) {
		diff["name"] = infra.DiffValue{Old: current.Properties["name"], New: desired.Properties["name"]}
	}
	if len(diff) == 0 {
		return &infra.ResourceChange{ResourceID: desired.ID, ChangeType: infra.ChangeNoop, ResourceType: desired.Type, ProviderName: "aws"}, nil
	}
	return &infra.ResourceChange{ResourceID: desired.ID, ChangeType: infra.ChangeUpdate, ResourceType: desired.Type, ProviderName: "aws", Diff: diff}, nil
}

func createRoute53Zone(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	zoneName := getStringProp(r.Properties, "name", string(r.ID))
	if !strings.HasSuffix(zoneName, ".") {
		zoneName += "."
	}

	result, err := client.Route53.CreateHostedZone(ctx, &route53.CreateHostedZoneInput{
		Name:            aws.String(zoneName),
		CallerReference: aws.String(string(r.ID) + "-" + fmt.Sprintf("%d", time.Now().Unix())),
	})
	if err != nil {
		return nil, fmt.Errorf("create Route53 zone %q: %w", zoneName, err)
	}

	nsList := make([]string, 0)
	if result.DelegationSet != nil {
		nsList = append(nsList, result.DelegationSet.NameServers...)
	}

	zoneID := aws.ToString(result.HostedZone.Id)
	zoneID = strings.TrimPrefix(zoneID, "/hostedzone/")

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":           zoneID,
		"arn":          fmt.Sprintf("arn:aws:route53:::hostedzone/%s", zoneID),
		"name_servers": strings.Join(nsList, ","),
	}
	return created, nil
}

func readRoute53Zone(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	zoneID := r.Outputs["id"]
	if zoneID == "" {
		return nil, fmt.Errorf("read Route53 zone %q: no zone ID in state", r.ID)
	}

	result, err := client.Route53.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: aws.String(zoneID),
	})
	if err != nil {
		return nil, fmt.Errorf("read Route53 zone %q: %w", r.ID, err)
	}

	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	if result.HostedZone != nil {
		existing.Outputs = map[string]string{
			"id":  zoneID,
			"arn": fmt.Sprintf("arn:aws:route53:::hostedzone/%s", zoneID),
		}
	}
	return existing, nil
}

func updateRoute53Zone(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteRoute53Zone(ctx context.Context, client *Client, r *infra.Resource) error {
	zoneID := r.Outputs["id"]
	if zoneID == "" {
		return fmt.Errorf("delete Route53 zone %q: no zone ID", r.ID)
	}
	_, err := client.Route53.DeleteHostedZone(ctx, &route53.DeleteHostedZoneInput{
		Id: aws.String(zoneID),
	})
	if err != nil {
		return fmt.Errorf("delete Route53 zone %q: %w", r.ID, err)
	}
	return nil
}
