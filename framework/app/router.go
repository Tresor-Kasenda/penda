package app

import (
	"errors"
	"slices"
	"strings"
)

type route struct {
	method      string
	pattern     string
	parts       []segment
	handler     Handler
	middlewares []Middleware
}

type segmentKind int

const (
	segmentStatic segmentKind = iota
	segmentParam
	segmentWildcard
)

type segment struct {
	kind  segmentKind
	value string
}

func (a *App) match(method, path string) (route, map[string]string, []string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	allowedSet := map[string]struct{}{}

	for i := range a.routes {
		rt := &a.routes[i]
		params, ok := matchPath(rt.parts, path)
		if !ok {
			continue
		}

		if rt.method == normalizedMethod {
			copied := *rt
			copied.middlewares = append([]Middleware(nil), rt.middlewares...)
			return copied, params, nil, true
		}

		allowedSet[rt.method] = struct{}{}
	}

	if len(allowedSet) == 0 {
		return route{}, nil, nil, false
	}

	allowed := make([]string, 0, len(allowedSet))
	for methodName := range allowedSet {
		allowed = append(allowed, methodName)
	}
	slices.Sort(allowed)

	return route{}, nil, allowed, false
}

func parsePattern(path string) ([]segment, error) {
	if path == "" {
		return nil, errors.New("path cannot be empty")
	}
	if path[0] != '/' {
		return nil, errors.New("path must start with /")
	}

	parts := splitPath(path)
	segments := make([]segment, 0, len(parts))

	for idx, part := range parts {
		switch {
		case strings.HasPrefix(part, ":"):
			name := strings.TrimPrefix(part, ":")
			if name == "" {
				return nil, errors.New("path param name cannot be empty")
			}
			segments = append(segments, segment{kind: segmentParam, value: name})
		case strings.HasPrefix(part, "*"):
			name := strings.TrimPrefix(part, "*")
			if name == "" {
				return nil, errors.New("wildcard name cannot be empty")
			}
			if idx != len(parts)-1 {
				return nil, errors.New("wildcard must be the last segment")
			}
			segments = append(segments, segment{kind: segmentWildcard, value: name})
		default:
			segments = append(segments, segment{kind: segmentStatic, value: part})
		}
	}

	return segments, nil
}

func matchPath(pattern []segment, path string) (map[string]string, bool) {
	parts := splitPath(path)
	params := map[string]string{}

	pathIdx := 0
	for patternIdx := 0; patternIdx < len(pattern); patternIdx++ {
		seg := pattern[patternIdx]
		if seg.kind == segmentWildcard {
			params[seg.value] = strings.Join(parts[pathIdx:], "/")
			return params, true
		}

		if pathIdx >= len(parts) {
			return nil, false
		}

		part := parts[pathIdx]
		switch seg.kind {
		case segmentStatic:
			if seg.value != part {
				return nil, false
			}
		case segmentParam:
			params[seg.value] = part
		}

		pathIdx++
	}

	if pathIdx != len(parts) {
		return nil, false
	}

	return params, true
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func normalizeGroupPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return ""
	}

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	return strings.TrimRight(prefix, "/")
}

func joinPaths(prefix, path string) string {
	if prefix == "/" {
		prefix = ""
	}
	prefix = strings.TrimRight(prefix, "/")

	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if prefix == "" {
		return path
	}

	return prefix + path
}
