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
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

var (
	key1 = []byte("dh-test-secret-0123456789abcdef0")
	key2 = []byte("dh-test-secret-0123456789abcdef1")
	key3 = []byte("dh-test-secret-0123456789abcdef2")
	// badKey has a wrong length.
	badKey = []byte("dh-test-secret-012345678")
)

// testSwitchDelay is the default of KEY_SWITCH_DELAY.
const testSwitchDelay = 2 * time.Minute

type fakeStatus struct {
	mu      sync.Mutex
	reasons []domain.Reason
}

func (s *fakeStatus) SetCondition(reason domain.Reason, _ string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reasons = append(s.reasons, reason)
}

func (s *fakeStatus) SetCPSource(domain.CPSource) {}

func (s *fakeStatus) SetProfile(string) {}

func (s *fakeStatus) last() domain.Reason {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.reasons) == 0 {
		return ""
	}

	return s.reasons[len(s.reasons)-1]
}

type fakeMetrics struct {
	mu      sync.Mutex
	invalid map[string]bool
}

func (m *fakeMetrics) Decision(domain.FeedDecision)                  {}
func (m *fakeMetrics) Poll()                                         {}
func (m *fakeMetrics) PollAnswer(domain.Network, domain.AnswerClass) {}

func (m *fakeMetrics) ConfigInvalid(parameter string, invalid bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.invalid == nil {
		m.invalid = make(map[string]bool)
	}
	m.invalid[parameter] = invalid
}

// keyInvalid returns the value of config_invalid{parameter="key"} and whether the series exists.
func (m *fakeMetrics) keyInvalid() (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.invalid[metricKeyParameter]

	return v, ok
}

// writeKeys writes the key volume like the kubelet: the files appear at once.
func writeKeys(t *testing.T, dir string, keys ...[]byte) {
	t.Helper()

	tmp, err := os.MkdirTemp(filepath.Dir(dir), "keys")
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{keyFilePrimary, keyFileSecondary} {
		if i < len(keys) {
			if err := os.WriteFile(filepath.Join(tmp, name), keys[i], 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		t.Fatal(err)
	}
}

func newTestKeyring(t *testing.T, keys ...[]byte) (*Keyring, string, *fakeStatus, *fakeMetrics) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "memberlist-key")
	writeKeys(t, dir, keys...)
	status, metrics := &fakeStatus{}, &fakeMetrics{}

	return NewKeyring(dir, testSwitchDelay, status, metrics, log.NewNop()), dir, status, metrics
}

func ringKeys(k *Keyring) [][]byte {
	return k.Ring().GetKeys()
}

func TestKeyID(t *testing.T) {
	sum := sha256.Sum256(key1)
	if got, want := keyID(key1), hex.EncodeToString(sum[:8]); got != want || len(got) != 16 {
		t.Errorf("keyID = %q, want %q", got, want)
	}
}

// First start of the agent. One key in the Secret is a new or a regenerated
// key: the agent encrypts with it at once. Two keys are a planned rotation:
// the agent encrypts with the old secondary key and switches to the new
// primary key after the delay, as the running agents do.
func TestKeyringFirstStart(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys [][]byte
		// ring is the keyring after the start, the primary key first.
		ring [][]byte
	}{
		{name: "one key", keys: [][]byte{key1}, ring: [][]byte{key1}},
		{name: "rotation", keys: [][]byte{key1, key2}, ring: [][]byte{key2, key1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				k, _, status, metrics := newTestKeyring(t, tc.keys...)

				if err := k.Wait(t.Context()); err != nil {
					t.Fatalf("Wait: %v", err)
				}

				if got := ringKeys(k); !slices.EqualFunc(got, tc.ring, bytes.Equal) {
					t.Errorf("keyring = %q, want %q", got, tc.ring)
				}
				if k.KeyID() != keyID(tc.ring[0]) {
					t.Errorf("KeyID = %q, want %q", k.KeyID(), keyID(tc.ring[0]))
				}
				if v, ok := metrics.keyInvalid(); !ok || v {
					t.Errorf("config_invalid{key} = %v (set %v), want false", v, ok)
				}
				if status.last() != "" {
					t.Errorf("condition reason %q is set with a valid key", status.last())
				}

				time.Sleep(testSwitchDelay - time.Second)
				k.reload()
				if k.KeyID() != keyID(tc.ring[0]) {
					t.Fatalf("KeyID = %q before the delay, want %q", k.KeyID(), keyID(tc.ring[0]))
				}

				time.Sleep(time.Second)
				k.reload()
				if got := ringKeys(k); len(got) != len(tc.keys) || !bytes.Equal(got[0], key1) {
					t.Errorf("keyring = %q after the delay, want the primary key of the Secret first", got)
				}
			})
		})
	}
}

