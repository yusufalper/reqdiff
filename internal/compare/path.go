package compare

import (
	"errors"
	"strings"
)

type Paths [][]string

func CompilePaths(patterns []string) (Paths, error) {
	out := make(Paths, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			out = append(out, []string{})
			continue
		}
		if !strings.HasPrefix(pattern, "/") {
			return nil, errors.New("ignore path must be a JSON Pointer")
		}
		parts := strings.Split(pattern[1:], "/")
		for i, part := range parts {
			for j := 0; j < len(part); j++ {
				if part[j] == '~' {
					if j+1 >= len(part) || (part[j+1] != '0' && part[j+1] != '1') {
						return nil, errors.New("invalid JSON Pointer escape")
					}
					j++
				}
			}
			parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		}
		out = append(out, parts)
	}
	return out, nil
}
func match(pattern, path []string) bool {
	for i, p := range pattern {
		if p != "*" && p != path[i] {
			return false
		}
	}
	return true
}
func (p Paths) Match(path []string) bool {
	for _, pattern := range p {
		if len(pattern) == len(path) && match(pattern, path) {
			return true
		}
	}
	return false
}
func (p Paths) Covers(path []string) bool {
	for _, pattern := range p {
		if len(pattern) <= len(path) && match(pattern, path) {
			return true
		}
	}
	return false
}
func pointer(path []string) string {
	var b strings.Builder
	for _, p := range path {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(p, "~", "~0"), "/", "~1"))
	}
	return b.String()
}
