package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/distribution/reference"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/storage/driver"
)

// onTTLExpiryFunc is called when a repository's TTL expires
type expiryFunc func(reference.Reference) error

const (
	entryTypeBlob = iota
	entryTypeManifest
	indexSaveFrequency = 5 * time.Second
)

// schedulerEntry represents an entry in the scheduler
// fields are exported for serialization
type schedulerEntry struct {
	Key       string    `json:"Key"`
	Expiry    time.Time `json:"ExpiryData"`
	EntryType int       `json:"EntryType"`

	timer *time.Timer
}

// New returns a new instance of the scheduler
func New(ctx context.Context, ttl time.Duration, driver driver.StorageDriver, path string) *TTLExpirationScheduler {
	return &TTLExpirationScheduler{
		entries:         make(map[string]*schedulerEntry),
		ttl:             ttl,
		driver:          driver,
		pathToStateFile: path,
		ctx:             ctx,
		stopped:         true,
		doneChan:        make(chan struct{}),
		saveTimer:       time.NewTicker(indexSaveFrequency),
	}
}

// TTLExpirationScheduler is a scheduler used to perform actions
// when TTLs expire
type TTLExpirationScheduler struct {
	sync.Mutex

	entries map[string]*schedulerEntry

	driver          driver.StorageDriver
	ctx             context.Context
	ttl             time.Duration
	pathToStateFile string

	stopped bool

	onBlobExpire     expiryFunc
	onManifestExpire expiryFunc

	indexDirty bool
	saveTimer  *time.Ticker
	doneChan   chan struct{}
}

// OnBlobExpire is called when a scheduled blob's TTL expires
func (ttles *TTLExpirationScheduler) OnBlobExpire(f expiryFunc) {
	ttles.Lock()
	defer ttles.Unlock()

	ttles.onBlobExpire = f
}

// OnManifestExpire is called when a scheduled manifest's TTL expires
func (ttles *TTLExpirationScheduler) OnManifestExpire(f expiryFunc) {
	ttles.Lock()
	defer ttles.Unlock()

	ttles.onManifestExpire = f
}

// AddBlob would schedule a blob for deletion once its TTL expires, and does nothing.
//
// Deckhouse: this store has one owner for retention, and it is not a timer inside the registry. As a
// pull-through cache, distribution schedules everything it caches for deletion after `repositoryTTL` —
// a compile-time constant of one week, with no configuration field to change it: `configuration.Proxy`
// has no TTL at all. Measured on a cluster, a blob fetched through the store appeared in
// `scheduler-state.json` with an expiry seven days out, and `ttl: 0s` in the live configuration changed
// nothing.
//
// That contradicts what the module promises. An image pulled through the cache has to stay in it,
// because the cache is what the cluster pulls from once the upstream is dropped — and retention already
// has an owner: the module's garbage collection, which keeps what the deployed releases need and removes
// the rest on a schedule the operator sets. Two deleters in one store, one of them invisible and
// unconfigurable, is a race between a promise and a timer rather than a policy.
//
// Emptied rather than left unreachable behind an early return, which is how this arrived here: it was a
// build-time edit applied to a cloned fork, and `go vet` reported the body it left behind as dead code.
func (ttles *TTLExpirationScheduler) AddBlob(blobRef reference.Canonical) error {
	return nil
}

// AddManifest would schedule a manifest for deletion once its TTL expires, and does nothing. See AddBlob.
func (ttles *TTLExpirationScheduler) AddManifest(manifestRef reference.Canonical) error {
	return nil
}

// Start starts the scheduler
func (ttles *TTLExpirationScheduler) Start() error {
	ttles.Lock()
	defer ttles.Unlock()

	err := ttles.initState()
	if err != nil {
		return err
	}

	if !ttles.stopped {
		return fmt.Errorf("scheduler already started")
	}

	dcontext.GetLogger(ttles.ctx).Infof("Starting cached object TTL expiration scheduler...")
	ttles.stopped = false

	// Start timer for each deserialized entry
	for _, entry := range ttles.entries {
		entry.timer = ttles.startTimer(entry, time.Until(entry.Expiry))
	}

	// Start a ticker to periodically save the entries index

	go func() {
		for {
			select {
			case <-ttles.saveTimer.C:
				ttles.Lock()
				if !ttles.indexDirty {
					ttles.Unlock()
					continue
				}

				err := ttles.writeState()
				if err != nil {
					dcontext.GetLogger(ttles.ctx).Errorf("Error writing scheduler state: %s", err)
				} else {
					ttles.indexDirty = false
				}
				ttles.Unlock()

			case <-ttles.doneChan:
				return
			}
		}
	}()

	return nil
}

func (ttles *TTLExpirationScheduler) add(r reference.Reference, eType int) {
	entry := &schedulerEntry{
		Key:       r.String(),
		Expiry:    time.Now().Add(ttles.ttl),
		EntryType: eType,
	}
	dcontext.GetLogger(ttles.ctx).Infof("Adding new scheduler entry for %s with ttl=%s", entry.Key, time.Until(entry.Expiry))
	if oldEntry, present := ttles.entries[entry.Key]; present && oldEntry.timer != nil {
		oldEntry.timer.Stop()
	}
	ttles.entries[entry.Key] = entry
	entry.timer = ttles.startTimer(entry, ttles.ttl)
	ttles.indexDirty = true
}

func (ttles *TTLExpirationScheduler) startTimer(entry *schedulerEntry, ttl time.Duration) *time.Timer {
	return time.AfterFunc(ttl, func() {
		ttles.Lock()
		defer ttles.Unlock()

		var f expiryFunc

		switch entry.EntryType {
		case entryTypeBlob:
			f = ttles.onBlobExpire
		case entryTypeManifest:
			f = ttles.onManifestExpire
		default:
			f = func(reference.Reference) error {
				return fmt.Errorf("scheduler entry type")
			}
		}

		ref, err := reference.Parse(entry.Key)
		if err == nil {
			if err := f(ref); err != nil {
				dcontext.GetLogger(ttles.ctx).Errorf("Scheduler error returned from OnExpire(%s): %s", entry.Key, err)
			}
		} else {
			dcontext.GetLogger(ttles.ctx).Errorf("Error unpacking reference: %s", err)
		}

		delete(ttles.entries, entry.Key)
		ttles.indexDirty = true
	})
}

// Stop stops the scheduler.
func (ttles *TTLExpirationScheduler) Stop() {
	ttles.Lock()
	defer ttles.Unlock()

	if err := ttles.writeState(); err != nil {
		dcontext.GetLogger(ttles.ctx).Errorf("Error writing scheduler state: %s", err)
	}

	for _, entry := range ttles.entries {
		entry.timer.Stop()
	}

	close(ttles.doneChan)
	ttles.saveTimer.Stop()
	ttles.stopped = true
}

func (ttles *TTLExpirationScheduler) writeState() error {
	jsonBytes, err := json.Marshal(ttles.entries)
	if err != nil {
		return err
	}

	err = ttles.driver.PutContent(ttles.ctx, ttles.pathToStateFile, jsonBytes)
	if err != nil {
		return err
	}

	return nil
}

func (ttles *TTLExpirationScheduler) initState() error {
	if _, err := ttles.driver.Stat(ttles.ctx, ttles.pathToStateFile); err != nil {
		switch err := err.(type) {
		case driver.PathNotFoundError:
			return ttles.writeState()
		default:
			return err
		}
	}

	bytes, err := ttles.driver.GetContent(ttles.ctx, ttles.pathToStateFile)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, &ttles.entries)
	if err != nil {
		return err
	}
	return nil
}
