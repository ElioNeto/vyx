# Azure Provider — Implementation Guide

## Current Status

The Azure provider scaffold is implemented but is **not yet functional** for real cloud operations.

## What's Implemented

- Provider interface (Validate, Plan, Create, Read, Update, Delete, Capabilities)
- 3 resource types: Storage Account, Virtual Machine, SQL Database
- Build tag isolation: `//go:build with_azure`

## What's Missing

1. **Azure SDK integration** — uses stubs for Read/Delete
2. **Authentication** — needs Azure SDK's credential chain
3. **Resource operations** — actual API calls via Azure SDK
4. **Tests** — no test files yet

## Implementation Plan

### Step 1: Add Azure SDK Dependencies

```bash
cd core
go get github.com/Azure/azure-sdk-for-go/sdk/storage/azblob
go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute
go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity
```

### Step 2: Create Azure Client

```go
package azure

import (
    "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
    "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type Client struct {
    SubscriptionID string
    ResourceGroup  string
    BlobClient     *azblob.Client
    // ... more clients
}

func NewClient(config map[string]any) (*Client, error) {
    cred, err := azidentity.NewDefaultAzureCredential(nil)
    if err != nil {
        return nil, fmt.Errorf("azure auth: %w", err)
    }
    // ... create clients
}
```

### Step 3: Implement Resource Operations

Replace the stub `Read()` and `Delete()` with real Azure SDK calls.

### Step 4: Tests

Add unit tests and integration tests with Azure emulators.
