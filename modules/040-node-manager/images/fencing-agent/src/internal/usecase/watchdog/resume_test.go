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

package watchdog

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// The log tells about the resume only after the device gets the first feed
// after the resume, one time for each resume. A failed feed does not count.
func TestResumeIsLoggedAtTheFirstFeed(t *testing.T) {
	h := newHarness(t, critical)

	var out bytes.Buffer

	h.m.logger = log.NewLogger(log.WithOutput(&out))
	resumed := func() int { return strings.Count(out.String(), "Watchdog feed resumed") }

	h.step()
	h.stopEngine()
	h.step()

	h.m.ResumeFeed()

	if got := resumed(); got != 0 {
		t.Fatalf("resume lines = %d after the decision without a feed, want 0; log %q", got, out.String())
	}

	h.device.keepAliveErr = errors.New("write /dev/watchdog: input/output error")
	h.setBool(&h.shouldFeed, true)
	h.step()

	if got := resumed(); got != 0 {
		t.Fatalf("resume lines = %d after a failed feed, want 0; log %q", got, out.String())
	}

	h.device.keepAliveErr = nil
	h.step()
	h.step()

	if got := resumed(); got != 1 {
		t.Fatalf("resume lines = %d after the feeds, want 1; log %q", got, out.String())
	}

	h.stopEngine()
	h.m.ResumeFeed()
	h.setBool(&h.shouldFeed, true)
	h.step()

	if got := resumed(); got != 2 {
		t.Fatalf("resume lines = %d after the second resume and feed, want 2; log %q", got, out.String())
	}
}

// The stop of the engine reverses: ResumeFeed lets the steps feed again.
func TestResumeFeedAfterTheStop(t *testing.T) {
	h := newHarness(t, critical)
	h.step()
	h.step()

	h.stopEngine()
	h.step()

	if !h.m.ResumeFeed() || h.m.Stopped() || h.m.Final() {
		t.Fatalf("ResumeFeed after the stop: resumed=false or stopped/final; journal %v", h.j.all())
	}

	h.setBool(&h.shouldFeed, true)
	h.step()

	if got := h.j.count("keepalive"); got != 2 {
		t.Fatalf("feeds = %d, want 2 (before the stop and after the resume)", got)
	}
}

// After Finalize (DeclareSuicide is sent) the stop does not reverse.
func TestNoResumeAfterFinalize(t *testing.T) {
	h := newHarness(t, critical)
	h.step()
	h.step()

	h.stopEngine()
	h.m.Finalize()

	if h.m.ResumeFeed() || !h.m.Stopped() {
		t.Fatalf("ResumeFeed after Finalize resumed the feed; journal %v", h.j.all())
	}

	h.setBool(&h.shouldFeed, true)
	for range 5 {
		h.step()
	}

	if got := h.j.count("keepalive"); got != 1 {
		t.Fatalf("feeds = %d, want 1 (only before the stop)", got)
	}
}

// The phase Proven makes the stop final, also when the phase goes away.
func TestNoResumeAfterProven(t *testing.T) {
	h := newHarness(t, critical)
	h.step()
	h.step()

	h.stopEngine()
	h.m.SetProven("Proven")
	h.m.SetProven("")

	if h.m.ResumeFeed() || !h.m.Final() {
		t.Fatalf("ResumeFeed after Proven resumed the feed; journal %v", h.j.all())
	}

	h.setBool(&h.shouldFeed, true)
	for range 5 {
		h.step()
	}

	if got := h.j.count("keepalive"); got != 1 {
		t.Fatalf("feeds = %d, want 1 (only before the stop)", got)
	}
}

// After Close (SIGTERM) the stop does not reverse.
func TestNoResumeAfterClose(t *testing.T) {
	h := newHarness(t, critical)
	h.step()
	h.step()

	h.stopEngine()
	h.m.Close()

	if h.m.ResumeFeed() {
		t.Fatalf("ResumeFeed after Close resumed the feed; journal %v", h.j.all())
	}
}
