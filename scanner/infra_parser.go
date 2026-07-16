package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// InfraResource represents a discovered infrastructure resource from annotations.
type InfraResource struct {
	Type         string            `json:"type"`
	ProviderName string            `json:"provider"`
	ID           string            `json:"id,omitempty"`
	Properties   map[string]any    `json:"properties"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Outputs      []string          `json:"outputs,omitempty"`
	File         string            `json:"-"`
	Line         int               `json:"-"`
}

// InfraResourceMap is the top-level structure written to infra_map.json.
type InfraResourceMap struct {
	Resources []InfraResource `json:"resources"`
	Stacks    []InfraStack    `json:"stacks,omitempty"`
}

// InfraStack is a named group of infrastructure resources.
type InfraStack struct {
	Name      string          `json:"name"`
	Resources []InfraResource `json:"resources"`
}

// ─── Annotation regex patterns ──────────────────────────────────────────

// @Resource(type: "aws_s3_bucket", id: "my-bucket")
// @Resource(type: "aws_s3_bucket")  — id defaults to filename+line
var infraResourceRe = regexp.MustCompile(`@Resource\(\s*type:\s*"([^"]+)"(?:\s*,\s*id:\s*"([^"]*)")?\s*\)`)

// @Provider(aws)
var infraProviderRe = regexp.MustCompile(`@Provider\(\s*(\w+)\s*\)`)

// @DependsOn(database, cache)
var infraDependsOnRe = regexp.MustCompile(`@DependsOn\(\s*([^)]+)\s*\)`)

// @Tags(env: "production", team: "platform")
var infraTagsRe = regexp.MustCompile(`@Tags\(\s*([^)]+)\s*\)`)

// @Output(name) — may appear multiple times
var infraOutputRe = regexp.MustCompile(`@Output\(\s*(\w+)\s*\)`)

// ─── Public functions ───────────────────────────────────────────────────

// ParseInfraFiles scans a directory for infrastructure resource annotations.
// Supported file types: .go, .py, .yaml, .yml
func ParseInfraFiles(dir string) ([]InfraResource, []AnnotationError) {
	var resources []InfraResource
	var errs []AnnotationError

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go":
			r, e := parseInfraGoFile(path)
			resources = append(resources, r...)
			errs = append(errs, e...)
		case ".py":
			r, e := parseInfraPyFile(path)
			resources = append(resources, r...)
			errs = append(errs, e...)
		case ".yaml", ".yml":
			r, e := parseInfraYAMLFile(path)
			resources = append(resources, r...)
			errs = append(errs, e...)
		}
		return nil
	})

	return resources, errs
}

// ─── Go file parser ─────────────────────────────────────────────────────

func parseInfraGoFile(path string) ([]InfraResource, []AnnotationError) {
	return parseInfraFile(path, "//")
}

// ─── Python file parser ─────────────────────────────────────────────────

func parseInfraPyFile(path string) ([]InfraResource, []AnnotationError) {
	return parseInfraFile(path, "#")
}

// ─── Generic file parser (shared by .go and .py) ────────────────────────

func parseInfraFile(path, commentPrefix string) ([]InfraResource, []AnnotationError) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []AnnotationError{{
			File: path, Line: 0,
			Message: fmt.Sprintf("cannot open file: %v", err),
		}}
	}

	lines := strings.Split(string(data), "\n")
	var resources []InfraResource
	var errs []AnnotationError

	var pending *InfraResource

	for lineNum, line := range lines {
		lineIdx := lineNum + 1 // 1-indexed
		trimmed := strings.TrimSpace(line)

		// Only process comment lines
		if !strings.HasPrefix(trimmed, commentPrefix) {
			flushPending(&resources, &pending)
			continue
		}

		// Remove comment prefix
		commentLine := strings.TrimSpace(strings.TrimPrefix(trimmed, commentPrefix))
		commentLine = strings.TrimSpace(commentLine)

		// Match @Resource
		if m := infraResourceRe.FindStringSubmatch(commentLine); m != nil {
			flushPending(&resources, &pending)

			resourceType := m[1]
			resourceID := m[2]
			if resourceID == "" {
				// Default id: filename_without_ext + _ + line
				base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				resourceID = fmt.Sprintf("%s_%d", base, lineIdx)
			}

			pending = &InfraResource{
				Type:         resourceType,
				ProviderName: "",
				ID:           resourceID,
				Properties:   make(map[string]any),
				DependsOn:    nil,
				Tags:         make(map[string]string),
				Outputs:      nil,
				File:         path,
				Line:         lineIdx,
			}
			continue
		}

		if pending == nil {
			continue
		}

		// Match @Provider
		if m := infraProviderRe.FindStringSubmatch(commentLine); m != nil {
			pending.ProviderName = m[1]
			continue
		}

		// Match @DependsOn
		if m := infraDependsOnRe.FindStringSubmatch(commentLine); m != nil {
			deps := strings.Split(m[1], ",")
			for _, dep := range deps {
				d := strings.TrimSpace(dep)
				if d != "" {
					pending.DependsOn = append(pending.DependsOn, d)
				}
			}
			continue
		}

		// Match @Tags
		if m := infraTagsRe.FindStringSubmatch(commentLine); m != nil {
			pending.Tags = parseTagList(m[1])
			continue
		}

		// Match @Output
		if m := infraOutputRe.FindStringSubmatch(commentLine); m != nil {
			pending.Outputs = append(pending.Outputs, m[1])
			continue
		}
	}

	// Flush any pending resource at EOF
	flushPending(&resources, &pending)

	return resources, errs
}

// ─── YAML file parser ───────────────────────────────────────────────────

func parseInfraYAMLFile(path string) ([]InfraResource, []AnnotationError) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []AnnotationError{{
			File: path, Line: 0,
			Message: fmt.Sprintf("cannot open file: %v", err),
		}}
	}

	content := string(data)
	var resources []InfraResource
	var errs []AnnotationError

	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		lineIdx := lineNum + 1
		trimmed := strings.TrimSpace(line)

		// Match # @Resource in YAML comments
		if strings.HasPrefix(trimmed, "#") {
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if m := infraResourceRe.FindStringSubmatch(comment); m != nil {
				resourceType := m[1]
				resourceID := m[2]
				if resourceID == "" {
					base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
					resourceID = fmt.Sprintf("%s_%d", base, lineIdx)
				}

				r := InfraResource{
					Type:         resourceType,
					ProviderName: "",
					ID:           resourceID,
					Properties:   make(map[string]any),
					DependsOn:    nil,
					Tags:         make(map[string]string),
					Outputs:      nil,
					File:         path,
					Line:         lineIdx,
				}

				// Scan following lines for @Provider, @Tags, @DependsOn, @Output
				for j := lineNum + 1; j < len(lines); j++ {
					nextLine := strings.TrimSpace(lines[j])
					if !strings.HasPrefix(nextLine, "#") {
						break
					}
					nextComment := strings.TrimSpace(strings.TrimPrefix(nextLine, "#"))

					if m := infraProviderRe.FindStringSubmatch(nextComment); m != nil {
						r.ProviderName = m[1]
					}
					if m := infraDependsOnRe.FindStringSubmatch(nextComment); m != nil {
						deps := strings.Split(m[1], ",")
						for _, dep := range deps {
							d := strings.TrimSpace(dep)
							if d != "" {
								r.DependsOn = append(r.DependsOn, d)
							}
						}
					}
					if m := infraTagsRe.FindStringSubmatch(nextComment); m != nil {
						r.Tags = parseTagList(m[1])
					}
					if m := infraOutputRe.FindStringSubmatch(nextComment); m != nil {
						r.Outputs = append(r.Outputs, m[1])
					}
				}

				resources = append(resources, r)
			}
		}
	}

	return resources, errs
}

// ─── Helpers ────────────────────────────────────────────────────────────

// flushPending appends a pending resource to the slice and resets it.
func flushPending(resources *[]InfraResource, pending **InfraResource) {
	if *pending == nil {
		return
	}
	if (*pending).ProviderName == "" {
		(*pending).ProviderName = "unknown"
	}
	*resources = append(*resources, **pending)
	*pending = nil
}

// parseTagList parses a @Tags argument like `env: "production", team: "platform"`
// into a map[string]string.
func parseTagList(raw string) map[string]string {
	tags := make(map[string]string)
	parts := splitTagPairs(raw)
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		val = strings.Trim(val, `"'`)
		if key != "" {
			tags[key] = val
		}
	}
	return tags
}

// splitTagPairs splits a tag string on commas, respecting quoted values.
func splitTagPairs(raw string) []string {
	var result []string
	depth := 0
	current := strings.Builder{}
	for _, ch := range raw {
		switch ch {
		case ',':
			if depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		case '"', '\'':
			current.WriteRune(ch)
			if depth == 0 {
				depth = 1
			} else {
				depth = 0
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
