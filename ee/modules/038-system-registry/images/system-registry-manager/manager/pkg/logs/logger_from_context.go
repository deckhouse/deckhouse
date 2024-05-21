/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package logs

import (
	"context"

	log "github.com/sirupsen/logrus"
)

type contextKey string

const loggerKey contextKey = "logger"

func SetLoggerToContext(ctx context.Context, processName string) context.Context {
	newLogger := log.WithField("app", processName)
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
