package solvers

import (
	"strings"

	"github.com/knadh/koanf/v2"
)

type variables struct {
	delimeters *delimiters
}

// NewVariablesSolver will resolve variables
func NewVariablesSolver(s, e string) ConfigSolver {
	return &variables{
		delimeters: &delimiters{
			Start: s,
			End:   e,
		},
	}
}

// Solve will transform a configuration object
func (s variables) Solve(config *koanf.Koanf) *koanf.Koanf {

	c := config.All()

	for key, val := range c {
		v2, ok := val.(string)
		if !ok {
			continue
		}
		s.keypath(key, v2, config)
	}

	return config
}

func (s variables) keypath(key, val string, config *koanf.Koanf) {
	current := val
	visited := map[string]struct{}{
		current: {},
	}
	changed := false

	for {
		next, updated, directValue, setDirect := s.replaceTokens(key, current, config)
		if !updated {
			break
		}
		changed = true
		if setDirect {
			config.Set(key, directValue)
			return
		}
		if next == current {
			break
		}
		if _, seen := visited[next]; seen {
			break
		}
		visited[next] = struct{}{}
		current = next
	}

	if changed && current != val {
		config.Set(key, current)
	}
}

func (s variables) replaceTokens(key, input string, config *koanf.Koanf) (string, bool, any, bool) {
	startDelimiter := s.delimeters.Start
	endDelimiter := s.delimeters.End
	offset := 0
	var out strings.Builder
	out.Grow(len(input))
	changed := false

	for {
		startIndex := strings.Index(input[offset:], startDelimiter)
		if startIndex == -1 {
			out.WriteString(input[offset:])
			break
		}
		startIndex += offset

		contentStart := startIndex + len(startDelimiter)
		endOffset := strings.Index(input[contentStart:], endDelimiter)
		if endOffset == -1 {
			out.WriteString(input[offset:])
			break
		}

		contentEnd := contentStart + endOffset
		tokenEnd := contentEnd + len(endDelimiter)
		path := input[contentStart:contentEnd]

		out.WriteString(input[offset:startIndex])

		if path == "" || path == key || !config.Exists(path) {
			out.WriteString(input[startIndex:tokenEnd])
			offset = tokenEnd
			continue
		}

		resolved := config.Get(path)
		isFullMatch := startIndex == 0 && tokenEnd == len(input)
		if isFullMatch {
			if resolvedStr, ok := resolved.(string); ok {
				if resolvedStr == input {
					return input, false, nil, false
				}
				return resolvedStr, true, nil, false
			}
			return input, true, resolved, true
		}

		out.WriteString(ToString(resolved))
		changed = true
		offset = tokenEnd
	}

	next := out.String()
	if next != input {
		changed = true
	}

	return next, changed, nil, false
}
