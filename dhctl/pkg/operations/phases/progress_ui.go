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

package phases

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// RunProgress opens a terminal progress session named name, runs body (which
// emits Progress events into the supplied channel), and closes the session
// afterwards. A background goroutine consumes the channel and advances the bar.
//
// The "current action" spinner that the legacy bar fed from a label channel is
// now driven automatically by dhlog.RunProcess markers, so no label channel is
// needed here.
func RunProgress(ctx context.Context, l *slog.Logger, name string, body func(progressCh chan Progress) error) error {
	progressCh, finish, err := InitProgress(ctx, l, name)
	if err != nil {
		return err
	}
	defer finish()

	return body(progressCh)
}

// InitProgress opens a terminal progress session named name and starts a
// background goroutine that consumes Progress events sent on the returned
// channel, advancing the bar. The returned finish function closes the channel,
// waits for the consumer to drain, and closes the progress session. It is safe
// to call finish exactly once (e.g. via defer).
//
// This mirrors the contract of the legacy deferred-finish progress bar
// initializer, but drives the slog TerminalUIHandler instead of the pterm bar.
func InitProgress(ctx context.Context, l *slog.Logger, name string) (chan Progress, func(), error) {
	titles, err := LoadTitles()
	if err != nil {
		return nil, nil, fmt.Errorf("load phase titles: %w", err)
	}

	progressCh := make(chan Progress, 5)

	dhlog.StartProgress(ctx, l, name)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		consumeProgress(ctx, l, titles, progressCh, stop)
	}()

	var finished bool
	finish := func() {
		if finished {
			return
		}
		finished = true

		// Signal the consumer to stop, but do NOT close progressCh: the pipeline's deferred
		// Finalize can still emit a final Progress event after the body (and runProgress) has
		// returned. Senders use a non-blocking send, so a late event to the un-drained, never-
		// closed channel neither panics nor blocks.
		close(stop)
		<-done

		dhlog.FinishProgress(ctx, l)
	}

	return progressCh, finish, nil
}

// consumeProgress reads Progress events and advances the new progress bar. The
// increment math is ported verbatim from the legacy updateProgress loop: the
// legacy bar operated on an integer scale of 0..100, here we keep the same
// integer math and report the resulting fraction (current/100) to
// dhlog.Progress.
func consumeProgress(ctx context.Context, l *slog.Logger, titles *Titles, progressCh chan Progress, stop <-chan struct{}) {
	inc := 0
	lastCompleted := ""
	current := 0 // 0..100, mirrors the legacy pterm bar's Current.

	for {
		var msg Progress
		select {
		case <-stop:
			return
		case msg = <-progressCh:
		}

		if inc == 0 || lastCompleted == "" {
			// calculate increment
			phasesCount := len(msg.Phases)
			for _, p := range msg.Phases {
				phasesCount += len(p.SubPhases)
			}
			if phasesCount > 0 {
				inc = 100 / phasesCount
			}

			text := phaseToString(titles, msg, false)
			if text != "" {
				dhlog.Progress(ctx, l, float64(current)/100, text)
			}
		}

		if msg.CompletedPhase != "" {
			completed := phaseToString(titles, msg, true)

			if completed == lastCompleted {
				continue
			}

			increment := int(math.Round(msg.Progress*100) - float64(current))

			if increment == 0 {
				increment = inc
			}

			if current+increment > 100 {
				increment = 100 - current
			}

			current += increment
			lastCompleted = completed

			// The successful phase transition is THE only thing tagged for the compact view.
			l.InfoContext(ctx, strings.TrimSpace(completed), dhlog.ShowInCompacted(), dhlog.BadgeSuccess())
			dhlog.Progress(ctx, l, float64(current)/100, phaseToString(titles, msg, false))
		}
	}
}

// phaseToString maps a Progress event to a human-readable bar title. Titles
// are sourced from the embedded i18n catalog (see titles.go), so the CLI
// progress bar, the GetPhaseCatalog RPC and the `phase-catalog` CLI command
// share a single source of truth.
func phaseToString(titles *Titles, p Progress, completed bool) string {
	msg := ""
	if completed {
		if p.CompletedSubPhase != "" {
			msg = fmt.Sprintf("%s: %s", titles.Phase(p.CurrentPhase), titles.SubPhase(p.CompletedSubPhase))
		} else {
			msg = titles.Phase(p.CompletedPhase)
		}
	} else {
		if p.CurrentSubPhase != "" {
			msg = fmt.Sprintf("%s: %s", titles.Phase(p.CurrentPhase), titles.SubPhase(p.CurrentSubPhase))
		} else {
			if p.CurrentPhase != "" {
				msg = titles.Phase(p.CurrentPhase)
			} else {
				msg = titles.Phase(p.CompletedPhase)
			}
		}
	}

	return fmt.Sprintf("%-60s", msg)
}
