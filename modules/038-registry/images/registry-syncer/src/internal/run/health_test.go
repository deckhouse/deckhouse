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

package run

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/report"
)

// answeringRegistry is a TLS listener answering the API root the way a registry does — 401, asking
// for a token — and the authority that signed its certificate, generated for the test.
func answeringRegistry(t *testing.T) (address, authority string) {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	return parsed.Host, string(certificate)
}

// silentRegistry accepts connections and never says a word on them: the registry a dial calls alive.
func silentRegistry(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = connection.Close() })
		}
	}()
	return listener.Addr().String()
}

// TestTheRegistryProbeWantsAnAnswer: any HTTP answer from both listeners is a registry that serves; a
// port that is closed, or one that accepts and says nothing, is not.
func TestTheRegistryProbeWantsAnAnswer(t *testing.T) {
	serving, authority := answeringRegistry(t)
	write, _ := answeringRegistry(t)

	loop, _, _ := newLoop(t, true, nil)
	loop.LocalCA = authority
	loop.LocalAddress = serving
	loop.WriteAddress = write
	assert.NoError(t, loop.probeRegistry(context.Background()), "401 is an answer")

	t.Run("the write endpoint closed", func(t *testing.T) {
		closed := closedAddress(t)
		loop.WriteAddress = closed

		err := loop.probeRegistry(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), closed, "the error names the listener that failed")
	})

	t.Run("accepting, and answering nothing", func(t *testing.T) {
		loop.WriteAddress = silentRegistry(t)

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		assert.Error(t, loop.probeRegistry(ctx), "what a dial calls alive")
	})

	t.Run("a certificate the copy would not trust either", func(t *testing.T) {
		loop.WriteAddress = write
		loop.LocalCA = ""

		assert.Error(t, loop.probeRegistry(context.Background()))
	})
}

// certificateRegistry is answeringRegistry behind a listener that refuses a handshake without a client
// certificate, as a write endpoint that asks for one does, with the pair it admits and a count of the
// requests that got past the handshake. The key pair is generated for the test.
//
// Its server certificate is the one every httptest server is given, so the authority from
// answeringRegistry trusts it too, as in TestTheRegistryProbeWantsAnAnswer.
func certificateRegistry(t *testing.T) (address string, pair tls.Certificate, admitted *atomic.Int32) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "registry-syncer"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	issued, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	clients := x509.NewCertPool()
	clients.AddCert(issued)

	admitted = &atomic.Int32{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		admitted.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	return parsed.Host, tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, admitted
}

// TestTheRegistryProbeAsksAsTheFillDoes: the probe is built on the transport the fill's clients are —
// see localTransport — so whatever a listener asks of a client, the probe gives too. Built on a transport
// of its own, it was refused by a write endpoint that requires a client certificate while the fill was
// admitted, and five minutes later every replica reported its registry as not answering, which the
// election reads as a reason to move the lease.
func TestTheRegistryProbeAsksAsTheFillDoes(t *testing.T) {
	serving, authority := answeringRegistry(t)
	write, pair, admitted := certificateRegistry(t)

	loop, _, _ := newLoop(t, true, nil)
	loop.LocalCA = authority
	loop.LocalAddress = serving
	loop.WriteAddress = write
	require.Error(t, loop.probeRegistry(context.Background()), "the write endpoint asks for a certificate")

	loop.LocalTransport = func() (*http.Transport, error) {
		transport, err := fill.Transport(authority)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{pair}
		return transport, nil
	}
	require.NoError(t, loop.probeRegistry(context.Background()), "the probe presents what the loop's transport carries")
	probed := admitted.Load()
	assert.Positive(t, probed)

	destination, err := loop.writeRegistry()
	require.NoError(t, err)
	registry, err := name.NewRegistry(destination.Address)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = remote.Catalog(ctx, registry, destination.Options...) // 401, past the handshake
	assert.Greater(t, admitted.Load(), probed, "and so does the fill's client, over the same transport")
}

