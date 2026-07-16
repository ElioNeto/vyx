//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
)

// ─── ElastiCache Cluster ────────────────────────────────────────────────

func planElastiCacheCluster(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"cluster_id", "engine", "node_type", "num_cache_nodes", "engine_version"}
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

func createElastiCacheCluster(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	clusterID := getStringProp(r.Properties, "cluster_id", string(r.ID))
	engine := getStringProp(r.Properties, "engine", "redis")
	nodeType := getStringProp(r.Properties, "node_type", "cache.t3.micro")
	numNodes := int32(getIntProp(r.Properties, "num_cache_nodes", 1))

	input := &elasticache.CreateCacheClusterInput{
		CacheClusterId: aws.String(clusterID),
		Engine:         aws.String(engine),
		CacheNodeType:  aws.String(nodeType),
		NumCacheNodes:  aws.Int32(numNodes),
	}
	if ev := getStringProp(r.Properties, "engine_version", ""); ev != "" {
		input.EngineVersion = aws.String(ev)
	}

	result, err := client.ElastiCache.CreateCacheCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create ElastiCache cluster %q: %w", clusterID, err)
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":  aws.ToString(result.CacheCluster.CacheClusterId),
		"arn": aws.ToString(result.CacheCluster.ARN),
	}
	if result.CacheCluster.CacheNodes != nil && len(result.CacheCluster.CacheNodes) > 0 {
		node := result.CacheCluster.CacheNodes[0]
		created.Outputs["configuration_endpoint"] = fmt.Sprintf("%s:%d",
			aws.ToString(node.Endpoint.Address), node.Endpoint.Port)
		created.Outputs["port"] = fmt.Sprintf("%d", node.Endpoint.Port)
	}
	return created, nil
}

func readElastiCacheCluster(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	clusterID := getStringProp(r.Properties, "cluster_id", string(r.ID))
	result, err := client.ElastiCache.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
		CacheClusterId: aws.String(clusterID),
	})
	if err != nil {
		return nil, fmt.Errorf("read ElastiCache cluster %q: %w", clusterID, err)
	}
	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	if len(result.CacheClusters) > 0 {
		cc := result.CacheClusters[0]
		existing.Outputs = map[string]string{
			"id":  aws.ToString(cc.CacheClusterId),
			"arn": aws.ToString(cc.ARN),
		}
	}
	return existing, nil
}

func updateElastiCacheCluster(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	clusterID := getStringProp(desired.Properties, "cluster_id", string(desired.ID))
	_, err := client.ElastiCache.ModifyCacheCluster(ctx, &elasticache.ModifyCacheClusterInput{
		CacheClusterId: aws.String(clusterID),
		CacheNodeType:  aws.String(getStringProp(desired.Properties, "node_type", "")),
		ApplyImmediately: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("update ElastiCache cluster %q: %w", clusterID, err)
	}
	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteElastiCacheCluster(ctx context.Context, client *Client, r *infra.Resource) error {
	clusterID := getStringProp(r.Properties, "cluster_id", string(r.ID))
	_, err := client.ElastiCache.DeleteCacheCluster(ctx, &elasticache.DeleteCacheClusterInput{
		CacheClusterId: aws.String(clusterID),
	})
	if err != nil {
		return fmt.Errorf("delete ElastiCache cluster %q: %w", clusterID, err)
	}
	return nil
}
