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

package bound

import (
	"context"
	"errors"
	"fmt"
	"strings"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	storagemiddleware "github.com/distribution/distribution/v3/registry/storage/driver/middleware"
)

// MiddlewareName is the storage middleware this package registers with distribution.
//
// A middleware over upstream's own filesystem driver rather than a driver of its own, and the
// difference is the garbage collection: it opens the storage by driver name, with no middleware,
// so a sweep — which only deletes — runs on the plain driver without anything having to know. The
// serving configuration keeps naming `filesystem`, exactly as upstream documents it.
const MiddlewareName = "deckhouse-bounded"

// Mode is what a refusal does to a writer that is already streaming.
type Mode string

const (
	// Fail returns the refusal from the write. For the write endpoint, where the client is the one
	// sending the bytes and the sooner it hears, the less it sends for nothing.
	Fail Mode = "fail"

	// Discard keeps accepting the stream and drops it, and fails the commit instead. For the
	// serving half, where the writer is the local copy of a cache miss: distribution sends the blob
	// to the client and into the writer through one `io.MultiWriter`, so a write that fails there
	// cuts off the CLIENT mid-blob. Dropped instead, the client gets the whole blob from the
	// upstream and the store simply does not keep it — nothing is linked to an upload that never
	// committed, and the cancellation that follows removes it.
	//
	// The same goes for a copy that would not fit from the start, or would hardly fit: once the
	// store is within a measurement of a limit, a new copy is not written at all — its writer drops
	// everything and fails the commit, and its scratch files are skipped. Otherwise every miss at the
	// edge would write until refused and then be deleted, and the disk would churn for nothing. The
	// client does not notice either way.
	//
	// What is not an upload is never dropped: a manifest or a link reported as written and not there
	// would be a store lying about what it holds. Those fail, a miss that needs one is a clean server
	// error, and the agent answers those by trying the next registry.
	Discard Mode = "discard"
)

// Options builds the middleware's options for one driver instance.
//
// The guard travels in the options as a value, not a name: the configuration is handed to
// distribution in memory, and a process-global table of guards would be one more thing a test
// could leave behind.
func Options(guard *Guard, mode Mode) map[string]any {
	return map[string]any{"guard": guard, "onrefusal": string(mode)}
}

func init() {
	if err := storagemiddleware.Register(MiddlewareName, newDriver); err != nil {
		panic(err)
	}
}

func newDriver(_ context.Context, inner storagedriver.StorageDriver, options map[string]any) (storagedriver.StorageDriver, error) {
	guard, ok := options["guard"].(*Guard)
	if !ok || guard == nil {
		return nil, fmt.Errorf("the %s storage middleware needs a guard", MiddlewareName)
	}

	mode := Mode(fmt.Sprint(options["onrefusal"]))
	switch mode {
	case Fail, Discard:
	default:
		return nil, fmt.Errorf("the %s storage middleware: onrefusal %q is neither %q nor %q",
			MiddlewareName, mode, Fail, Discard)
	}

	return &driver{StorageDriver: inner, guard: guard, mode: mode}, nil
}

// driver is the storage with the guard in front of every write. Reads, listings, moves and
// deletions pass straight through: none of them takes space.
type driver struct {
	storagedriver.StorageDriver
	guard *Guard
	mode  Mode
}

func (d *driver) PutContent(ctx context.Context, path string, content []byte) error {
	if d.mode == Discard && tag(path) && d.guard.Refusal() != nil {
		// A cache miss resolves its tag before it fetches the manifest, and records the tag first.
		// Written while the store refuses, it would outlive the manifest the store then refused —
		// a tag naming an image the store does not have, which is the store claiming to hold what
		// it does not. A live cluster left exactly that behind. Not written, the tag is recorded
		// with its manifest on the next miss the store has room for; with the upstream there, the
		// tag is asked of the upstream first anyway.
		return nil
	}
	if err := d.guard.take(path, int64(len(content)), true); err != nil {
		if d.drops(path, err) {
			// An upload's scratch file — its start time, its hash state. The upload it belongs to
			// is being dropped too, and its cancellation ignores what is not there.
			return nil
		}
		return err
	}
	return d.StorageDriver.PutContent(ctx, path, content)
}

func (d *driver) Writer(ctx context.Context, path string, append bool) (storagedriver.FileWriter, error) {
	if d.mode == Discard && upload(path) {
		if full := d.guard.Refusal(); full != nil {
			return &dropped{refused: full}, nil
		}
	}
	if err := d.guard.take(path, 0, true); err != nil {
		if d.drops(path, err) {
			return &dropped{refused: err}, nil
		}
		return nil, err
	}
	inner, err := d.StorageDriver.Writer(ctx, path, append)
	if err != nil {
		return nil, err
	}
	return &writer{FileWriter: inner, path: path, guard: d.guard, mode: d.mode}, nil
}

// writer checks every chunk it is given against the guard before passing it on.
type writer struct {
	storagedriver.FileWriter
	path  string
	guard *Guard
	mode  Mode

	// refused is set once the guard refused a chunk in Discard mode. Everything after it is
	// dropped, and the commit reports it.
	refused error
}

func (w *writer) Write(p []byte) (int, error) {
	if w.refused != nil {
		return len(p), nil
	}
	if err := w.guard.take(w.path, int64(len(p)), false); err != nil {
		if w.mode == Discard {
			w.refused = err
			return len(p), nil
		}
		return 0, err
	}
	return w.FileWriter.Write(p)
}

func (w *writer) Commit(ctx context.Context) error {
	if w.refused != nil {
		return w.refused
	}
	return w.FileWriter.Commit(ctx)
}

// drops tells whether a refused write at path is dropped rather than failed.
func (d *driver) drops(path string, err error) bool {
	var full *ErrStoreFull
	return d.mode == Discard && upload(path) && errors.As(err, &full)
}

// upload tells a path inside an upload in progress, which is scratch until it commits.
func upload(path string) bool { return strings.Contains(path, "/_uploads/") }

// tag tells a path that records a tag, which is a claim about a manifest rather than content.
func tag(path string) bool { return strings.Contains(path, "/_manifests/tags/") }

// dropped is the writer of a copy the store will not keep: it takes the stream, keeps nothing, and
// fails the commit.
type dropped struct {
	refused error
	size    int64
}

func (w *dropped) Write(p []byte) (int, error) {
	w.size += int64(len(p))
	return len(p), nil
}

func (w *dropped) Size() int64                  { return w.size }
func (w *dropped) Close() error                 { return nil }
func (w *dropped) Cancel(context.Context) error { return nil }
func (w *dropped) Commit(context.Context) error { return w.refused }