// TestASilenceIsReportedAndEndsWhenTheRegistryAnswers: the first failure only starts the series, one
// that has lasted registryDownReported is reported, and an answer ends it — the only thing that does.
func TestASilenceIsReportedAndEndsWhenTheRegistryAnswers(t *testing.T) {
	loop, _, _ := newLoop(t, true, nil)
	loop.LocalAddress = closedAddress(t)
	loop.WriteAddress = loop.LocalAddress
	loop.configured.Store(true)

	assert.Empty(t, loop.checkHealth(context.Background()), "a registry that has just gone quiet may be restarting")
	assert.NotZero(t, loop.registryDownSince.Load(), "but the silence is timed from here")

	loop.registryDownSince.Store(time.Now().Add(-registryDownReported).UnixNano())
	found := loop.checkHealth(context.Background())
	assert.Contains(t, found, "this replica's registry is not answering")
	assert.Equal(t, found, loop.HealthError())

	serving, authority := answeringRegistry(t)
	loop.LocalCA = authority
	loop.LocalAddress = serving
	loop.WriteAddress = serving

	assert.Empty(t, loop.checkHealth(context.Background()))
	assert.Empty(t, loop.HealthError())
	assert.Zero(t, loop.registryDownSince.Load(), "the answer ends the silence")
}

// TestNoSilenceIsTimedBeforeTheRegistryIsConfigured: the wrapper starts the registry only once a pass
// has written its configuration, and a pass does that only after it has read the storage object. A pod
// that starts while the API server is away does not count those minutes as the registry's silence, and
// does not report it the moment the registry begins its walk of the store.
func TestNoSilenceIsTimedBeforeTheRegistryIsConfigured(t *testing.T) {
	loop, _, _ := newLoop(t, true, nil)
	loop.LocalAddress = closedAddress(t)
	loop.WriteAddress = loop.LocalAddress

	for range 3 {
		assert.Empty(t, loop.checkHealth(context.Background()))
	}
	assert.Zero(t, loop.registryDownSince.Load(), "not configured: nothing to ask, nothing to time")

	loop.configured.Store(true)
	assert.Empty(t, loop.checkHealth(context.Background()))
	assert.NotZero(t, loop.registryDownSince.Load(), "configured: the silence is timed from here")
}

// TestAStoreThatRefusesWritesIsReported: a filesystem gone read-only is reported once it has refused
// for storeFailingReported, and stops being reported when it takes a write again.
func TestAStoreThatRefusesWritesIsReported(t *testing.T) {
	refusal := error(syscall.EROFS)
	loop, _, _ := newLoop(t, true, nil)
	loop.DataDir = t.TempDir()
	loop.StoreProbe = func(string) error { return refusal }

	assert.Empty(t, loop.checkHealth(context.Background()), "the first refusal starts the series")
	require.False(t, loop.storeFailingSince.IsZero())

	loop.storeFailingSince = time.Now().Add(-storeFailingReported)
	found := loop.checkHealth(context.Background())
	assert.Contains(t, found, "this replica's store does not accept writes")
	assert.Contains(t, found, syscall.EROFS.Error())

	refusal = nil
	assert.Empty(t, loop.checkHealth(context.Background()))
	assert.True(t, loop.storeFailingSince.IsZero())
}

// TestAFullStoreIsNotAFailingOne: the registry refuses writes long before the disk runs out, and a store
// at its budget or at the reserve is reported as that already. Read as a failure, it would take the
// lease off a leader whose only problem is room.
func TestAFullStoreIsNotAFailingOne(t *testing.T) {
	for _, refusal := range []error{syscall.ENOSPC, syscall.EDQUOT, &fs.PathError{Op: "write", Err: syscall.ENOSPC}} {
		loop, _, _ := newLoop(t, true, nil)
		loop.DataDir = t.TempDir()
		loop.StoreProbe = func(string) error { return refusal }
		loop.storeFailingSince = time.Now().Add(-time.Hour)

		assert.Empty(t, loop.checkHealth(context.Background()), "%v", refusal)
		assert.True(t, loop.storeFailingSince.IsZero(), "%v", refusal)
	}
}

