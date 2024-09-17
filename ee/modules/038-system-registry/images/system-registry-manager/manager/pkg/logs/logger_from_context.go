/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package logs

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"path/filepath"
	"runtime"
)

type contextKey string
type ContextHook struct{}

func (hook ContextHook) Levels() []log.Level {
	return log.AllLevels
}

func (hook ContextHook) Fire(entry *log.Entry) error {
	pc, file, line, ok := runtime.Caller(8) // 8 to get to the correct stack frame
	if !ok {
		file = "unknown"
		line = 0
	}

	// Add filename and line number to log entry
	fileName := fmt.Sprintf("%s:%d", filepath.Base(file), line)
	entry.Data["file"] = fileName

	// Add function name to log entry
	fn := runtime.FuncForPC(pc)
	if fn != nil {
		funcName := filepath.Ext(fn.Name())[1:]
		entry.Data["func"] = funcName
	}

	return nil
}

const loggerKey contextKey = "logger"

func SetLoggerToContext(ctx context.Context, processName string) context.Context {
	newLogger := log.WithField("app", processName)
	newLogger.Logger.AddHook(ContextHook{})
	return context.WithValue(ctx, loggerKey, newLogger)
}

func GetLoggerFromContext(ctx context.Context) *log.Entry {
	logger, ok := ctx.Value(loggerKey).(*log.Entry)
	if !ok {
		// Return a new log entry with "unknown" process if log entry is absent in context
		return log.WithField("app", "unknown")
	}
	return logger
}
