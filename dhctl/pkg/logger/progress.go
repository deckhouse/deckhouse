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

import (
	"context"
	"log/slog"
)

const (
	// attrKeyProgressEvent marks a progress lifecycle record. Values: start, end, pause, resume.
	attrKeyProgressEvent = "progress_event"
	// attrKeyProgressName names the progress session opened by a start event.
	attrKeyProgressName = "progress_name"
	// attrKeyProgressValue carries the bar fraction (float64 in [0,1]).
	attrKeyProgressValue = "progress_value"
	// attrKeyProgressTitle carries the bar title accompanying a progress value.
	attrKeyProgressTitle = "progress_title"
)

const (
	progressStart  = "start"
	progressEnd    = "end"
	progressPause  = "pause"
	progressResume = "resume"
)

// StartProgress opens a terminal progress session named name.
func StartProgress(ctx context.Context, l *slog.Logger, name string) {
	l.InfoContext(ctx, "progress:start",
		slog.String(attrKeyProgressEvent, progressStart),
		slog.String(attrKeyProgressName, name))
}

// FinishProgress closes the active terminal progress session.
func FinishProgress(ctx context.Context, l *slog.Logger) {
	l.InfoContext(ctx, "progress:end",
		slog.String(attrKeyProgressEvent, progressEnd))
}

// PauseProgress stops rendering the progress session (e.g. around interactive input).
func PauseProgress(ctx context.Context, l *slog.Logger) {
	l.InfoContext(ctx, "progress:pause",
		slog.String(attrKeyProgressEvent, progressPause))
}

// ResumeProgress restarts rendering the progress session after a pause.
func ResumeProgress(ctx context.Context, l *slog.Logger) {
	l.InfoContext(ctx, "progress:resume",
		slog.String(attrKeyProgressEvent, progressResume))
}

// Progress advances the active bar to frac (0..1) and sets its title.
func Progress(ctx context.Context, l *slog.Logger, frac float64, title string) {
	l.InfoContext(ctx, "progress",
		slog.Float64(attrKeyProgressValue, frac),
		slog.String(attrKeyProgressTitle, title))
}

// progressEvent returns the progress_event value carried by r, or "" if absent.
func progressEvent(r slog.Record) string { return firstString(r, attrKeyProgressEvent) }

// progressValue returns the progress_value carried by r and whether it was present.
func progressValue(r slog.Record) (float64, bool) { return firstFloat(r, attrKeyProgressValue) }

// progressTitle returns the progress_title carried by r, or "" if absent.
func progressTitle(r slog.Record) string { return firstString(r, attrKeyProgressTitle) }

// recordProgressName returns the progress_name value carried by r, or "" if absent.
func recordProgressName(r slog.Record) string { return firstString(r, attrKeyProgressName) }
