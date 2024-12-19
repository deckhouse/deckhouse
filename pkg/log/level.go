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

package log

import (
	"fmt"
	"log/slog"
)

var _ slog.Leveler = (*Level)(nil)

type Level slog.Level

const (
	LevelTrace Level = -8
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
	LevelFatal Level = 12
)

func (l Level) Level() slog.Level {
	return slog.Level(l)
}

func (l Level) String() string {
	str := func(base string, val Level) string {
		if val == 0 {
			return base
		}
		return fmt.Sprintf("%s%+d", base, val)
	}

	switch {
	case l < LevelDebug:
		return str("trace", l-LevelTrace)
	case l < LevelInfo:
		return str("debug", l-LevelDebug)
	case l < LevelWarn:
		return str("info", l-LevelInfo)
	case l < LevelError:
		return str("warn", l-LevelWarn)
	case l < LevelFatal:
		return str("error", l-LevelError)
	default:
		return str("fatal", l-LevelFatal)
	}
}