// First start of the agent with a key of a wrong format: the InvalidKey
// signals at start, then the correct key appears in the Secret.
func TestKeyringFirstStartInvalidKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, dir, status, metrics := newTestKeyring(t, badKey, key2)

		done := make(chan error, 1)
		go func() { done <- k.Wait(t.Context()) }()

		time.Sleep(10 * time.Second)
		synctest.Wait()

		select {
		case err := <-done:
			t.Fatalf("Wait returned %v with a key that is not valid", err)
		default:
		}
		if k.Ring() != nil {
			t.Error("the keyring exists with a key that is not valid")
		}
		if status.last() != domain.ReasonInvalidKey {
			t.Errorf("condition reason = %q, want InvalidKey", status.last())
		}
		if v, _ := metrics.keyInvalid(); !v {
			t.Error("config_invalid{key} is not set")
		}

		writeKeys(t, dir, key1)
		time.Sleep(keyReloadInterval)
		synctest.Wait()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Wait: %v", err)
			}
		default:
			t.Fatal("Wait does not return after a valid key appears")
		}
		if v, _ := metrics.keyInvalid(); v {
			t.Error("config_invalid{key} stays set after a valid key appears")
		}
	})
}

func TestKeyringWaitStopsWithContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, _, _, _ := newTestKeyring(t, badKey)

		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()

		if err := k.Wait(ctx); err == nil {
			t.Fatal("Wait returned nil without a valid key")
		}
	})
}

// The agent stays on the current keyring.
func TestKeyringRejectsInvalidKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, dir, status, metrics := newTestKeyring(t, key1)
		if err := k.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		writeKeys(t, dir, badKey, key1)
		time.Sleep(testSwitchDelay + time.Second)
		k.reload()

		if got := ringKeys(k); len(got) != 1 || !bytes.Equal(got[0], key1) {
			t.Errorf("keyring = %q, want only the current key", got)
		}
		if v, _ := metrics.keyInvalid(); !v {
			t.Error("config_invalid{key} is not set")
		}
		if status.last() != "" {
			t.Errorf("condition reason = %q with a current keyring, want none", status.last())
		}

		writeKeys(t, dir, key1)
		k.reload()
		if v, ok := metrics.keyInvalid(); !ok || v {
			t.Error("config_invalid{key} stays set after a valid key appears")
		}
	})
}

func TestKeyringSkipsInvalidSecondaryKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, _, _, metrics := newTestKeyring(t, key1, badKey)
		if err := k.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		if got := ringKeys(k); len(got) != 1 || !bytes.Equal(got[0], key1) {
			t.Errorf("keyring = %q, want only the primary key", got)
		}
		if v, _ := metrics.keyInvalid(); !v {
			t.Error("config_invalid{key} is not set")
		}
	})
}

// The primary key switches after 2 min. A key change arrives without a restart.
func TestKeyringSwitchAfterDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, dir, _, _ := newTestKeyring(t, key1)
		if err := k.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		writeKeys(t, dir, key2, key1)
		if k.reload() {
			t.Fatal("switched at once")
		}
		// The new key is first an additional key.
		if got := ringKeys(k); len(got) != 2 || !bytes.Equal(got[0], key1) || !bytes.Equal(got[1], key2) {
			t.Fatalf("keyring = %q, want the old primary key and the new additional key", got)
		}

		time.Sleep(testSwitchDelay - time.Second)
		if k.reload() {
			t.Fatal("switched before the delay")
		}

		time.Sleep(time.Second)
		if !k.reload() {
			t.Fatal("did not switch after the delay")
		}
		if got := ringKeys(k); len(got) != 2 || !bytes.Equal(got[0], key2) {
			t.Errorf("keyring = %q, want the new primary key and the old key for decryption", got)
		}
		if k.KeyID() != keyID(key2) {
			t.Errorf("KeyID = %q, want the keyID of the new key", k.KeyID())
		}
	})
}

