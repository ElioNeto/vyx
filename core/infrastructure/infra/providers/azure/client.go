//go:build with_azure
// +build with_azure

package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// Client wraps Azure SDK clients for resource management.
// This is a scaffold — real implementation requires Azure SDK modules.
//
// To add Azure SDK support:
//   cd core && go get github.com/Azure/azure-sdk-for-go/sdk/azidentity
//   cd core && go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources
//   cd core && go get github.com/Azure/azure-sdk-for-go/sdk/storage/azblob
//
type Client struct {
	SubscriptionID    string
	ResourceGroup     string
	Location          string
	credential        *azidentity.DefaultAzureCredential
}

// NewClient creates an Azure SDK client. Currently returns a scaffold
// that provides the structure but does not make real API calls.
func NewClient(config map[string]any) (*Client, error) {
	subID := ""
	if v, ok := config["subscription_id"].(string); ok {
		subID = v
	}
	rg := "vyx-rg"
	if v, ok := config["resource_group"].(string); ok && v != "" {
		rg = v
	}
	location := "eastus"
	if v, ok := config["location"].(string); ok && v != "" {
		location = v
	}

	// Attempt to create Azure credential (will fail without SDK)
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure: create credential (install Azure SDK modules): %w", err)
	}

	return &Client{
		SubscriptionID: subID,
		ResourceGroup:  rg,
		Location:       location,
		credential:     cred,
	}, nil
}
