package scanner

// Route represents a discovered route from annotation parsing.
type Route struct {
	Path      string   `json:"path"`
	Method    string   `json:"method"`
	WorkerID  string   `json:"worker_id"`
	AuthRoles []string `json:"auth_roles"`
	Validate  string   `json:"validate"`
	Type      string   `json:"type"` // "api" or "page"
}

// AnnotationError holds information about a malformed annotation.
type AnnotationError struct {
	File    string
	Line    int
	Message string
}

func (e *AnnotationError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
}
