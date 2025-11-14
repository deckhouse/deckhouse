/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log *zap.Logger
)

// Init initializes the logger with the specified log level
func Init(level string) error {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var err error
	log, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

// Sync flushes any buffered log entries
func Sync() error {
	if log != nil {
		return log.Sync()
	}
	return nil
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	if log != nil {
		log.Debug(msg, fields...)
	}
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	if log != nil {
		log.Info(msg, fields...)
	}
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	if log != nil {
		log.Warn(msg, fields...)
	}
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	if log != nil {
		log.Error(msg, fields...)
	}
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	if log != nil {
		log.Fatal(msg, fields...)
	}
}

// Helper functions for common logging patterns

// Debugf logs a formatted debug message
func Debugf(msg string, args ...interface{}) {
	if log != nil {
		log.Sugar().Debugf(msg, args...)
	}
}

// Infof logs a formatted info message
func Infof(msg string, args ...interface{}) {
	if log != nil {
		log.Sugar().Infof(msg, args...)
	}
}

// Warnf logs a formatted warning message
func Warnf(msg string, args ...interface{}) {
	if log != nil {
		log.Sugar().Warnf(msg, args...)
	}
}

// Errorf logs a formatted error message
func Errorf(msg string, args ...interface{}) {
	if log != nil {
		log.Sugar().Errorf(msg, args...)
	}
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(msg string, args ...interface{}) {
	if log != nil {
		log.Sugar().Fatalf(msg, args...)
	}
}
