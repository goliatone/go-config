package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	glog "github.com/goliatone/go-logger/glog"
	masker "github.com/goliatone/go-masker"
)

type Logger interface {
	Debug(format string, args ...any)
	Info(format string, args ...any)
	Error(format string, args ...any)
}

var LoggerEnabled = true

type DefaultLogger struct {
	name    string
	writer  io.Writer
	once    sync.Once
	backend glog.Logger
}

func NewDefaultLogger(name string) *DefaultLogger {
	return &DefaultLogger{name: name}
}

func newDefaultLogger(name string, writer io.Writer) *DefaultLogger {
	return &DefaultLogger{name: name, writer: writer}
}

func (d *DefaultLogger) Debug(msg string, args ...any) {
	if LoggerEnabled {
		message, fields := d.sanitize(msg, args...)
		d.getBackend().Debug(message, fields...)
	}
}

func (d *DefaultLogger) Info(msg string, args ...any) {
	if LoggerEnabled {
		message, fields := d.sanitize(msg, args...)
		d.getBackend().Info(message, fields...)
	}
}

func (d *DefaultLogger) Error(msg string, args ...any) {
	if LoggerEnabled {
		message, fields := d.sanitize(msg, args...)
		d.getBackend().Error(message, fields...)
	}
}

func (d *DefaultLogger) getBackend() glog.Logger {
	d.once.Do(func() {
		options := []glog.Option{
			glog.WithName(d.name),
			glog.WithLevel(glog.Debug),
			glog.WithLoggerTypeConsole(),
		}
		writer := d.writer
		if writer == nil {
			writer = os.Stderr
		}
		options = append(options, glog.WithWriter(writer))
		d.backend = glog.NewLogger(options...)
	})
	return d.backend
}

func (d *DefaultLogger) sanitize(msg string, args ...any) (string, []any) {
	if len(args) == 0 {
		return msg, nil
	}

	maskedArgs, err := maskArgs(args)
	if err != nil {
		return msg, []any{"masking_error", "log fields omitted"}
	}
	if strings.Contains(msg, "%") {
		return fmt.Sprintf(msg, maskedArgs...), nil
	}
	return msg, maskedArgs
}

func maskArgs(args []any) ([]any, error) {
	masked := make([]any, 0, len(args))
	for i := 0; i < len(args); {
		key, isKey := args[i].(string)
		if isKey && i+1 < len(args) {
			wrapped, err := MaskSensitive(map[string]any{key: args[i+1]})
			if err != nil {
				return nil, err
			}
			values, ok := wrapped.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("masked log fields have type %T", wrapped)
			}
			masked = append(masked, key, values[key])
			i += 2
			continue
		}

		value, err := MaskSensitive(args[i])
		if err != nil {
			return nil, err
		}
		masked = append(masked, value)
		i++
	}
	return masked, nil
}

var secureMasker, secureMaskerErr = masker.NewSecure()

// MaskSensitive returns a deeply copied value with known credential fields
// replaced by go-masker's fixed redaction marker. It never returns the input
// value when masking fails.
func MaskSensitive(value any) (any, error) {
	if secureMaskerErr != nil {
		return nil, secureMaskerErr
	}
	return secureMasker.Mask(value)
}
