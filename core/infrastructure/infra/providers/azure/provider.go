// Package azure implements the Microsoft Azure cloud provider for the vyx IaC module.
//
// This provider is compiled conditionally with the build tag "with_azure".
// To include it:
//
//	go build -tags with_azure ./cmd/vyx
//
//go:build with_azure
// +build with_azure

package azure

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// ProviderID is the unique identifier for the Azure provider.
const ProviderID infra.ProviderID = "azure"

// Provider implements the infra.Provider interface for Microsoft Azure.
type Provider struct {
	id          infra.ProviderID
	subscription string
	resourceGroup string
}

// New creates a new Azure provider.
func New(config map[string]any) (*Provider, error) {
	sub := ""
	if v, ok := config["subscription"].(string); ok {
		sub = v
	}
	rg := "vyx-rg"
	if v, ok := config["resource_group"].(string); ok && v != "" {
		rg = v
	}

	return &Provider{
		id:           ProviderID,
		subscription: sub,
		resourceGroup: rg,
	}, nil
}

// ID returns the provider identifier.
func (p *Provider) ID() infra.ProviderID { return p.id }

// Validate checks that the resource has required properties.
func (p *Provider) Validate(ctx context.Context, r *infra.Resource) error {
	if r.Properties == nil {
		return &infra.ErrValidation{ResourceID: r.ID, Message: "properties is required"}
	}
	return nil
}

// Plan determines what changes are needed.
func (p *Provider) Plan(ctx context.Context, desired, current *infra.Resource) (*infra.ResourceChange, error) {
	if current == nil || current.State == infra.ResourceStateDeleted || current.State == infra.ResourceStatePending {
		return &infra.ResourceChange{
			ResourceID: desired.ID, ChangeType: infra.ChangeCreate,
			ResourceType: desired.Type, ProviderName: p.id,
		}, nil
	}
	// Compare properties to detect changes
	diff := make(map[string]infra.DiffValue)
	for k, newVal := range desired.Properties {
		oldVal := current.Properties[k]
		if !valuesEqual(oldVal, newVal) {
			diff[k] = infra.DiffValue{Old: oldVal, New: newVal}
		}
	}
	if len(diff) == 0 {
		return &infra.ResourceChange{
			ResourceID: desired.ID, ChangeType: infra.ChangeNoop,
			ResourceType: desired.Type, ProviderName: p.id,
		}, nil
	}
	return &infra.ResourceChange{
		ResourceID: desired.ID, ChangeType: infra.ChangeUpdate,
		ResourceType: desired.Type, ProviderName: p.id, Diff: diff,
	}, nil
}

// Create provisions a new Azure resource.
func (p *Provider) Create(ctx context.Context, r *infra.Resource) (*infra.Resource, error) {
	switch r.Type {
	case "azure_storage_account":
		return createStorageAccount(ctx, p, r)
	case "azure_virtual_machine":
		return createVirtualMachine(ctx, p, r)
	default:
		return nil, &infra.ErrValidation{ResourceID: r.ID, Message: fmt.Sprintf("unsupported Azure resource type: %s", r.Type)}
	}
}

// Read queries the current state of an Azure resource.
func (p *Provider) Read(ctx context.Context, r *infra.Resource) (*infra.Resource, error) {
	// Placeholder — real implementation uses Azure SDK
	existing := r.Clone()
	existing.State = infra.ResourceStateCreated
	return existing, nil
}

// Update modifies an Azure resource to match desired state.
func (p *Provider) Update(ctx context.Context, desired, current *infra.Resource) (*infra.Resource, error) {
	updated := desired.Clone()
	updated.State = infra.ResourceStateCreated
	updated.Outputs = current.Outputs
	return updated, nil
}

// Delete destroys an Azure resource.
func (p *Provider) Delete(ctx context.Context, r *infra.Resource) error {
	// Placeholder — real implementation calls Azure SDK
	return nil
}

// Capabilities returns the resource types this provider can manage.
func (p *Provider) Capabilities() []infra.ResourceCapability {
	return []infra.ResourceCapability{
		{
			Type:        "azure_storage_account",
			Description: "Azure Storage Account for blobs, files, queues, tables",
			InputSchema: map[string]infra.SchemaField{
				"name":     {Type: "string", Required: true, Description: "Storage account name (3-24 chars, lowercase)"},
				"sku":      {Type: "string", Required: false, Default: "Standard_LRS", Description: "SKU (Standard_LRS, Standard_GRS, etc.)"},
				"kind":     {Type: "string", Required: false, Default: "StorageV2", Description: "Storage kind"},
				"location": {Type: "string", Required: false, Description: "Azure region"},
			},
			OutputFields: []string{"id", "name", "primary_blob_endpoint", "primary_connection_string"},
		},
		{
			Type:        "azure_virtual_machine",
			Description: "Azure Virtual Machine",
			InputSchema: map[string]infra.SchemaField{
				"name":       {Type: "string", Required: true, Description: "VM name"},
				"size":       {Type: "string", Required: true, Description: "VM size (e.g. Standard_B2s)"},
				"image":      {Type: "string", Required: true, Description: "OS image URN"},
				"admin_user": {Type: "string", Required: true, Description: "Admin username"},
				"location":   {Type: "string", Required: false, Description: "Azure region"},
			},
			OutputFields: []string{"id", "name", "public_ip", "private_ip"},
		},
		{
			Type:        "azure_sql_database",
			Description: "Azure SQL Database",
			InputSchema: map[string]infra.SchemaField{
				"name":     {Type: "string", Required: true, Description: "Database name"},
				"tier":     {Type: "string", Required: false, Default: "Basic", Description: "Service tier"},
				"location": {Type: "string", Required: false, Description: "Azure region"},
			},
			OutputFields: []string{"id", "name", "server_fqdn"},
		},
	}
}
