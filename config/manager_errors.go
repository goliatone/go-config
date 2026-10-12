package config

import gerrors "github.com/goliatone/go-errors"

func (c *Container[C]) providerFailure(err error, category gerrors.Category, message, code, stage string, metadata map[string]any) error {
	if !c.stagedLoad {
		return gerrors.Wrap(err, category, message).WithTextCode(code).WithMetadata(metadata)
	}
	safe := map[string]any{"stage": stage}
	for _, key := range []string{"factory_index", "total_factories", "provider_index", "source_index", "total_sources"} {
		if index, ok := metadata[key].(int); ok {
			safe[key] = index
		}
	}
	failure := gerrors.New(message, category).WithTextCode(code).WithMetadata(safe)
	failure.Source = safeContextCause(err)
	return failure
}
