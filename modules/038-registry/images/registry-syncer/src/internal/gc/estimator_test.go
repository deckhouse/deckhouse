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

package gc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// estimatorBench is an estimator with a clock the test moves and a store whose state it sets.
type estimatorBench struct {
	*Estimator

	now       time.Time
	plan      Plan
	filling   bool
	set       string
	estimates int
	published []int64
	err       error
}

func newEstimatorBench() *estimatorBench {
	b := &estimatorBench{
		now:  time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		plan: Plan{Known: true},
		set:  "v1.76.6",
	}
	b.Estimator = &Estimator{
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Plan:    func() Plan { return b.plan },
		Filling: func() bool { return b.filling },
		Set:     func() string { return b.set },
		Estimate: func(context.Context) (int64, error) {
			b.estimates++
			return int64(b.estimates) * 1000, b.err
		},
		Publish: func(bytes int64, _ time.Time) { b.published = append(b.published, bytes) },
		Now:     func() time.Time { return b.now },
	}
	return b
}

// after moves the clock and lets the estimator look.
func (b *estimatorBench) after(d time.Duration) {
	b.now = b.now.Add(d)
	b.step(context.Background())
}

func TestTheEstimateIsRedoneEveryHalfHourAndNoSooner(t *testing.T) {
	b := newEstimatorBench()

	b.after(time.Minute)
	assert.Equal(t, 1, b.estimates, "the first look makes the first estimate")

	for range 29 {
		b.after(time.Minute)
	}
	assert.Equal(t, 1, b.estimates, "nothing changed within the half hour")

	b.after(time.Minute)
	assert.Equal(t, 2, b.estimates)
	assert.Equal(t, []int64{1000, 2000}, b.published)
}

func TestANewSetIsEstimatedAgainButNotMoreOftenThanTheFloor(t *testing.T) {
	b := newEstimatorBench()
	b.after(time.Minute)

	b.set = "v1.77.0"
	b.after(time.Minute)
	assert.Equal(t, 1, b.estimates, "the set changed a minute after the last estimate")

	b.after(4 * time.Minute)
	assert.Equal(t, 2, b.estimates, "five minutes after the last one")

	b.set = "v1.77.1"
	b.after(5 * time.Minute)
	assert.Equal(t, 3, b.estimates)
	b.after(5 * time.Minute)
	assert.Equal(t, 3, b.estimates, "the same set as at the last estimate")
}

func TestAnUnknownSetIsNoChange(t *testing.T) {
	b := newEstimatorBench()
	b.after(time.Minute)

	b.set = ""
	b.after(10 * time.Minute)
	assert.Equal(t, 1, b.estimates)
}

func TestAFinishedCollectionMakesTheEstimateStale(t *testing.T) {
	b := newEstimatorBench()
	b.after(time.Minute)

	b.Collected()
	b.after(5 * time.Minute)
	assert.Equal(t, 2, b.estimates)
	b.after(5 * time.Minute)
	assert.Equal(t, 2, b.estimates, "once per collection")
}

// The behaviour the metric is rare for: a store being filled is not recounted on every look, and is
// not counted at all until the fill is over. The last estimate stays, with its time.
func TestNothingIsEstimatedWhileTheStoreIsWrittenTo(t *testing.T) {
	writers := map[string]func(b *estimatorBench, on bool){
		"a fill is owed":       func(b *estimatorBench, on bool) { b.plan.FillPending = on },
		"a fill runs":          func(b *estimatorBench, on bool) { b.filling = on },
		"the state is unknown": func(b *estimatorBench, on bool) { b.plan.Known = !on },
		"somebody pushes": func(b *estimatorBench, on bool) {
			b.Pushing = func() bool { return on }
		},
		"a collection runs": func(b *estimatorBench, on bool) {
			b.Collecting = func() bool { return on }
		},
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			b := newEstimatorBench()
			b.after(time.Minute)

			write(b, true)
			b.set = "v1.77.0"
			for range 120 {
				b.after(time.Minute)
			}
			assert.Equal(t, 1, b.estimates, "two hours of writing, a new set among them")

			write(b, false)
			b.after(5 * time.Minute)
			assert.Equal(t, 2, b.estimates, "within the floor once the store is quiet")
		})
	}
}

func TestAFailedEstimateLeavesTheLastOneAndIsTriedAgainLater(t *testing.T) {
	b := newEstimatorBench()
	b.after(time.Minute)

	b.err = errors.New("the installer cannot be read")
	b.after(30 * time.Minute)
	assert.Equal(t, 2, b.estimates)
	assert.Equal(t, []int64{1000}, b.published, "a failure publishes nothing")

	b.after(time.Minute)
	assert.Equal(t, 2, b.estimates, "not again before the floor")

	b.err = nil
	b.after(4 * time.Minute)
	assert.Equal(t, 3, b.estimates)
	assert.Equal(t, []int64{1000, 3000}, b.published)
}

func TestTheEstimatorStopsWithItsContext(t *testing.T) {
	b := newEstimatorBench()
	ctx, cancel := context.WithCancel(context.Background())
	looks := 0
	b.Sleep = func(ctx context.Context, _ time.Duration) bool {
		looks++
		if looks == 3 {
			cancel()
		}
		return ctx.Err() == nil
	}

	b.Run(ctx)
	assert.Equal(t, 3, looks)
	assert.Equal(t, 1, b.estimates)
}

// TestAFailedEstimateKeepsACollectionOwed: the collection made the last figure stale, and an estimate
// that failed after it must not leave that figure standing for another half hour.
func TestAFailedEstimateKeepsACollectionOwed(t *testing.T) {
	b := newEstimatorBench()
	b.after(time.Minute)

	b.Collected()
	b.err = errors.New("the API server did not answer")
	b.after(5 * time.Minute)
	assert.Equal(t, 2, b.estimates)

	b.err = nil
	b.after(5 * time.Minute)
	assert.Equal(t, 3, b.estimates, "still owed for the collection, five minutes on")
	assert.Equal(t, []int64{1000, 3000}, b.published)
}

// TestTheStoreIsNotAskedAboutUnlessAnEstimateIsOwed: the plan may measure the whole store, and was
// asked once a minute for an estimate due every half hour.
func TestTheStoreIsNotAskedAboutUnlessAnEstimateIsOwed(t *testing.T) {
	b := newEstimatorBench()
	plans := 0
	b.Plan = func() Plan { plans++; return b.plan }

	b.after(time.Minute)
	assert.Equal(t, 1, plans)

	for range 10 {
		b.after(time.Minute)
	}
	assert.Equal(t, 1, plans, "nothing owed, nothing asked")
}

// TestAStoreBeingWrittenToIsAskedAboutNoMoreOftenThanTheFloor: asking measures the whole store, and a
// fill that takes an hour is no reason to measure it sixty times.
func TestAStoreBeingWrittenToIsAskedAboutNoMoreOftenThanTheFloor(t *testing.T) {
	b := newEstimatorBench()
	plans := 0
	b.Plan = func() Plan { plans++; return b.plan }
	b.filling = true

	for range 60 {
		b.after(time.Minute)
	}
	assert.Zero(t, b.estimates)
	assert.Equal(t, 12, plans, "once in five minutes over an hour")
}
