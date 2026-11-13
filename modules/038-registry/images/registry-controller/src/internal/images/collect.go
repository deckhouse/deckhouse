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

package images

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/layout"
)

const (
	// ProxyPort is where the kube-rbac-proxy of a storage pod listens, on its node's address: the
	// same proxy the metrics are scraped through.
	ProxyPort = 4285

	// ClientSecret holds the authority the storage's certificates are issued by.
	ClientSecret = "registry-storage-client"

	// Namespace is where the storage runs.
	Namespace = "d8-system"

	// TokenFile is this pod's service account token, which the proxy authorizes.
	TokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	// maxIndexBytes bounds one answer: a store of thousands of images answers in a few megabytes.
	maxIndexBytes = 64 << 20
)

// DefaultTimeout bounds the wait for one replica. The answer comes from memory, so a replica that
// takes longer is not answering.
const DefaultTimeout = 10 * time.Second

// Collector asks every replica of the storage for its index.
type Collector struct {
	// Reader reads the storage, the nodes and the authority, uncached: the answers are wanted as the
	// cluster is now, and the secret is not in the manager's cache.
	Reader client.Reader

	// TokenFile is read on every collection, so that a rotated token is used. Empty means TokenFile.
	TokenFile string

	// Timeout bounds the wait for each replica. Zero means DefaultTimeout.
	Timeout time.Duration

	// Port is where the replicas' proxies listen. Zero means ProxyPort.
	Port int

	// Transport overrides the transport, for a test. Nil builds one trusting the storage's authority.
	Transport http.RoundTripper
}

// ErrNoCache is returned when the cache is turned off: there is no storage to ask.
var ErrNoCache = errors.New("the registry cache is turned off")

// Collect asks every replica the storage expects for its index, all at once. A replica that does not
// answer is in the result with its error, not a failure of the whole collection.
func (c *Collector) Collect(ctx context.Context) ([]Answer, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	if err := c.Reader.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrNoCache
		}
		return nil, fmt.Errorf("reading the storage: %w", err)
	}
	nodes, err := layout.ListStorageNodes(ctx, c.Reader)
	if err != nil {
		return nil, err
	}
	replicas := layout.CurrentReplicas(storage.Status.Replicas, nodes)

	httpClient, err := c.client(ctx)
	if err != nil {
		return nil, err
	}
	token, err := os.ReadFile(c.tokenFile())
	if err != nil {
		return nil, fmt.Errorf("reading the service account token: %w", err)
	}

	answers := make([]Answer, len(replicas))
	var wg sync.WaitGroup
	for i := range replicas {
		replica := replicas[i]
		answers[i].Node = replica.Node
		wg.Add(1)
		go func(answer *Answer) {
			defer wg.Done()
			answer.Index, answer.Err = c.ask(ctx, httpClient, strings.TrimSpace(string(token)), replica)
		}(&answers[i])
	}
	wg.Wait()
	return answers, nil
}

func (c *Collector) tokenFile() string {
	if c.TokenFile == "" {
		return TokenFile
	}
	return c.TokenFile
}

func (c *Collector) client(ctx context.Context) (*http.Client, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if c.Transport != nil {
		return &http.Client{Transport: c.Transport, Timeout: timeout}, nil
	}

	// The proxy serves the registry's own certificate, which names the node addresses and is issued
	// by the storage's authority: a token goes only to a replica that proves it is one.
	secret := &corev1.Secret{}
	if err := c.Reader.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: ClientSecret}, secret); err != nil {
		return nil, fmt.Errorf("reading the storage's authority: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(secret.Data["ca.crt"]) {
		return nil, errors.New("the storage's authority is not valid PEM")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

// ask asks one replica.
func (c *Collector) ask(
	ctx context.Context, httpClient *http.Client, token string, replica registryv1alpha1.StorageReplicaStatus,
) (*registryimagev1alpha1.ReplicaIndex, error) {
	host, _, err := net.SplitHostPort(replica.Address)
	if err != nil || host == "" {
		return nil, fmt.Errorf("the replica has not reported its address")
	}
	port := c.Port
	if port == 0 {
		port = ProxyPort
	}
	url := "https://" + net.JoinHostPort(host, strconv.Itoa(port)) + registryimagev1alpha1.IndexPath

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return nil, fmt.Errorf("%s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	index := &registryimagev1alpha1.ReplicaIndex{}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxIndexBytes)).Decode(index); err != nil {
		return nil, fmt.Errorf("reading the answer: %w", err)
	}
	if index.Node == "" {
		index.Node = replica.Node
	}
	return index, nil
}