// The delay comes from KEY_SWITCH_DELAY.
func TestKeyringSwitchAfterConfiguredDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const delay = 30 * time.Second

		dir := filepath.Join(t.TempDir(), "memberlist-key")
		writeKeys(t, dir, key1)
		k := NewKeyring(dir, delay, &fakeStatus{}, &fakeMetrics{}, log.NewNop())
		if err := k.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		writeKeys(t, dir, key2, key1)
		k.reload()
		time.Sleep(delay - time.Second)
		if k.reload() {
			t.Fatal("switched before the delay")
		}

		time.Sleep(time.Second)
		if !k.reload() {
			t.Fatal("did not switch after the delay")
		}

		// The same delay applies to the removal of the old key.
		writeKeys(t, dir, key2)
		k.reload()
		time.Sleep(delay - time.Second)
		k.reload()
		if len(ringKeys(k)) != 2 {
			t.Fatal("removed the old key earlier than the delay after the switch")
		}

		time.Sleep(time.Second)
		k.reload()
		if got := ringKeys(k); len(got) != 1 || !bytes.Equal(got[0], key2) {
			t.Errorf("keyring = %q, want only the new key", got)
		}
	})
}

// The removal of the old key.
func TestKeyringRemoveOldKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, dir, _, _ := newTestKeyring(t, key1)
		if err := k.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		writeKeys(t, dir, key2, key1)
		k.reload()
		time.Sleep(testSwitchDelay)
		if !k.reload() {
			t.Fatal("did not switch after the delay")
		}

		// The hook removes the old key from the Secret.
		writeKeys(t, dir, key2)
		k.reload()
		time.Sleep(testSwitchDelay - time.Second)
		k.reload()
		if len(ringKeys(k)) != 2 {
			t.Fatal("removed the old key earlier than 2 min after the switch")
		}

		time.Sleep(time.Second)
		k.reload()
		if got := ringKeys(k); len(got) != 1 || !bytes.Equal(got[0], key2) {
			t.Errorf("keyring = %q, want only the new key", got)
		}
	})
}

// The old key leaves the Secret before the switch. The agent keeps it for the
// delay after the switch, because it encrypted with this key until the switch.
func TestKeyringRemoveOldKeyBeforeTheSwitch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, dir, _, _ := newTestKeyring(t, key1)
		if err := k.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		writeKeys(t, dir, key2, key1)
		k.reload()
		time.Sleep(time.Minute)
		writeKeys(t, dir, key2)
		k.reload()

		time.Sleep(testSwitchDelay - time.Minute)
		if !k.reload() {
			t.Fatal("did not switch after the delay")
		}

		time.Sleep(testSwitchDelay - time.Second)
		k.reload()
		if len(ringKeys(k)) != 2 {
			t.Fatal("removed the old key earlier than the delay after the switch")
		}

		time.Sleep(time.Second)
		k.reload()
		if got := ringKeys(k); len(got) != 1 || !bytes.Equal(got[0], key2) {
			t.Errorf("keyring = %q, want only the new key", got)
		}
	})
}

func TestKeyringKeepsKeysOfTheSecret(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		k, dir, _, _ := newTestKeyring(t, key1, key2)
		if err := k.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		time.Sleep(testSwitchDelay)
		k.reload()
		if len(ringKeys(k)) != 2 {
			t.Fatal("removed a key that is in the Secret")
		}

		// A second rotation while the first is not complete: the last
		// primary key of the Secret is the target.
		writeKeys(t, dir, key3, key2)
		k.reload()
		time.Sleep(testSwitchDelay)
		if !k.reload() || !bytes.Equal(ringKeys(k)[0], key3) {
			t.Fatal("did not switch to the last primary key of the Secret")
		}
	})
}

