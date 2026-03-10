package solvers

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
)

const (
	defaultSelectKey  = "$select"
	defaultDefaultKey = "$default"
)

// SelectResolutionError captures details about an unresolved select directive.
type SelectResolutionError struct {
	NodePath    string
	SelectPath  string
	SelectValue string
	DefaultKey  string
	Reason      string
}

func (e *SelectResolutionError) Error() string {
	if e == nil {
		return "select resolution failed"
	}
	base := strings.TrimSpace(e.Reason)
	if base == "" {
		base = "select resolution failed"
	}
	if strings.TrimSpace(e.NodePath) == "" {
		return base
	}
	return fmt.Sprintf("%s at %s", base, e.NodePath)
}

type selectSolver struct {
	selectKey  string
	defaultKey string
	err        error
}

// NewSelectSolver resolves profile selectors inside object nodes, for example:
//
//	{
//	  "$select": "${app.env}",
//	  "$default": "production",
//	  "development": {...},
//	  "production": {...}
//	}
func NewSelectSolver(selectKey, defaultKey string) ConfigSolver {
	selectKey = strings.TrimSpace(selectKey)
	if selectKey == "" {
		selectKey = defaultSelectKey
	}

	defaultKey = strings.TrimSpace(defaultKey)
	if defaultKey == "" {
		defaultKey = defaultDefaultKey
	}

	return &selectSolver{
		selectKey:  selectKey,
		defaultKey: defaultKey,
	}
}

func (s *selectSolver) Err() error {
	return s.err
}

func (s *selectSolver) Solve(config *koanf.Koanf) *koanf.Koanf {
	s.err = nil

	if config == nil {
		return config
	}

	root := config.Raw()
	rootKeys := make([]string, 0, len(root))
	for key := range root {
		rootKeys = append(rootKeys, key)
	}

	resolved, err := s.resolveAny(root, config, "")
	if err != nil {
		s.err = err
		return config
	}

	resolvedRoot, ok := resolved.(map[string]any)
	if !ok {
		s.err = &SelectResolutionError{
			NodePath: "<root>",
			Reason:   "root selection must resolve to an object",
		}
		return config
	}

	for _, key := range rootKeys {
		config.Delete(key)
	}
	for key, value := range resolvedRoot {
		config.Set(key, value)
	}

	return config
}

func (s *selectSolver) resolveAny(value any, config *koanf.Koanf, path string) (any, error) {
	switch current := value.(type) {
	case map[string]any:
		if _, hasSelect := current[s.selectKey]; hasSelect {
			selected, err := s.resolveSelectMap(current, config, path)
			if err != nil {
				return nil, err
			}
			return s.resolveAny(selected, config, path)
		}

		for key, child := range current {
			nextPath := joinPath(path, key)
			resolved, err := s.resolveAny(child, config, nextPath)
			if err != nil {
				return nil, err
			}
			current[key] = resolved
		}
		return current, nil
	case []any:
		for i, child := range current {
			nextPath := fmt.Sprintf("%s[%d]", normalizePath(path), i)
			resolved, err := s.resolveAny(child, config, nextPath)
			if err != nil {
				return nil, err
			}
			current[i] = resolved
		}
		return current, nil
	default:
		return value, nil
	}
}

func (s *selectSolver) resolveSelectMap(node map[string]any, config *koanf.Koanf, path string) (any, error) {
	selectRaw := node[s.selectKey]
	selector := strings.TrimSpace(ToString(selectRaw))

	selectedKey, selectPath, selectedFound := s.resolveSelector(selector, config)
	if selectedFound {
		if selected, ok := node[selectedKey]; ok {
			return selected, nil
		}
	}

	defaultKey := ""
	if rawDefault, ok := node[s.defaultKey]; ok {
		defaultKey = strings.TrimSpace(ToString(rawDefault))
		if defaultKey != "" {
			if selected, exists := node[defaultKey]; exists {
				return selected, nil
			}
		}
	}

	reason := fmt.Sprintf("selected branch %q not found", selectedKey)
	if defaultKey != "" {
		reason = fmt.Sprintf("selected branch %q and default branch %q were not found", selectedKey, defaultKey)
	}

	return nil, &SelectResolutionError{
		NodePath:    normalizePath(path),
		SelectPath:  selectPath,
		SelectValue: selectedKey,
		DefaultKey:  defaultKey,
		Reason:      reason,
	}
}

func (s *selectSolver) resolveSelector(selector string, config *koanf.Koanf) (value string, selectPath string, found bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", "", false
	}

	if path, ok := selectorTemplatePath(selector); ok {
		return lookupSelectorPath(path, config)
	}

	if strings.Contains(selector, ".") {
		return lookupSelectorPath(selector, config)
	}

	// Literal branch key.
	return selector, "", true
}

func lookupSelectorPath(path string, config *koanf.Koanf) (value string, selectPath string, found bool) {
	path = strings.TrimSpace(path)
	if path == "" || config == nil || !config.Exists(path) {
		return "", path, false
	}
	return strings.TrimSpace(ToString(config.Get(path))), path, true
}

func selectorTemplatePath(selector string) (string, bool) {
	if !strings.HasPrefix(selector, "${") || !strings.HasSuffix(selector, "}") {
		return "", false
	}
	path := strings.TrimSpace(selector[2 : len(selector)-1])
	if path == "" {
		return "", false
	}
	return path, true
}

func joinPath(base, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return normalizePath(base)
	}
	if strings.TrimSpace(base) == "" {
		return key
	}
	return base + "." + key
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "<root>"
	}
	return path
}
