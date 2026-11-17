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

package dexauthenticator

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/naming"
)

const (
	// InstanceAppLabel and InstanceLabel are the labels of the pods of an instance.
	InstanceAppLabel = "app"
	InstanceAppValue = "shared-dex-authenticator"
	InstanceLabel    = "instance"

	// instancePort is the port the proxy of an instance listens on, and statusPath where it tells
	// which table it serves (patch 018 of the image).
	instancePort = "8443"
	statusPath   = "/client-table"

	statusTimeout = 3 * time.Second
	// statusTTL is how long an answer of the pods is used: the objects of one instance are
	// reconciled in a row, and one question answers them all.
	statusTTL = 2 * time.Second
	// statusBodyLimit bounds the answer of a pod: the identifiers of the refused records of a
	// full table.
	statusBodyLimit = 4 << 20

	// statusIDSize is the length of the identifier of a record in the answer of a pod, in bytes
	// before it is written in hex (patch 018 of the image).
	statusIDSize = 16
)

// LoadStatus is the table the pods of an instance serve.
type LoadStatus struct {
	// Generation is the oldest generation a pod of the instance serves.
	Generation int64
	// Failed are the records a pod refused or could not build a client from, by the identifier
	// the pod gives them (statusID): its answer names no object, since every pod of the cluster
	// reaches the port it answers on.
	Failed []string
}

// statusID is the identifier a pod of an instance gives the record in its answer: the HMAC of
// namespace/name under the cookie secret of the record, as patch 018 of the image computes it.
// Only the writer of the record, which has the secret, finds its record by it.
func statusID(record *Record) string {
	mac := hmac.New(sha256.New, []byte(record.CookieSecret))
	mac.Write([]byte(record.key().String()))
	return hex.EncodeToString(mac.Sum(nil)[:statusIDSize])
}

// refused reports whether a pod of the instance refused record.
func (s LoadStatus) refused(record *Record) bool {
	return record != nil && slices.Contains(s.Failed, statusID(record))
}

// InstanceStatus tells which table an instance serves.
type InstanceStatus interface {
	Loaded(ctx context.Context, instance string) (LoadStatus, error)
}

// errNoPods tells that an instance has no pod that could have loaded a table.
var errNoPods = errors.New("the instance has no running pod")

// podStatus asks the pods of an instance. A table gets to a pod when kubelet syncs the volume of
// its Secret, up to a minute and more after the write, and the controller has no right to change
// the pods to hurry it: such a right would let it replace the image of any pod of the module.
type podStatus struct {
	client client.Client
	http   *http.Client
	now    func() time.Time
	// url is the address of the status of the pod with the given IP.
	url func(ip string) string

	mu     sync.Mutex
	cached map[string]cachedStatus
}

type cachedStatus struct {
	status LoadStatus
	err    error
	at     time.Time
}

// NewPodStatus returns the InstanceStatus that asks the pods of the instances; c reads the pods
// from the cache.
func NewPodStatus(c client.Client) InstanceStatus {
	return &podStatus{
		client: c,
		now:    time.Now,
		url:    func(ip string) string { return "https://" + net.JoinHostPort(ip, instancePort) + statusPath },
		cached: map[string]cachedStatus{},
		http: &http.Client{
			Timeout: statusTimeout,
			Transport: &http.Transport{
				// A pod serves a certificate it makes for itself at its start, as the dex-authenticator
				// of an object did, and the ingress controllers do not check it either. The answer is a
				// generation number: a forged one makes a route appear early or late, nothing more.
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, //nolint:gosec // see above
				MaxIdleConnsPerHost: 1,
				IdleConnTimeout:     30 * time.Second,
			},
			// The address is the one of a pod; an answer that sends elsewhere is not followed.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Loaded asks every running pod of the instance and returns the oldest generation among them. An
// instance without a pod that answers, or with a ready pod that does not, has loaded nothing. A
// pod that is not ready and does not answer is left out: the Service of the instance sends it
// nothing.
func (p *podStatus) Loaded(ctx context.Context, instance string) (LoadStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cached, ok := p.cached[instance]; ok && p.now().Sub(cached.at) < statusTTL {
		return cached.status, cached.err
	}
	status, err := p.ask(ctx, instance)
	p.cached[instance] = cachedStatus{status: status, err: err, at: p.now()}
	return status, err
}

func (p *podStatus) ask(ctx context.Context, instance string) (LoadStatus, error) {
	pods := &corev1.PodList{}
	if err := p.client.List(ctx, pods, client.InNamespace(naming.DexNamespace),
		client.MatchingLabels{InstanceAppLabel: InstanceAppValue, InstanceLabel: instance}); err != nil {
		return LoadStatus{}, fmt.Errorf("list the pods of %s: %w", instance, err)
	}

	var out LoadStatus
	asked := 0
	for i := range pods.Items {
		pod := &pods.Items[i]
		// A pod that has not started yet mounts the present Secret when it does.
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
			continue
		}
		status, err := p.askPod(ctx, pod.Status.PodIP)
		if err != nil {
			if !isReady(pod) {
				// It gets no request, and reads the table anew when its proxy starts.
				continue
			}
			return LoadStatus{}, fmt.Errorf("pod %s: %w", pod.Name, err)
		}
		if asked == 0 || status.Generation < out.Generation {
			out.Generation = status.Generation
		}
		for _, key := range status.Failed {
			if !slices.Contains(out.Failed, key) {
				out.Failed = append(out.Failed, key)
			}
		}
		asked++
	}
	if asked == 0 {
		return LoadStatus{}, fmt.Errorf("%s: %w", instance, errNoPods)
	}
	return out, nil
}

func isReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (p *podStatus) askPod(ctx context.Context, ip string) (LoadStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url(ip), http.NoBody)
	if err != nil {
		return LoadStatus{}, fmt.Errorf("build the request: %w", err)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return LoadStatus{}, fmt.Errorf("ask for the table: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return LoadStatus{}, fmt.Errorf("ask for the table: status %d", resp.StatusCode)
	}
	var answer struct {
		Generation int64    `json:"generation"`
		Failed     []string `json:"failed"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, statusBodyLimit)).Decode(&answer); err != nil {
		return LoadStatus{}, fmt.Errorf("read the answer: %w", err)
	}
	return LoadStatus{Generation: answer.Generation, Failed: answer.Failed}, nil
}
