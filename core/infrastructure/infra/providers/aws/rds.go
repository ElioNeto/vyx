//go:build with_aws
// +build with_aws

package aws

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

func planRDSInstance(desired, current *infra.Resource) (*infra.ResourceChange, error) {
	diff := make(map[string]infra.DiffValue)
	fields := []string{"engine", "engine_version", "instance_class", "allocated_storage", "multi_az"}
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

func createRDSInstance(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	engine := getStringProp(r.Properties, "engine", "postgres")
	engineVersion := getStringProp(r.Properties, "engine_version", "16.3")
	instanceClass := getStringProp(r.Properties, "instance_class", "db.r6g.large")
	storage := getIntProp(r.Properties, "allocated_storage", 100)
	dbName := getStringProp(r.Properties, "db_name", "mydb")
	username := getStringProp(r.Properties, "username", "dbadmin")

	_, err := client.RDS.CreateDBInstance(ctx, &rds.CreateDBInstanceInput{
		Engine:               aws.String(engine),
		EngineVersion:        aws.String(engineVersion),
		DBInstanceClass:      aws.String(instanceClass),
		AllocatedStorage:     aws.Int32(int32(storage)),
		DBInstanceIdentifier: aws.String(string(r.ID)),
		DBName:               aws.String(dbName),
		MasterUsername:       aws.String(username),
		MasterUserPassword:   aws.String("changeme123"), // should come from config
		MultiAZ:              aws.Bool(getBoolProp(r.Properties, "multi_az", false)),
		PubliclyAccessible:   aws.Bool(getBoolProp(r.Properties, "publicly_accessible", false)),
	})
	if err != nil {
		return nil, fmt.Errorf("create RDS instance %q: %w", r.ID, err)
	}

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":      string(r.ID),
		"engine":  engine,
		"version": engineVersion,
		"status":  "creating",
	}
	return created, nil
}

func readRDSInstance(ctx context.Context, client *Client, r *infra.Resource) (*infra.Resource, error) {
	result, err := client.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(string(r.ID)),
	})
	if err != nil {
		return nil, fmt.Errorf("read RDS instance %q: %w", r.ID, err)
	}

	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	if len(result.DBInstances) > 0 {
		db := result.DBInstances[0]
		existing.Outputs = map[string]string{
			"id":       aws.ToString(db.DBInstanceIdentifier),
			"endpoint": aws.ToString(db.Endpoint.Address),
			"port":     fmt.Sprintf("%d", aws.ToInt32(db.Endpoint.Port)),
			"status":   aws.ToString(db.DBInstanceStatus),
		}
	}
	return existing, nil
}

func updateRDSInstance(ctx context.Context, client *Client, desired, current *infra.Resource) (*infra.Resource, error) {
	_, err := client.RDS.ModifyDBInstance(ctx, &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(string(desired.ID)),
		DBInstanceClass:      aws.String(getStringProp(desired.Properties, "instance_class", "")),
		AllocatedStorage:     aws.Int32(int32(getIntProp(desired.Properties, "allocated_storage", 100))),
		ApplyImmediately:     aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("update RDS instance %q: %w", desired.ID, err)
	}

	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

func deleteRDSInstance(ctx context.Context, client *Client, r *infra.Resource) error {
	_, err := client.RDS.DeleteDBInstance(ctx, &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String(string(r.ID)),
		SkipFinalSnapshot:    aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("delete RDS instance %q: %w", r.ID, err)
	}
	return nil
}
