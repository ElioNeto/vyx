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
	tsRouteRe    = regexp.MustCompile(`@Route\(\s*(\w+)\s+([^)]+)\)`)
	tsValidateRe = regexp.MustCompile(`@Validate\(\s*([^)]+)\s*\)`)
	tsAuthRe     = regexp.MustCompile(`@Auth\(roles:\s*\[([^\]]+)\]\)`)
	tsPageRe     = regexp.MustCompile(`@Page\(([^)]+)\)`)
)

// ParseTSFiles walks the given directory tree and extracts annotated routes
// from all *.ts and *.tsx files. workerID is applied to every route found.
func ParseTSFiles(dir, workerID string) ([]Route, []AnnotationError) {
	var routes []Route
	var errs []AnnotationError

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}
		r, e := parseTSFile(path, workerID)
		routes = append(routes, r...)
		errs = append(errs, e...)
		return nil
	})

	return routes, errs
}

func parseTSFile(path, workerID string) ([]Route, []AnnotationError) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []AnnotationError{{File: path, Line: 0, Message: fmt.Sprintf("cannot open file: %v", err)}}
	}
	defer f.Close()

	var routes []Route
	var errs []AnnotationError

	var pendingRoute, pendingValidate, pendingAuth, pendingPage string
	var routeLine int
	lineNum := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if !strings.HasPrefix(line, "//") {
			if pendingRoute != "" || pendingPage != "" {
				route, parseErr := buildTSRoute(path, routeLine, pendingRoute, pendingPage, pendingValidate, pendingAuth, workerID)
				if parseErr != nil {
					errs = append(errs, *parseErr)
				} else {
					routes = append(routes, route)
				}
				pendingRoute, pendingValidate, pendingAuth, pendingPage = "", "", "", ""
			}
			continue
		}

		comment := strings.TrimSpace(strings.TrimPrefix(line, "//"))

		switch {
		case tsRouteRe.MatchString(comment):
			pendingRoute = comment
			routeLine = lineNum
		case tsPageRe.MatchString(comment):
			pendingPage = comment
			routeLine = lineNum
		case tsValidateRe.MatchString(comment):
			pendingValidate = comment
		case tsAuthRe.MatchString(comment):
			pendingAuth = comment
		}
	}

	if pendingRoute != "" || pendingPage != "" {
		route, parseErr := buildTSRoute(path, routeLine, pendingRoute, pendingPage, pendingValidate, pendingAuth, workerID)
		if parseErr != nil {
			errs = append(errs, *parseErr)
		} else {
			routes = append(routes, route)
		}
	}

	return routes, errs
}

func buildTSRoute(file string, line int, routeAnnot, pageAnnot, validateAnnot, authAnnot, workerID string) (Route, *AnnotationError) {
	var method, path, routeType string

	if routeAnnot != "" {
		m := tsRouteRe.FindStringSubmatch(routeAnnot)
		if m == nil {
			return Route{}, &AnnotationError{File: file, Line: line, Message: "malformed @Route annotation"}
		}
		method = strings.ToUpper(strings.TrimSpace(m[1]))
		path = strings.TrimSpace(m[2])
		routeType = "api"
	} else if pageAnnot != "" {
		m := tsPageRe.FindStringSubmatch(pageAnnot)
		if m == nil {
			return Route{}, &AnnotationError{File: file, Line: line, Message: "malformed @Page annotation"}
		}
		method = "GET"
		path = strings.TrimSpace(m[1])
		routeType = "page"
	} else {
		return Route{}, &AnnotationError{File: file, Line: line, Message: "no @Route or @Page annotation found"}
	}

	validate := ""
	if validateAnnot != "" {
		vm := tsValidateRe.FindStringSubmatch(validateAnnot)
		if vm != nil {
			validate = strings.TrimSpace(vm[1])
		}
	}

	var roles []string
	if authAnnot != "" {
		am := tsAuthRe.FindStringSubmatch(authAnnot)
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
		Type:      routeType,
	}, nil
}
