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
	pyRouteRe    = regexp.MustCompile(`@Route\(\s*(\w+)\s+([^)]+)\)`)
	pyValidateRe = regexp.MustCompile(`@Validate\(\s*([^)]+)\s*\)`)
	pyAuthRe     = regexp.MustCompile(`@Auth\(roles:\s*\[([^\]]+)\]\)`)
)

func ParsePyFiles(dir, workerID string) ([]Route, []AnnotationError) {
	var routes []Route
	var errs []AnnotationError

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".py") {
			return nil
		}
		r, e := parsePyFile(path, workerID)
		routes = append(routes, r...)
		errs = append(errs, e...)
		return nil
	})

	return routes, errs
}

func parsePyFile(path, workerID string) ([]Route, []AnnotationError) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []AnnotationError{{File: path, Line: 0, Message: fmt.Sprintf("cannot open file: %v", err)}}
	}
	defer func() { _ = f.Close() }()

	var routes []Route
	var errs []AnnotationError

	var pendingRoute, pendingValidate, pendingAuth string
	var routeLine int
	lineNum := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "class ") {
			continue
		}

		if !strings.HasPrefix(line, "#") {
			if pendingRoute != "" {
				route, parseErr := buildPyRoute(path, routeLine, pendingRoute, pendingValidate, pendingAuth, workerID)
				if parseErr != nil {
					errs = append(errs, *parseErr)
				} else {
					routes = append(routes, route)
				}
				pendingRoute, pendingValidate, pendingAuth = "", "", ""
			}
			continue
		}

		comment := strings.TrimPrefix(line, "#")
		comment = strings.TrimSpace(comment)

		switch {
		case pyRouteRe.MatchString(comment):
			pendingRoute = comment
			routeLine = lineNum
		case pyValidateRe.MatchString(comment):
			pendingValidate = comment
		case pyAuthRe.MatchString(comment):
			pendingAuth = comment
		}
	}

	if pendingRoute != "" {
		route, parseErr := buildPyRoute(path, routeLine, pendingRoute, pendingValidate, pendingAuth, workerID)
		if parseErr != nil {
			errs = append(errs, *parseErr)
		} else {
			routes = append(routes, route)
		}
	}

	return routes, errs
}

func buildPyRoute(file string, line int, routeAnnot, validateAnnot, authAnnot, workerID string) (Route, *AnnotationError) {
	m := pyRouteRe.FindStringSubmatch(routeAnnot)

	method := strings.ToUpper(strings.TrimSpace(m[1]))
	path := strings.TrimSpace(m[2])

	validate := ""
	if validateAnnot != "" {
		vm := pyValidateRe.FindStringSubmatch(validateAnnot)
		if vm != nil {
			validate = strings.TrimSpace(vm[1])
		}
	}

	var roles []string
	if authAnnot != "" {
		am := pyAuthRe.FindStringSubmatch(authAnnot)
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
		File:      file,
		Line:      line,
	}, nil
}