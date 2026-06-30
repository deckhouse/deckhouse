// Copyright 2026 Flant JSC
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

import "log/slog"

// The marker attributes that drive terminal routing and rendering are read back from records
// with the same first-matching-key scan. These three helpers are that scan; the typed accessors
// (badgeStatus, hasFileOnly, progressEvent, …) are one-line wrappers over them, kept next to the
// attr-key constants they read.

// firstString returns the string value of the first attr on r with the given key, or "" if absent.
func firstString(r slog.Record, key string) string {
	var v string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			v = a.Value.String()
			return false
		}
		return true
	})
	return v
}

// firstBool reports whether r carries key with a true boolean value.
func firstBool(r slog.Record, key string) bool {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key && a.Value.Kind() == slog.KindBool && a.Value.Bool() {
			found = true
			return false
		}
		return true
	})
	return found
}

// firstFloat returns the float value of the first attr on r with the given key and whether it was present.
func firstFloat(r slog.Record, key string) (float64, bool) {
	var v float64
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			v = a.Value.Float64()
			found = true
			return false
		}
		return true
	})
	return v, found
}
