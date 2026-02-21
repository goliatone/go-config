package solvers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/knadh/koanf/v2"
	"go.beyondstorage.io/v5/services"
	"go.beyondstorage.io/v5/types"
)

type ProtocolResolver func(uri string, state *uriResolveState) (any, error)

type URIErrorStrategy int

const (
	URIErrorLeaveUnchanged URIErrorStrategy = iota
	URIErrorRemoveKey
)

type URISolverOption func(*uris)

type uris struct {
	fs            fs.FS
	delimeters    *delimiters
	resolvers     map[string]ProtocolResolver
	newStorager   func(conn string) (storageReader, error)
	errorStrategy URIErrorStrategy
}

type storageReader interface {
	ReadWithContext(ctx context.Context, path string, w io.Writer, pairs ...types.Pair) (int64, error)
}

type uriResolveState struct {
	storagersByConn map[string]storageReader
	valuesByURI     map[string]string
	includeByURI    map[string]any
	includePending  map[string]struct{}
}

// NewURISolver will resolve variables
func NewURISolver(s, e string) ConfigSolver {
	return NewURISolverWithFS(s, e, os.DirFS("."))
}

func NewURISolverWithFS(s, e string, f fs.FS) ConfigSolver {
	return NewURISolverWithFSAndOptions(s, e, f)
}

func NewURISolverWithOptions(s, e string, opts ...URISolverOption) ConfigSolver {
	return NewURISolverWithFSAndOptions(s, e, os.DirFS("."), opts...)
}

func NewURISolverWithFSAndOptions(s, e string, f fs.FS, opts ...URISolverOption) ConfigSolver {
	solver := &uris{
		fs: f,
		delimeters: &delimiters{
			Start: s,
			End:   e,
		},
		resolvers:     map[string]ProtocolResolver{},
		newStorager:   newStorageReader,
		errorStrategy: URIErrorLeaveUnchanged,
	}
	solver.registerDefaultResolvers()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(solver)
	}
	return solver
}

func WithURIErrorStrategy(strategy URIErrorStrategy) URISolverOption {
	return func(s *uris) {
		s.errorStrategy = strategy
	}
}

func WithURIOnErrorRemove() URISolverOption {
	return WithURIErrorStrategy(URIErrorRemoveKey)
}

func WithURIOnErrorLeaveUnchanged() URISolverOption {
	return WithURIErrorStrategy(URIErrorLeaveUnchanged)
}

func WithURIProtocolResolver(protocol string, resolver ProtocolResolver) URISolverOption {
	return func(s *uris) {
		s.registerResolver(protocol, resolver)
	}
}

func newStorageReader(conn string) (storageReader, error) {
	return services.NewStoragerFromString(conn)
}

func (s *uris) registerDefaultResolvers() {
	s.registerResolver("file", func(uri string, _ *uriResolveState) (any, error) {
		return SolveFileProtocol(s.fs, uri)
	})
	s.registerResolver("base64", func(uri string, _ *uriResolveState) (any, error) {
		return SolveBase64DecodeProtocol(s.fs, uri)
	})
	s.registerResolver("storage", func(uri string, state *uriResolveState) (any, error) {
		return s.resolveStorageProtocol(uri, state)
	})
	s.registerResolver("include", func(uri string, state *uriResolveState) (any, error) {
		return s.resolveIncludeProtocol(uri, state)
	})
}

func (s *uris) registerResolver(protocol string, resolver ProtocolResolver) {
	if protocol == "" || resolver == nil {
		return
	}
	s.resolvers[protocol] = resolver
}

func newURIResolveState() *uriResolveState {
	return &uriResolveState{
		storagersByConn: map[string]storageReader{},
		valuesByURI:     map[string]string{},
		includeByURI:    map[string]any{},
		includePending:  map[string]struct{}{},
	}
}

// Solve will transform a configuration object
func (s uris) Solve(config *koanf.Koanf) *koanf.Koanf {
	c := config.All()
	state := newURIResolveState()

	for key, val := range c {
		v2, ok := val.(string)
		if !ok {
			continue
		}
		s.keypath(key, v2, config, state)
	}

	return config
}

func (s uris) keypath(key, val string, config *koanf.Koanf, state *uriResolveState) {
	protocol, uri, ok := s.extractProtocolURI(val)
	if !ok {
		return
	}
	content, err := s.resolveByProtocol(protocol, uri, state)
	if err != nil {
		if s.errorStrategy == URIErrorRemoveKey {
			config.Delete(key)
		}
		return
	}
	config.Set(key, content)
}

func SolveFileProtocol(f fs.FS, uri string) (string, error) {
	content := ""
	safePath, err := sanitizeFileURIPath(uri)
	if err != nil {
		return content, err
	}
	b, err := fs.ReadFile(f, safePath)
	if err == nil {
		content = string(b)
		content = strings.TrimRight(content, "\n")
	}
	return content, err
}

