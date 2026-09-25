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

package rejoin

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const (
	idleTick = time.Second

	jitterFraction = 0.2
)

type Params struct {
	Interval    time.Duration
	MaxInterval time.Duration
}

type Deps struct {
	Attempt         func(ctx context.Context) error
	EpisodeStarted  func()
	HasQuorum       func() bool
	OwnFailedRecord func(ctx context.Context) bool
	NotMember       func(err error) bool
	Changed         <-chan struct{}
	Sleep           func(ctx context.Context, d time.Duration) bool
}

type episode struct {
	open        bool
	start       time.Time
	quorum, own bool
	attempts    int
	delay       time.Duration
	lastDelay   time.Duration
	lastClass   string
	streak      streak
}

type streak struct {
	class    string
	attempts int
	start    time.Time
}

type Loop struct {
	params Params
	deps   Deps
	logger *log.Logger
	ep     episode
}

func New(params Params, deps Deps, logger *log.Logger) *Loop {
	if deps.EpisodeStarted == nil {
		deps.EpisodeStarted = func() {}
	}

	if deps.OwnFailedRecord == nil {
		deps.OwnFailedRecord = func(context.Context) bool { return false }
	}

	if deps.NotMember == nil {
		deps.NotMember = func(error) bool { return false }
	}

	if deps.Sleep == nil {
		deps.Sleep = sleep
	}

	return &Loop{params: params, deps: deps, logger: logger}
}

func (l *Loop) Run(ctx context.Context) error {
	ticker := time.NewTicker(idleTick)
	defer ticker.Stop()

	defer l.closeEpisode("rejoin stopped by shutdown")

	idle := func() {
		select {
		case <-ctx.Done():
		case <-l.deps.Changed:
		case <-ticker.C:
		}
	}

	for ctx.Err() == nil {
		quorum, own := l.deps.HasQuorum(), l.deps.OwnFailedRecord(ctx)
		if ctx.Err() != nil {
			return nil
		}

		if quorum && !own {
			l.finish()
			idle()

			continue
		}

		l.begin(quorum, own)

		started := time.Now()
		l.ep.attempts++

		err := l.deps.Attempt(ctx)
		if ctx.Err() != nil {
			return nil
		}

		if err == nil {
			quorum, own = l.deps.HasQuorum(), l.deps.OwnFailedRecord(ctx)
			if ctx.Err() != nil {
				return nil
			}

			if quorum && !own {
				l.finish()
				idle()

				continue
			}
		}

		delay := jitter(l.ep.delay)
		l.report(err, started, delay, quorum, own)

		slept := l.deps.Sleep(ctx, delay)
		l.ep.lastDelay = delay

		if !slept {
			return nil
		}

		l.ep.delay = min(l.ep.delay*2, l.params.MaxInterval)
	}

	return nil
}

func (l *Loop) begin(quorum, own bool) {
	if !l.ep.open {
		l.ep = episode{
			open:      true,
			start:     time.Now(),
			quorum:    quorum,
			own:       own,
			delay:     l.params.Interval,
			lastClass: domain.JoinErrorClassNone,
		}

		if quorum {
			l.logger.Warn("a peer recorded this node as failed, rejoin started although gossip quorum holds")
		} else {
			l.logger.Info("gossip quorum lost, rejoin started", "own_failed_record", own)
		}

		l.deps.EpisodeStarted()

		return
	}

	if quorum == l.ep.quorum && own == l.ep.own {
		return
	}

	l.ep.quorum, l.ep.own = quorum, own

	l.logger.Info("rejoin trigger changed", "has_quorum", quorum, "own_failed_record", own)
}

func (l *Loop) report(err error, started time.Time, delay time.Duration, quorum, own bool) {
	if err == nil {
		l.endStreak(started)

		l.logger.Debug("rejoin attempt joined, the trigger remains",
			"attempt", l.ep.attempts,
			"next_in", delay.String(),
			"has_quorum", quorum,
			"own_failed_record", own,
		)

		return
	}

	class := domain.JoinErrorClassTransport
	if l.deps.NotMember(err) {
		class = domain.JoinErrorClassNotMember
	}

	l.ep.lastClass = class

	if l.ep.streak.class != class {
		l.endStreak(started)
		l.ep.streak = streak{class: class, start: started}
	}

	l.ep.streak.attempts++

	msg := "rejoin attempt failed"
	if class == domain.JoinErrorClassNotMember {
		msg = "this node is not a member of its NodeGroup, rejoin does not join until that changes"
	}

	level := slog.LevelDebug
	if l.ep.streak.attempts == 1 {
		level = slog.LevelWarn
	}

	l.logger.Log(context.Background(), level, msg,
		"error", err,
		"attempt", l.ep.attempts,
		"attempt_elapsed", time.Since(started).String(),
		"next_in", delay.String(),
	)
}

func (l *Loop) endStreak(end time.Time) {
	if l.ep.streak.class == "" {
		return
	}

	l.summarize("rejoin failure streak ended", l.ep.streak.attempts, end.Sub(l.ep.streak.start), l.ep.streak.class)
	l.ep.streak = streak{}
}

func (l *Loop) finish() {
	l.closeEpisode("rejoin finished, gossip quorum holds and no peer records this node as failed")
}

func (l *Loop) closeEpisode(msg string) {
	if !l.ep.open {
		return
	}

	l.summarize(msg, l.ep.attempts, time.Since(l.ep.start), l.ep.lastClass)
	l.ep = episode{}
}

func (l *Loop) summarize(msg string, attempts int, elapsed time.Duration, class string) {
	l.logger.Info(msg,
		"attempts", attempts,
		"elapsed", elapsed.Truncate(time.Millisecond).String(),
		"last_delay", l.ep.lastDelay.String(),
		"last_error_class", class,
	)
}

func jitter(delay time.Duration) time.Duration {
	spread := float64(delay) * jitterFraction

	return delay + time.Duration((rand.Float64()*2-1)*spread)
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
