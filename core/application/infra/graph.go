package infra

import (
	"fmt"
	"strings"

	"github.com/ElioNeto/vyx/core/domain/infra"
)

// GraphFormat represents the output format for dependency graphs.
type GraphFormat string

const (
	GraphFormatMermaid GraphFormat = "mermaid"
	GraphFormatDot     GraphFormat = "dot"
)

// GraphGenerator produces visual representations of resource dependencies.
type GraphGenerator struct{}

// NewGraphGenerator creates a new GraphGenerator.
func NewGraphGenerator() *GraphGenerator {
	return &GraphGenerator{}
}

// Generate produces a graph in the specified format.
func (g *GraphGenerator) Generate(stack *infra.Stack, format GraphFormat) (string, error) {
	switch format {
	case GraphFormatMermaid:
		return g.generateMermaid(stack)
	case GraphFormatDot:
		return g.generateDot(stack)
	default:
		return "", fmt.Errorf("unsupported graph format: %q (use 'mermaid' or 'dot')", format)
	}
}

// generateMermaid produces a Mermaid flow diagram.
func (g *GraphGenerator) generateMermaid(stack *infra.Stack) (string, error) {
	var b strings.Builder

	b.WriteString("graph TD\n")
	b.WriteString(fmt.Sprintf("  title[Infrastructure: %s]\n", stack.Name))

	// Subgraph per provider
	providerGroups := make(map[infra.ProviderID][]*infra.Resource)
	for _, r := range stack.Resources {
		providerGroups[r.ProviderName] = append(providerGroups[r.ProviderName], r)
	}

	for provider, resources := range providerGroups {
		cleanProvider := sanitizeID(string(provider))
		b.WriteString(fmt.Sprintf("  subgraph %s [Provider: %s]\n", cleanProvider, provider))
		for _, r := range resources {
			nodeID := sanitizeID(string(r.ID))
			label := fmt.Sprintf("%s\\n(%s)", r.ID, r.Type)
			b.WriteString(fmt.Sprintf("    %s[%s]\n", nodeID, label))
		}
		b.WriteString("  end\n")
	}

	// Dependencies
	for _, r := range stack.Resources {
		for _, dep := range r.DependsOn {
			from := sanitizeID(string(dep))
			to := sanitizeID(string(r.ID))
			b.WriteString(fmt.Sprintf("  %s --> %s\n", from, to))
		}
	}

	return b.String(), nil
}

// generateDot produces a Graphviz DOT diagram.
func (g *GraphGenerator) generateDot(stack *infra.Stack) (string, error) {
	var b strings.Builder

	b.WriteString("digraph infra {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString(fmt.Sprintf("  label=\"Infrastructure: %s\";\n", stack.Name))
	b.WriteString("  fontsize=14;\n")
	b.WriteString("  node [style=filled, fillcolor=lightyellow, shape=box];\n\n")

	// Provider subgraphs
	providerGroups := make(map[infra.ProviderID][]*infra.Resource)
	for _, r := range stack.Resources {
		providerGroups[r.ProviderName] = append(providerGroups[r.ProviderName], r)
	}

	for provider, resources := range providerGroups {
		b.WriteString(fmt.Sprintf("  subgraph cluster_%s {\n", sanitizeID(string(provider))))
		b.WriteString(fmt.Sprintf("    label=\"Provider: %s\";\n", provider))
		b.WriteString("    style=filled;\n")
		b.WriteString("    fillcolor=lightgray;\n")
		for _, r := range resources {
			nodeID := sanitizeID(string(r.ID))
			b.WriteString(fmt.Sprintf("    %s [label=\"%s (%s)\"];\n", nodeID, r.ID, r.Type))
		}
		b.WriteString("  }\n\n")
	}

	// Dependencies
	for _, r := range stack.Resources {
		for _, dep := range r.DependsOn {
			from := sanitizeID(string(dep))
			to := sanitizeID(string(r.ID))
			b.WriteString(fmt.Sprintf("  %s -> %s;\n", from, to))
		}
	}

	b.WriteString("}\n")
	return b.String(), nil
}

// sanitizeID replaces characters that are invalid in graph node IDs.
func sanitizeID(id string) string {
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, ":", "_")
	id = strings.ReplaceAll(id, "/", "_")
	if id != "" && id[0] >= '0' && id[0] <= '9' {
		id = "n_" + id
	}
	return id
}