func sanitizeFileURIPath(uri string) (string, error) {
	cleaned := path.Clean(strings.TrimSpace(uri))
	if cleaned == "." || cleaned == "/" {
		return "", fmt.Errorf("invalid file uri path %q", uri)
	}
	if !fs.ValidPath(cleaned) {
		return "", fmt.Errorf("invalid file uri path %q", uri)
	}
	return cleaned, nil
}

func SolveBase64DecodeProtocol(f fs.FS, uri string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(uri)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *uris) resolveStorageProtocol(uri string, state *uriResolveState) (string, error) {
	if state == nil {
		state = newURIResolveState()
	}
	if content, ok := state.valuesByURI[uri]; ok {
		return content, nil
	}
	conn, objectPath, err := parseStorageURI(uri)
	if err != nil {
		return "", err
	}

	store, ok := state.storagersByConn[conn]
	if !ok {
		store, err = s.newStorager(conn)
		if err != nil {
			return "", err
		}
		state.storagersByConn[conn] = store
	}

	var out bytes.Buffer
	_, err = store.ReadWithContext(context.Background(), objectPath, &out)
	if err != nil {
		return "", err
	}

	content := strings.TrimRight(out.String(), "\n")
	state.valuesByURI[uri] = content

	return content, nil
}

func (s *uris) resolveIncludeProtocol(uri string, state *uriResolveState) (any, error) {
	if state == nil {
		state = newURIResolveState()
	}
	if cached, ok := state.includeByURI[uri]; ok {
		return cached, nil
	}
	if _, pending := state.includePending[uri]; pending {
		return nil, fmt.Errorf("cyclic include uri %q", uri)
	}

	protocol, innerURI, err := parseProtocolURI(uri, s.delimeters.End)
	if err != nil {
		return nil, err
	}

	state.includePending[uri] = struct{}{}
	defer delete(state.includePending, uri)

	rawValue, err := s.resolveByProtocol(protocol, innerURI, state)
	if err != nil {
		return nil, err
	}

	parsed, err := decodeIncludedValue(rawValue)
	if err != nil {
		return nil, err
	}
	state.includeByURI[uri] = parsed
	return parsed, nil
}

func (s uris) resolveByProtocol(protocol, uri string, state *uriResolveState) (any, error) {
	resolver, ok := s.resolvers[protocol]
	if !ok {
		return nil, fmt.Errorf("unknown uri protocol %q", protocol)
	}
	return resolver(uri, state)
}

func (s uris) extractProtocolURI(value string) (protocol string, uri string, ok bool) {
	start := strings.Index(value, s.delimeters.Start)
	if start != 0 {
		return "", "", false
	}
	end := strings.Index(value, s.delimeters.End)
	if end < start {
		return "", "", false
	}

	start = start + len(s.delimeters.Start)
	protocol = value[start:end]

	end = end + len(s.delimeters.End)
	uri = value[end:]
	if protocol == "" || uri == "" {
		return "", "", false
	}

	return protocol, uri, true
}

func parseProtocolURI(raw string, delimiter string) (protocol string, uri string, err error) {
	if delimiter == "" {
		delimiter = "://"
	}

	idx := strings.Index(raw, delimiter)
	if idx <= 0 {
		return "", "", fmt.Errorf("invalid nested include uri %q", raw)
	}
	protocol = strings.TrimSpace(raw[:idx])
	uri = strings.TrimSpace(raw[idx+len(delimiter):])
	if protocol == "" || uri == "" {
		return "", "", fmt.Errorf("invalid nested include uri %q", raw)
	}

	return protocol, uri, nil
}

func decodeIncludedValue(value any) (any, error) {
	switch v := value.(type) {
	case nil:
		return nil, fmt.Errorf("include payload is nil")
	case string:
		return parseJSON(v)
	case []byte:
		return parseJSON(string(v))
	default:
		// If a custom resolver already returned an object, accept it.
		return v, nil
	}
}

func parseJSON(input string) (any, error) {
	var out any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseStorageURI(uri string) (conn string, objectPath string, err error) {
	parts := strings.SplitN(strings.TrimSpace(uri), "#", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid storage uri %q", uri)
	}

	conn = strings.TrimSpace(parts[0])
	objectPath = strings.TrimLeft(strings.TrimSpace(parts[1]), "/")
	if conn == "" {
		return "", "", fmt.Errorf("invalid storage uri %q", uri)
	}
	if objectPath == "" {
		return "", "", fmt.Errorf("invalid storage uri %q", uri)
	}

	return conn, objectPath, nil
}
