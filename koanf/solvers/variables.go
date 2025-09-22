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

	for {
		start := strings.Index(current, s.delimeters.Start)
		if start == -1 {
			break
		}

		contentStart := start + len(s.delimeters.Start)
		endOffset := strings.Index(current[contentStart:], s.delimeters.End)
		if endOffset == -1 {
			break
		}

		contentEnd := contentStart + endOffset
		path := current[contentStart:contentEnd]
		if path == "" || path == key {
			break
		}

		if !config.Exists(path) {
			break
		}

		resolved := config.Get(path)
		isFullMatch := start == 0 && contentEnd+len(s.delimeters.End) == len(current)

		if isFullMatch {
			if key == path {
				break
			}
			config.Set(key, resolved)
			return
		}

		next := s.replaceValue(current, resolved)
		if next == current {
			break
		}

		current = next
	}

	if current != val {
		config.Set(key, current)
	}
}

func (s variables) replaceValue(input string, replacement any) string {
	startDelimiter := s.delimeters.Start
	endDelimiter := s.delimeters.End

	startIndex := strings.Index(input, startDelimiter)
	if startIndex == -1 {
		return input
	}

	endIndex := strings.Index(input[startIndex:], endDelimiter)
	if endIndex == -1 {
		return input
	}

	endIndex += startIndex

	// Extract parts before and after the delimited substring
	before := input[:startIndex]
	after := input[endIndex+len(endDelimiter):]

	// Concatenate the parts with the replacement
	result := before + ToString(replacement) + after

	return result
}