// ringNode is the keyring of an agent with its own key volume: kubelet updates
// the volumes on the nodes at different times.
type ringNode struct {
	name string
	dir  string
	k    *Keyring
}

func startRing(t *testing.T, name string, keys ...[]byte) *ringNode {
	t.Helper()

	dir := filepath.Join(t.TempDir(), name, "memberlist-key")
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		t.Fatal(err)
	}
	writeKeys(t, dir, keys...)
	k := NewKeyring(dir, testSwitchDelay, &fakeStatus{}, &fakeMetrics{}, log.NewNop())
	if err := k.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}

	return &ringNode{name: name, dir: dir, k: k}
}

// runRings reloads the keyrings each second for d, as the agents do. It reports
// each second when an agent does not have the primary key of another agent.
func runRings(t *testing.T, d time.Duration, nodes ...*ringNode) {
	t.Helper()

	for range int(d / time.Second) {
		time.Sleep(time.Second)
		for _, n := range nodes {
			n.k.reload()
		}
		for _, rx := range nodes {
			for _, tx := range nodes {
				primary := tx.k.Ring().GetPrimaryKey()
				if rx != tx && !slices.ContainsFunc(ringKeys(rx.k), func(k []byte) bool { return bytes.Equal(k, primary) }) {
					t.Errorf("%s does not decrypt %s", rx.name, tx.name)
				}
			}
		}
	}
}

// A planned rotation: the volume of b gets the new key 1 min later than the
// volume of a. An agent that starts in this time encrypts with the old key
// until the delay ends. All agents decrypt each other during the rotation.
func TestKeyringRotationWithANewAgent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b := startRing(t, "a", key1), startRing(t, "b", key1)
		writeKeys(t, a.dir, key2, key1)
		runRings(t, 20*time.Second, a, b)

		n := startRing(t, "n", key2, key1)
		runRings(t, 40*time.Second, a, b, n)

		writeKeys(t, b.dir, key2, key1)
		runRings(t, 3*time.Minute, a, b, n)

		for _, r := range []*ringNode{a, b, n} {
			writeKeys(t, r.dir, key2)
		}
		runRings(t, 5*time.Minute, a, b, n)
	})
}

// The old key stays in the Secret for a long time after a rotation. A new agent
// starts and encrypts with the old key for the delay. The administrator removes
// the old key in this time: the running agents keep it for the delay after they
// see the change, so they decrypt the new agent until it switches.
func TestKeyringOldKeyRemovedAfterANewStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := startRing(t, "a", key1)
		writeKeys(t, a.dir, key2, key1)
		runRings(t, 10*time.Minute, a)

		n := startRing(t, "n", key2, key1)
		runRings(t, time.Minute, a, n)

		writeKeys(t, a.dir, key2)
		writeKeys(t, n.dir, key2)
		runRings(t, 5*time.Minute, a, n)
		for _, r := range []*ringNode{a, n} {
			if got := ringKeys(r.k); len(got) != 1 || !bytes.Equal(got[0], key2) {
				t.Errorf("%s keyring = %q, want only the new key", r.name, got)
			}
		}
	})
}

// The keys of the Secret change places before the delay of a new agent ends.
// All agents decrypt each other and use the primary key of the Secret.
func TestKeyringKeysChangePlaces(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := startRing(t, "a", key1)
		writeKeys(t, a.dir, key2, key1)
		runRings(t, 30*time.Second, a)

		n := startRing(t, "n", key2, key1)
		runRings(t, 30*time.Second, a, n)

		writeKeys(t, a.dir, key1, key2)
		writeKeys(t, n.dir, key1, key2)
		runRings(t, 6*time.Minute, a, n)
		for _, r := range []*ringNode{a, n} {
			if !bytes.Equal(r.k.Ring().GetPrimaryKey(), key1) {
				t.Errorf("%s primary key = %q, want the primary key of the Secret", r.name, r.k.Ring().GetPrimaryKey())
			}
		}
	})
}