// TestAHungWriteIsAFailureAndIsNotRepeated: a write stuck in the kernel cannot be interrupted, so the
// probe stops waiting for it rather than for the disk — and does not start another on every round
// while the first one hangs.
func TestAHungWriteIsAFailureAndIsNotRepeated(t *testing.T) {
	release := make(chan struct{})
	var started atomic.Int32

	loop, _, _ := newLoop(t, true, nil)
	loop.DataDir = t.TempDir()
	loop.StoreProbeTimeout = 20 * time.Millisecond
	loop.StoreProbe = func(string) error {
		started.Add(1)
		<-release
		return nil
	}

	err := loop.probeStoreWrite(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not finished")

	loop.storeProbeStarted = loop.storeProbeStarted.Add(-time.Minute) // a round later, and hanging still
	again := loop.probeStoreWrite(context.Background())
	require.Error(t, again, "still hanging: still a failure")
	assert.Equal(t, err.Error(), again.Error(),
		"in the same words, or every round would be a new status write and the watch could not take it back")
	assert.EqualValues(t, 1, started.Load(), "and still the same write")

	close(release)
	require.Eventually(t, func() bool { return loop.probeStoreWrite(context.Background()) == nil },
		time.Second, 10*time.Millisecond, "returned, it is the answer")
	assert.EqualValues(t, 1, started.Load())

	assert.NoError(t, loop.probeStoreWrite(context.Background()))
	assert.EqualValues(t, 2, started.Load(), "and the next round writes again")
}

// TestTheStoreProbeLeavesNothingBehind: what the registry does to a blob, in miniature, and gone again.
func TestTheStoreProbeLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, probeStore(dir))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)

	if os.Geteuid() == 0 {
		t.Skip("a read-only directory does not refuse root")
	}
	readOnly := filepath.Join(dir, "read-only")
	require.NoError(t, os.Mkdir(readOnly, 0o500))
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })
	assert.Error(t, probeStore(readOnly))
}

// TestTheHealthWatchPublishesWhileThePassCannot is the leader on a disk stuck in the kernel. Its pass
// walks the store before it publishes and so never publishes; the status keeps its last clean report,
// full and without an error, and the elector — which does not wait for passes — renews its lease. The
// watch publishes the failure itself, touching nothing but the error and healthError, and takes it back
// once the disk answers.
//
// No pass runs beside it here, which is the case: the watch shares no lock and no channel with the pass,
// so a pass that never returns is, to the watch, a pass that does not exist.
func TestTheHealthWatchPublishesWhileThePassCannot(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.SafeToDropUpstream = true
	storage.Status.AllReplicasFull = true
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
		Full: true, VerifiedDigests: 459, VerifiedSet: "set",
	}}

	release := make(chan struct{})
	loop, c, _ := newLoop(t, true, storage)
	loop.DataDir = t.TempDir()
	loop.StoreProbeTimeout = 10 * time.Millisecond
	loop.StoreProbe = func(string) error {
		<-release
		return nil
	}
	// The disk has refused for longer than it is given already.
	loop.storeFailingSince = time.Now().Add(-storeFailingReported)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go loop.watchHealth(ctx, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		return replicaOf(t, c, "master-0").Error != ""
	}, 5*time.Second, 20*time.Millisecond)

	replica := replicaOf(t, c, "master-0")
	assert.Contains(t, replica.Error, "this replica's store does not accept writes")
	assert.Equal(t, replica.Error, replica.HealthError, "and said where the other replicas tell it from a failed copy")
	assert.True(t, replica.Full, "only the error is touched: completeness comes back with the disk")
	assert.Equal(t, "set", replica.VerifiedSet)
	assert.EqualValues(t, 459, replica.VerifiedDigests)

	stored := &registryv1alpha1.RegistryStorage{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: registryv1alpha1.SingletonName}, stored))
	assert.False(t, stored.Status.SafeToDropUpstream, "a leader in trouble takes the permission back in the same write")
	assert.True(t, stored.Status.AllReplicasFull,
		"and leaves what the replicas hold alone, or the controller would put it back after every round")

	close(release)
	require.Eventually(t, func() bool {
		return replicaOf(t, c, "master-0").Error == ""
	}, 5*time.Second, 20*time.Millisecond, "the disk answers again, and the error goes")
	assert.Empty(t, replicaOf(t, c, "master-0").HealthError)
	assert.True(t, replicaOf(t, c, "master-0").Full)
}

