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
func (s variables) Solve(context *koanf.Koanf) *koanf.Koanf {

	c := context.All()

	for key, val := range c {
		v2, ok := val.(string)
		if !ok {
			continue
		}
		s.keypath(key, v2, context)
	}

	return context
}

func (s variables) keypath(key, val string, context *koanf.Koanf) {
	start := strings.Index(val, s.delimeters.Start)
	if start == -1 {
		return
	}

	if len(s.delimeters.Start) > 1 {
		start = start + len(s.delimeters.Start)
	}

	end := strings.Index(val[start:], s.delimeters.End)
	if end == -1 || end < start {
		return
	}
	end = end + len(s.delimeters.Start)

	path := val[start:end]
	if path == val {
		return
	}

	if !context.Exists(path) {
		return
	}

	newVal := context.Get(path)
	if len(s.delimeters.Start)+len(path)+len(s.delimeters.End) != len(val) {
		newVal = s.replaceValue(val, newVal)
	}

	context.Set(key, newVal)
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
