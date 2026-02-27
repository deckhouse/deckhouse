/*
Copyright 2026 Flant JSC

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
	"log/slog"
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func NewLogger(levelStr string) *log.Logger {
	level := getLogLevel(levelStr)

	logger := log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithLevel(level),
		log.WithHandlerType(log.JSONHandlerType),
	)
	return logger
}

func getLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