// TestTheFirstRoundTakesBackAStaleHealthError: only the watch writes healthError, and a pass keeps it, so
// one left by an earlier run of this process — or by the pod before it — is the watch's to take back, on
// its first round, healthy as it finds things. Left standing, it would keep a replica that serves out of
// the running whenever nobody is full and nobody leads. The pass's error is not the watch's and stays.
func TestTheFirstRoundTakesBackAStaleHealthError(t *testing.T) {
	const stale = "this replica's store does not accept writes: read-only file system"
	const copyFailed = "discovering what to copy: reading the image set of pr1110: " +
		"the image set could not be read from 10.12.4.204:5001"

	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleFollower, VerifiedDigests: 322,
		Error: copyFailed, HealthError: stale,
	}}
	loop, c, _ := newLoop(t, true, storage)
	loop.DataDir = t.TempDir()
	loop.StoreProbe = func(string) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go loop.watchHealth(ctx, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		return replicaOf(t, c, "master-0").HealthError == ""
	}, 5*time.Second, 20*time.Millisecond, "a stale healthError is taken back")
	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, copyFailed, replica.Error, "the pass's error is the pass's")
	assert.EqualValues(t, 322, replica.VerifiedDigests)
}

// TestAPassDoesNotEraseTheHealthError: a pass that went well — an idle one above all, which looks at
// nothing — does not know the registry answers, and publishing without the error would hand a hung
// leader its lease back every half a minute.
func TestAPassDoesNotEraseTheHealthError(t *testing.T) {
	local := startRegistry(t)
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: "upstream.invalid", Path: "/deckhouse/ee",
			},
		},
	})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader, Full: true, VerifiedDigests: 333,
	}}

	loop, c, _ := newLoop(t, true, storage)
	loop.LocalAddress = local
	loop.WriteAddress = local
	loop.DataDir = t.TempDir()
	loop.healthErr = "this replica's registry is not answering: 10.0.0.1:5003: context deadline exceeded"

	require.NoError(t, loop.once(context.Background()))

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, loop.HealthError(), replica.Error)
	assert.True(t, replica.Full, "and what the replica holds is left as it was")
	assert.EqualValues(t, 333, replica.VerifiedDigests)

	loop.healthErr = ""
	require.NoError(t, loop.once(context.Background()))
	assert.Empty(t, replicaOf(t, c, "master-0").Error, "a pass with nothing standing reports nothing")
}

// TestProgressDoesNotClearTheHealthError: a pass under way clears the previous pass's error, not the
// health watch's.
func TestProgressDoesNotClearTheHealthError(t *testing.T) {
	storage := storageWith(registryv1alpha1.RegistryStorageSpec{})
	storage.Status.Replicas = []registryv1alpha1.StorageReplicaStatus{{
		Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader,
		Error: "this replica's store does not accept writes: read-only file system",
	}}
	loop, c, _ := newLoop(t, true, storage)
	loop.healthErr = "this replica's store does not accept writes: read-only file system"

	onProgress := loop.progress(context.Background(),
		report.State{Node: "master-0", Role: registryv1alpha1.ReplicaRoleLeader}, "fill")
	onProgress(1, 459)

	replica := replicaOf(t, c, "master-0")
	assert.Equal(t, loop.HealthError(), replica.Error)
	assert.EqualValues(t, 1, replica.VerifiedDigests)
}
