package logger

import (
	"log"
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
		log.Printf("[DEBUG] "+d.name+" | "+format+"\n", args...)
	}
}

func (d *DefaultLogger) Info(format string, args ...any) {
	if LoggerEnabled {
		log.Printf("[INFO] "+d.name+" | "+format+"\n", args...)
	}
}

func (d *DefaultLogger) Error(format string, args ...any) {
	if LoggerEnabled {
		log.Printf("[ERROR] "+d.name+" | "+format+"\n", args...)
	}
}
