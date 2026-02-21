package logger

import (
	"fmt"
	"log"
	"strings"
)

type Logger interface {
	Debug(format string, args ...any)
	Info(format string, args ...any)
	Error(format string, args ...any)
}

var LoggerEnabled = true

type DefaultLogger struct {
	name string
}

func NewDefaultLogger(name string) *DefaultLogger {
	return &DefaultLogger{name: name}
}

func (d *DefaultLogger) Debug(format string, args ...any) {
	if LoggerEnabled {
		log.Printf("[DEBUG] %s | %s", d.name, formatMessage(format, args...))
	}
}

func (d *DefaultLogger) Info(format string, args ...any) {
	if LoggerEnabled {
		log.Printf("[INFO] %s | %s", d.name, formatMessage(format, args...))
	}
}

func (d *DefaultLogger) Error(format string, args ...any) {
	if LoggerEnabled {
		log.Printf("[ERROR] %s | %s", d.name, formatMessage(format, args...))
	}
}

func formatMessage(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}

	if strings.Contains(format, "%") {
		return fmt.Sprintf(format, args...)
	}

	var b strings.Builder
	b.WriteString(format)
	if len(args)%2 == 0 {
		for i := 0; i < len(args); i += 2 {
			b.WriteString(" ")
			b.WriteString(fmt.Sprint(args[i]))
			b.WriteString("=")
			b.WriteString(fmt.Sprint(args[i+1]))
		}
		return b.String()
	}

	for _, arg := range args {
		b.WriteString(" ")
		b.WriteString(fmt.Sprint(arg))
	}
	return b.String()
}
