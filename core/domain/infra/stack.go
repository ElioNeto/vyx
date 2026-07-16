package infra

// Stack is a named collection of resources that are managed together.
type Stack struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Resources   []*Resource  `json:"resources"`
	ProviderRefs []ProviderID `json:"provider_refs"`
}

// NewStack creates a new Stack with the given name.
func NewStack(name string) *Stack {
	return &Stack{
		Name:         name,
		Resources:    make([]*Resource, 0),
		ProviderRefs: make([]ProviderID, 0),
	}
}

// AddResource adds a resource to the stack.
func (s *Stack) AddResource(r *Resource) {
	s.Resources = append(s.Resources, r)
	// Ensure the provider is referenced.
	for _, p := range s.ProviderRefs {
		if p == r.ProviderName {
			return
		}
	}
	s.ProviderRefs = append(s.ProviderRefs, r.ProviderName)
}

// FindResource returns a resource by ID, or nil if not found.
func (s *Stack) FindResource(id ResourceID) *Resource {
	for _, r := range s.Resources {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// ResourceCount returns the number of resources in the stack.
func (s *Stack) ResourceCount() int {
	return len(s.Resources)
}

// TopologicalSort returns resources ordered by dependency (dependants after dependencies).
// Uses Kahn's algorithm for topological ordering.
func (s *Stack) TopologicalSort() ([]*Resource, error) {
	// Map resource IDs to indices.
	index := make(map[ResourceID]int, len(s.Resources))
	for i, r := range s.Resources {
		index[r.ID] = i
	}

	// Build adjacency list and in-degree count.
	inDegree := make(map[ResourceID]int, len(s.Resources))
	adj := make(map[ResourceID][]ResourceID, len(s.Resources))

	for _, r := range s.Resources {
		if _, ok := inDegree[r.ID]; !ok {
			inDegree[r.ID] = 0
		}
		for _, dep := range r.DependsOn {
			adj[dep] = append(adj[dep], r.ID)
			inDegree[r.ID]++
		}
	}

	// Queue resources with no dependencies.
	queue := make([]ResourceID, 0)
	for _, r := range s.Resources {
		if inDegree[r.ID] == 0 {
			queue = append(queue, r.ID)
		}
	}

	// Process queue.
	result := make([]*Resource, 0, len(s.Resources))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if res := s.FindResource(id); res != nil {
			result = append(result, res)
		}

		for _, neighbour := range adj[id] {
			inDegree[neighbour]--
			if inDegree[neighbour] == 0 {
				queue = append(queue, neighbour)
			}
		}
	}

	// Check for cycles.
	if len(result) != len(s.Resources) {
		return nil, &ErrCycleDetected{StackName: s.Name}
	}

	return result, nil
}

// TopologicalSortReverse returns resources in reverse dependency order (for destroy).
func (s *Stack) TopologicalSortReverse() ([]*Resource, error) {
	sorted, err := s.TopologicalSort()
	if err != nil {
		return nil, err
	}

	// Reverse the slice.
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}
	return sorted, nil
}
