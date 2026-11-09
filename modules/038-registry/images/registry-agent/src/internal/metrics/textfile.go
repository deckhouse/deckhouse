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

package metrics

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DefaultTextfileInterval is how often the textfile is rewritten.
//
// The scrape interval of the node exporter that reads it, give or take: a shorter one would
// rewrite a file nobody has read yet, a longer one would leave every other scrape looking at
// the same numbers.
const DefaultTextfileInterval = 30 * time.Second

// Textfile hands the agent's metrics to the node exporter by writing them into its textfile
// directory.
//
// This is how they reach Prometheus at all. The agent is a static pod that has to work while
// the API server does not, so nothing can scrape it the usual way: a kube-rbac-proxy beside it
// would authenticate against that same API server. The node exporter is already on every node
// and already scraped, and reads whatever *.prom files it finds in that directory, so the agent
// writes one there and needs neither a port nor anyone's credentials to be seen.
//
// What is written is Gatherer and nothing else, and the caller has to keep it that way: the
// node exporter merges these files into its own answer, and a go_* or process_* family here
// would collide with the exporter's own families of the same names.
type Textfile struct {
	Log *slog.Logger

	// Path is the file, inside the node exporter's directory and ending in .prom.
	Path string

	// Gatherer is what goes into it.
	Gatherer prometheus.Gatherer

	// Interval is how often it is rewritten. DefaultTextfileInterval when zero.
	Interval time.Duration
}

// Run writes the file at once and then on every tick until ctx is done, and removes it then.
//
// Removed rather than left, because a file left behind is a set of series the node exporter
// keeps reporting as current: a counter frozen at its last value and a layout origin that no
// longer describes anything. A stop that skips this — a SIGKILL, a node losing power — is the
// case the file's own mtime, which the node exporter reports, is there for.
//
// No error here stops the agent or is returned. Every pull on the node goes through the agent,
// and none of them may depend on whether its metrics could be written.
func (t *Textfile) Run(ctx context.Context) {
	interval := t.Interval
	if interval <= 0 {
		interval = DefaultTextfileInterval
	}

	t.writeLogged()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := os.Remove(t.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Log.Warn("the metrics textfile could not be removed", "path", t.Path, "error", err.Error())
			}
			return
		case <-ticker.C:
			t.writeLogged()
		}
	}
}

// Start runs Run in a goroutine of its own and returns what stops it: the returned function
// cancels Run and returns only once the file is removed.
//
// The wait is the point. On SIGTERM the agent's server returns as soon as its shutdown begins,
// the process exits right after, and a removal that merely raced the exit would lose about as
// often as it won. Nothing else removes the file then: the bashible step that deletes the
// manifest removes it too, but before the kubelet has stopped the agent, so a tick in between
// writes it again, and the step does not run a second time once it is in sync. The caller
// defers the returned function, so every way out of the agent, a failed listener included,
// goes through it.
func (t *Textfile) Start(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	removed := make(chan struct{})
	go func() {
		defer close(removed)
		t.Run(ctx)
	}()
	return func() {
		cancel()
		<-removed
	}
}

func (t *Textfile) writeLogged() {
	if err := t.Write(); err != nil {
		t.Log.Warn("the metrics textfile could not be written", "path", t.Path, "error", err.Error())
	}
}

// Write replaces the file once.
//
// Atomically: WriteToTextfile writes a temporary file beside the target and renames it over,
// so the node exporter reads either the previous set or this one and never half of either. The
// temporary name does not end in .prom, so it is not read even while it exists.
func (t *Textfile) Write() error {
	return prometheus.WriteToTextfile(t.Path, t.Gatherer)
}
