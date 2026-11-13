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

package protocol

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// KeyDir is the volume with the Secret of the memberlist key.
const KeyDir = "/etc/fencing/memberlist-key"

const (
	keyFilePrimary   = "primary"
	keyFileSecondary = "secondary"
	// keySize selects AES-256.
	keySize = 32
	// keyReloadInterval is the period of the reads of the key volume.
	keyReloadInterval = time.Second
	// metricKeyParameter is the parameter label of the config_invalid metric.
	metricKeyParameter = "key"
)

// Keyring keeps the memberlist keyring in sync with the key volume.
//
// A new key in the Secret is first an additional key: the agent decrypts with
// it, but does not encrypt with it. The agent encrypts with the new key
// switchDelay after it added the key. The agent does not wait for the keyIDs
// of the peers. A key that is not in the Secret is removed not earlier than
// switchDelay after the switch and switchDelay after the last change of the
// Secret: a peer that started in this time can still encrypt with that key.
type Keyring struct {
	dir         string
	switchDelay time.Duration
	status      domain.Status
	metrics     domain.Metrics
	logger      *log.Logger

	mu sync.Mutex
	// ring is nil until the first valid primary key.
	ring *hcml.Keyring
	// secret has the valid keys of the last valid read, the primary key first.
	secret       [][]byte
	pending      []byte
	pendingSince time.Time
	switchedAt   time.Time
	// changedAt is the time of the last change of the keys in the Secret.
	changedAt time.Time
	// checked and invalid are the last reported state of the key files.
	checked bool
	invalid bool
}

func NewKeyring(dir string, switchDelay time.Duration, status domain.Status, metrics domain.Metrics, logger *log.Logger) *Keyring {
	return &Keyring{dir: dir, switchDelay: switchDelay, status: status, metrics: metrics, logger: logger}
}

// Wait reads the key volume until it has a valid primary key. While the key is
// not valid, the condition reason is InvalidKey. The agent must not create
// memberlist and must not arm the watchdog before Wait returns nil.
func (k *Keyring) Wait(ctx context.Context) error {
	ticker := time.NewTicker(keyReloadInterval)
	defer ticker.Stop()

	reported := false
	for {
		k.reload()
		if k.Ring() != nil {
			return nil
		}

		if !reported {
			k.status.SetCondition(domain.ReasonInvalidKey, "The memberlist key in the Secret is not valid.")
			reported = true
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Ring returns the keyring for memberlist, or nil before Wait returns.
func (k *Keyring) Ring() *hcml.Keyring {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.ring
}

// KeyID returns the keyID of the key that the agent encrypts with.
func (k *Keyring) KeyID() string {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.ring == nil {
		return ""
	}

	return keyID(k.ring.GetPrimaryKey())
}

// reload reads the key volume, adds the new keys as additional keys, switches
// the primary key and removes the old keys. It returns true when it switched
// the primary key.
func (k *Keyring) reload() bool {
	keys, err := readKeys(k.dir)
	k.report(err)
	if len(keys) == 0 {
		return false
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	if !slices.EqualFunc(keys, k.secret, bytes.Equal) {
		k.changedAt = now
	}
	k.secret = keys
	if k.ring == nil {
		// Two keys at the first read are a planned rotation: the peers can
		// still encrypt with the old secondary key only. Encrypt with it, and
		// rotate switches to the new key by the usual rule.
		ring, err := hcml.NewKeyring(keys, keys[len(keys)-1])
		if err != nil {
			k.logger.Error("create the memberlist keyring", "error", err)
			return false
		}
		k.ring = ring
		k.switchedAt = now
	}

	for _, key := range keys {
		if err := k.ring.AddKey(key); err != nil {
			k.logger.Error("add a key to the memberlist keyring", "error", err)
		}
	}

	return k.rotate(now)
}

// rotate switches the primary key to the first key of the Secret
// switchDelay after reload added it to the keyring, and removes the old
// keys. rotate returns true when it switched the primary key. Call it with
// k.mu locked.
func (k *Keyring) rotate(now time.Time) bool {
	switched := false

	target := k.secret[0]
	if bytes.Equal(target, k.ring.GetPrimaryKey()) {
		k.pending = nil
	} else {
		if !bytes.Equal(target, k.pending) {
			k.pending = target
			k.pendingSince = now
		}

		if now.Sub(k.pendingSince) >= k.switchDelay {
			if err := k.ring.UseKey(target); err != nil {
				k.logger.Error("switch the primary memberlist key", "error", err)
				return false
			}
			k.logger.Info("switched the primary memberlist key", "key_id", keyID(target))
			k.pending = nil
			k.switchedAt = now
			switched = true
		}
	}

	if now.Sub(k.switchedAt) >= k.switchDelay && now.Sub(k.changedAt) >= k.switchDelay {
		for _, key := range slices.Clone(k.ring.GetKeys()[1:]) {
			if slices.ContainsFunc(k.secret, func(s []byte) bool { return bytes.Equal(s, key) }) {
				continue
			}
			if err := k.ring.RemoveKey(key); err != nil {
				k.logger.Error("remove a key from the memberlist keyring", "error", err)
				continue
			}
			k.logger.Info("removed an old memberlist key", "key_id", keyID(key))
		}
	}

	return switched
}

// report sets the config_invalid metric and writes the error to the log when
// the state of the key files changes.
func (k *Keyring) report(err error) {
	invalid := err != nil

	k.mu.Lock()
	changed := !k.checked || invalid != k.invalid
	k.checked, k.invalid = true, invalid
	k.mu.Unlock()

	if !changed {
		return
	}

	k.metrics.ConfigInvalid(metricKeyParameter, invalid)
	if invalid {
		k.logger.Error("the memberlist key is not valid, the key is not used", "error", err)
	}
}

// readKeys reads the key files. When the primary key is not valid, readKeys
// returns no keys. When only the secondary key is not valid, readKeys returns
// the primary key and the error. An empty or missing secondary file means
// that there is no secondary key.
func readKeys(dir string) ([][]byte, error) {
	primary, err := os.ReadFile(filepath.Join(dir, keyFilePrimary))
	if err != nil {
		return nil, fmt.Errorf("read the primary key: %w", err)
	}
	if len(primary) != keySize {
		return nil, fmt.Errorf("the primary key has %d bytes, must have %d", len(primary), keySize)
	}

	secondary, err := os.ReadFile(filepath.Join(dir, keyFileSecondary))
	switch {
	case errors.Is(err, os.ErrNotExist) || (err == nil && len(secondary) == 0):
		return [][]byte{primary}, nil
	case err != nil:
		return [][]byte{primary}, fmt.Errorf("read the secondary key: %w", err)
	case len(secondary) != keySize:
		return [][]byte{primary}, fmt.Errorf("the secondary key has %d bytes, must have %d", len(secondary), keySize)
	}

	return [][]byte{primary, secondary}, nil
}

// keyID is the first 8 bytes of the SHA-256 of the key, in hex.
func keyID(key []byte) string {
	sum := sha256.Sum256(key)

	return hex.EncodeToString(sum[:8])
}
