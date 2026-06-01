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

const (
	// attrKeyCompact marks a record to appear in the compact view. Without it, a record is file-only.
	attrKeyCompact = "compact"

	attrKeyProcessEvent = "process_event"
	attrKeyProcessName  = "process_name"

	// attrKeyBadge marks a curated status record (phase transition, lib-connection Success/Fail/
	// FailRetry) so the terminal renderer draws the legacy colored status badge before the title
	// instead of level-plain text. Value is one of the badge* constants below.
	attrKeyBadge = "badge"
)

// Badge status values carried by attrKeyBadge.
const (
	badgeSuccess = "success" // green background, " SUCCESS "
	badgeFailed  = "failed"  // red background, " FAILED "
	badgeWarning = "warning" // yellow background, " WARNING "
)

// BadgeSuccess/BadgeFailed/BadgeWarning return the attribute tagging a record to render with the
// matching colored status badge. Pair with ShowInCompacted() so the record reaches the compact view.
func BadgeSuccess() slog.Attr { return slog.String(attrKeyBadge, badgeSuccess) }
func BadgeFailed() slog.Attr  { return slog.String(attrKeyBadge, badgeFailed) }
func BadgeWarning() slog.Attr { return slog.String(attrKeyBadge, badgeWarning) }

// badgeStatus returns the badge value carried by r, or "" if absent.
func badgeStatus(r slog.Record) string {
	var status string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == attrKeyBadge {
			status = a.Value.String()
			return false
		}
		return true
	})
	return status
}

type processEvent string

const (
	processStart processEvent = "start"
	processEnd   processEvent = "end"
	processFail  processEvent = "fail"
)

// attrKeyFileOnly marks a record that must stay in the debug file and never reach the compact
// terminal, even at Warn/Error level. Used for lib-connection's streamed command output
// (bashible/ssh/exec per-line, including remote `set -x` stderr): in interactive mode it would
// flood the terminal, so the user is pointed to the debug-log file instead. -v still shows it.
const attrKeyFileOnly = "file_only"

// ShowInCompacted returns the attribute that tags a record to appear in the compact view.
func ShowInCompacted() slog.Attr { return slog.Bool(attrKeyCompact, true) }

// FileOnly tags a record to stay file-only on the terminal: suppressed in the compact view
// regardless of level, shown only with -v.
func FileOnly() slog.Attr { return slog.Bool(attrKeyFileOnly, true) }

func hasFileOnly(r slog.Record) bool {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == attrKeyFileOnly && a.Value.Kind() == slog.KindBool && a.Value.Bool() {
			found = true
			return false
		}
		return true
	})
	return found
}

func hasShowInCompacted(r slog.Record) bool {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == attrKeyCompact && a.Value.Kind() == slog.KindBool && a.Value.Bool() {
			found = true
			return false
		}
		return true
	})
	return found
}

// isRendererMarker reports whether r carries a progress/process control marker. Such records are
// not visible text — they drive the bar, the current-action line, and the process boxes — so the
// handler always routes them to the terminal renderer regardless of compact/verbose mode.
func isRendererMarker(r slog.Record) bool {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case attrKeyProcessEvent, attrKeyProgressEvent, attrKeyProgressValue:
			found = true
			return false
		}
		return true
	})
	return found
}

func processAttr(ev processEvent, name string) []slog.Attr {
	return []slog.Attr{
		slog.String(attrKeyProcessEvent, string(ev)),
		slog.String(attrKeyProcessName, name),
	}
}
