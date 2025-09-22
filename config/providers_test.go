package config

import (
	"errors"
	"io/fs"
	"syscall"
	"testing"
)

func TestDefaultErrorFilter_IgnoresNotExistByDefault(t *testing.T) {
	filter := DefaultErrorFilter()

	if !filter(fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist to be ignored by default")
	}

	pathErr := &fs.PathError{Err: syscall.ENOENT}
	if !filter(pathErr) {
		t.Fatalf("expected PathError wrapping ENOENT to be ignored")
	}
}

func TestDefaultErrorFilter_DoesNotIgnoreOtherErrorsByDefault(t *testing.T) {
	filter := DefaultErrorFilter()

	if filter(errors.New("boom")) {
		t.Fatalf("expected arbitrary errors to propagate when no allowlist provided")
	}
}

func TestDefaultErrorFilter_AllowsCustomErrors(t *testing.T) {
	customErr := errors.New("custom")
	filter := DefaultErrorFilter(customErr)

	if !filter(customErr) {
		t.Fatalf("expected custom error to be allowed when provided")
	}

	if filter(errors.New("other")) {
		t.Fatalf("expected unmatched errors to propagate even with custom allowlist")
	}
}
