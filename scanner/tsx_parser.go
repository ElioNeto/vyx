package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// tsxRouteRe matches: // @Page(/some/path) or // @Page( /some/path )
var tsxPageRe = regexp.MustCompile(`^\s*//\s*@Page\(\s*([^)]+?)\s*\)`)

// tsxAuthRe matches: // @Auth(roles: ["role1", "role2"])
var tsxAuthRe = regexp.MustCompile(`^\s*//\s*@Auth\(roles:\s*\[([^\]]+)\]\s*\)`)

// ParseTSXFiles walks dir recursively and parses @Page/@Auth annotations
// from .tsx files. Each discovered page becomes a RouteEntry with:
//
//	- Method: "GET" (pages are always GET)
//	- Type: "page"
//	- WorkerID: workerID parameter (typically "node:ssr")
func ParseTSXFiles(dir, workerID string) ([]Route, []AnnotationError) {
	var routes []Route
	var errs []AnnotationError

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".tsx") {
			return nil
		}

		r, e := parseTSXFile(path, workerID)
		routes = append(routes, r...)
		errs = append(errs, e...)
		return nil
	})

	return routes, errs
}

func parseTSXFile(path, workerID string) ([]Route, []AnnotationError) {
	var routes []Route
	var errs []AnnotationError

	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0

	var pendingPage *Route // page annotation found, waiting for optional @Auth

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if m := tsxPageRe.FindStringSubmatch(line); m != nil {
			// Flush any unmatched pending page before starting a new one.
			if pendingPage != nil {
				routes = append(routes, *pendingPage)
			}
			pagePath := strings.TrimSpace(m[1])
			if pagePath == "" {
				errs = append(errs, AnnotationError{
					File:    path,
					Line:    lineNum,
					Message: "@Page requires a non-empty path",
				})
				pendingPage = nil
				continue
			}
			pendingPage = &Route{
				Path:     pagePath,
				Method:   "GET",
				WorkerID: workerID,
				Type:     "page",
				File:     path,
				Line:     lineNum,
			}
			continue
		}

		if m := tsxAuthRe.FindStringSubmatch(line); m != nil && pendingPage != nil {
			roles := parseRoleList(m[1])
			pendingPage.AuthRoles = roles
			continue
		}

		// A non-annotation line after a @Page flushes it.
		if pendingPage != nil && strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "//") {
			routes = append(routes, *pendingPage)
			pendingPage = nil
		}
	}

	if pendingPage != nil {
		routes = append(routes, *pendingPage)
	}

	return routes, errs
}

// parseRoleList parses a comma-separated, possibly quoted list of role strings.
// e.g. `"user", "admin"` → ["user", "admin"]
func parseRoleList(raw string) []string {
	parts := strings.Split(raw, ",")
	roles := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if unq, err := strconv.Unquote(p); err == nil {
			roles = append(roles, unq)
		} else if p != "" {
			roles = append(roles, p)
		}
	}
	return roles
}
