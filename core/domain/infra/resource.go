// Package infra defines the domain entities and interfaces for infrastructure
// management (Infrastructure as Code). Zero external dependencies — only stdlib.
package infra

import "time"

// ResourceType identifies a type of cloud resource (e.g. "aws_s3_bucket", "gcp_compute_instance").
type ResourceType string

// ResourceID is a unique identifier within a stack.
type ResourceID string

// ProviderID identifies a cloud provider (e.g. "aws", "gcp", "azure").
type ProviderID string

// ResourceState represents the current provisioning state of a resource.
type ResourceState string

const (
	ResourceStatePending  ResourceState = "pending"
	ResourceStateCreating ResourceState = "creating"
	ResourceStateCreated  ResourceState = "created"
	ResourceStateUpdating ResourceState = "updating"
	ResourceStateDeleting ResourceState = "deleting"
	ResourceStateDeleted  ResourceState = "deleted"
	ResourceStateFailed   ResourceState = "failed"
)

// Resource is the central domain entity representing a single cloud resource.
type Resource struct {
	ID           ResourceID       `json:"id"`
	Type         ResourceType     `json:"type"`
	ProviderName ProviderID       `json:"provider"`
	Properties   map[string]any   `json:"properties"`
	Inputs       map[string]any   `json:"inputs"`
	Outputs      map[string]string `json:"outputs"`
	DependsOn    []ResourceID     `json:"depends_on"`
	Tags         map[string]string `json:"tags"`
	Metadata     ResourceMetadata `json:"metadata"`
	State        ResourceState    `json:"state"`
}

// ResourceMetadata holds source-location metadata for auditability.
type ResourceMetadata struct {
	SourceFile  string `json:"source_file"`
	SourceLine  int    `json:"source_line"`
	Description string `json:"description"`
}

// NewResource creates a new Resource with sensible defaults.
func NewResource(id ResourceID, typ ResourceType, provider ProviderID) *Resource {
	return &Resource{
		ID:           id,
		Type:         typ,
		ProviderName: provider,
		Properties:   make(map[string]any),
		Inputs:       make(map[string]any),
		Outputs:      make(map[string]string),
		DependsOn:    nil,
		Tags:         make(map[string]string),
		State:        ResourceStatePending,
		Metadata: ResourceMetadata{
			SourceFile: "",
			SourceLine: 0,
		},
	}
}

// IsManaged returns true when the resource exists in the cloud (was created or updated).
func (r *Resource) IsManaged() bool {
	return r.State == ResourceStateCreated || r.State == ResourceStateUpdating
}

// NeedsCreation returns true when the resource has never been provisioned.
func (r *Resource) NeedsCreation() bool {
	return r.State == ResourceStatePending || r.State == ResourceStateFailed
}

// Clone returns a deep copy of the resource.
func (r *Resource) Clone() *Resource {
	clone := &Resource{
		ID:           r.ID,
		Type:         r.Type,
		ProviderName: r.ProviderName,
		State:        r.State,
		Metadata:     r.Metadata,
	}

	if r.Properties != nil {
		clone.Properties = make(map[string]any, len(r.Properties))
		for k, v := range r.Properties {
			clone.Properties[k] = v
		}
	}
	if r.Inputs != nil {
		clone.Inputs = make(map[string]any, len(r.Inputs))
		for k, v := range r.Inputs {
			clone.Inputs[k] = v
		}
	}
	if r.Outputs != nil {
		clone.Outputs = make(map[string]string, len(r.Outputs))
		for k, v := range r.Outputs {
			clone.Outputs[k] = v
		}
	}
	if r.DependsOn != nil {
		clone.DependsOn = make([]ResourceID, len(r.DependsOn))
		copy(clone.DependsOn, r.DependsOn)
	}
	if r.Tags != nil {
		clone.Tags = make(map[string]string, len(r.Tags))
		for k, v := range r.Tags {
			clone.Tags[k] = v
		}
	}

	return clone
}

// ResourceOutput holds a named output value from a provisioned resource.
type ResourceOutput struct {
	ResourceID ResourceID `json:"resource_id"`
	Name       string     `json:"name"`
	Value      string     `json:"value"`
	Sensitive  bool       `json:"sensitive"`
}

// ResourceOutputs is a collection of outputs grouped by resource ID.
type ResourceOutputs map[ResourceID]map[string]string

// IsZero returns true when the ResourceOutputs map is empty.
func (ro ResourceOutputs) IsZero() bool {
	for _, outs := range ro {
		if len(outs) > 0 {
			return false
		}
	}
	return true
}

// Now is a time source for testing. In production it delegates to time.Now.
var Now = time.Now
