package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	goRouteRe    = regexp.MustCompile(`@Route\(\s*(\w+)\s+([^)]+)\)`)
	goValidateRe = regexp.MustCompile(`@Validate\(\s*([^)]+)\s*\)`)
	goAuthRe     = regexp.MustCompile(`@Auth\(roles:\s*\[([^\]]+)\]\)`)
)

// ParseGoFiles walks the given directory tree and extracts annotated routes
// from all *.go files. workerID is applied to every route found.
func ParseGoFiles(dir, workerID string) ([]Route, []AnnotationError) {
	var routes []Route
	var errs []AnnotationError

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		r, e := parseGoFile(path, workerID)
		routes = append(routes, r...)
		errs = append(errs, e...)
		return nil
	})

	return routes, errs
}

func parseGoFile(path, workerID string) ([]Route, []AnnotationError) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []AnnotationError{{File: path, Line: 0, Message: fmt.Sprintf("cannot open file: %v", err)}}
	}
	defer f.Close()

	var routes []Route
	var errs []AnnotationError

	var pendingRoute, pendingValidate, pendingAuth string
	var routeLine int
	lineNum := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if !strings.HasPrefix(line, "//") {
			// Flush pending annotation block when a non-comment line is reached.
			if pendingRoute != "" {
				route, parseErr := buildRoute(path, routeLine, pendingRoute, pendingValidate, pendingAuth, workerID)
				if parseErr != nil {
					errs = append(errs, *parseErr)
				} else {
					routes = append(routes, route)
				}
				pendingRoute, pendingValidate, pendingAuth = "", "", ""
			}
			continue
		}

		comment := strings.TrimPrefix(line, "//")
		comment = strings.TrimSpace(comment)

		switch {
		case goRouteRe.MatchString(comment):
			pendingRoute = comment
			routeLine = lineNum
		case goValidateRe.MatchString(comment):
			pendingValidate = comment
		case goAuthRe.MatchString(comment):
			pendingAuth = comment
		}
	}

	// Handle annotations at end-of-file.
	if pendingRoute != "" {
		route, parseErr := buildRoute(path, routeLine, pendingRoute, pendingValidate, pendingAuth, workerID)
		if parseErr != nil {
			errs = append(errs, *parseErr)
		} else {
			routes = append(routes, route)
		}
	}

	return routes, errs
}

func buildRoute(file string, line int, routeAnnot, validateAnnot, authAnnot, workerID string) (Route, *AnnotationError) {
	m := goRouteRe.FindStringSubmatch(routeAnnot)
	if m == nil {
		return Route{}, &AnnotationError{File: file, Line: line, Message: "malformed @Route annotation"}
	}

	method := strings.ToUpper(strings.TrimSpace(m[1]))
	path := strings.TrimSpace(m[2])

	validate := ""
	if validateAnnot != "" {
		vm := goValidateRe.FindStringSubmatch(validateAnnot)
		if vm != nil {
			validate = strings.TrimSpace(vm[1])
		}
	}

	var roles []string
	if authAnnot != "" {
		am := goAuthRe.FindStringSubmatch(authAnnot)
		if am != nil {
			for _, r := range strings.Split(am[1], ",") {
				role := strings.Trim(strings.TrimSpace(r), `"`)
				if role != "" {
					roles = append(roles, role)
				}
			}
		}
	}

	return Route{
		Path:      path,
		Method:    method,
		WorkerID:  workerID,
		AuthRoles: roles,
		Validate:  validate,
		Type:      "api",
	}, nil
}
