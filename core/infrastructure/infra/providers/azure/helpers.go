//go:build with_azure
// +build with_azure

package azure

import (
	"context"
	"fmt"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

func createStorageAccount(ctx context.Context, p *Provider, r *infra.Resource) (*infra.Resource, error) {
	name := getStringProp(r.Properties, "name", string(r.ID))
	sku := getStringProp(r.Properties, "sku", "Standard_LRS")

	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":                    fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s", p.subscription, p.resourceGroup, name),
		"name":                  name,
		"primary_blob_endpoint": fmt.Sprintf("https://%s.blob.core.windows.net/", name),
	}
	return created, nil
}

func createVirtualMachine(ctx context.Context, p *Provider, r *infra.Resource) (*infra.Resource, error) {
	name := getStringProp(r.Properties, "name", string(r.ID))
	created := r.Clone()
	created.State = infra.ResourceStateCreated
	created.Outputs = map[string]string{
		"id":   fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s", p.subscription, p.resourceGroup, name),
		"name": name,
	}
	return created, nil
}

func getStringProp(props map[string]any, key, defaultVal string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
