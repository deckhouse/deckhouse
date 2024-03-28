// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logger

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// NewLogger returns initialized slog logger
func NewLogger(level *slog.LevelVar) *slog.Logger {
	replace := func(_ []string, attr slog.Attr) slog.Attr {
		switch attr.Key {
		case slog.SourceKey:
			src, ok := attr.Value.Any().(*slog.Source)
			if ok {
				return slog.String(slog.SourceKey, fmt.Sprintf("%s:%d", src.File, src.Line))
			}
		case slog.TimeKey:
			return slog.String(slog.TimeKey, attr.Value.Time().Format(time.RFC3339))
		}

		return attr
	}

	opts := &slog.HandlerOptions{
		AddSource:   true,
		Level:       level,
		ReplaceAttr: replace,
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, opts))

	return log
}

// Err returns an slog.Attr for a string value
func Err(err error) slog.Attr {
	const errKey = "error"

	if err == nil {
		return slog.Attr{}
	}
	return slog.Attr{Key: errKey, Value: slog.StringValue(err.Error())}
}
