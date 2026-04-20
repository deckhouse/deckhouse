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

package upstream

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/transport"

	"kubernetes-api-proxy/pkg/kubernetes"
)

// Upstream represents a single Kubernetes API server backend (host:port)
// together with HTTP client settings used for health checking.
type Upstream struct {
	Addr string // host:port

	configGetter kubernetes.ClusterConfigGetter
}

// NewUpstream creates a new Upstream instance for the given Addr.
func NewUpstream(address string) *Upstream {
	return &Upstream{
		Addr: address,
	}
}

func (u *Upstream) UseKubernetesConfigGetter(getter kubernetes.ClusterConfigGetter) {
	u.configGetter = getter
}

// HealthCheck performs a /readyz probe against the upstream over HTTPS and
// returns an observed latency tier along with an error (if any).
//
// The caller can use the returned Tier to influence selection priority.
func (u *Upstream) HealthCheck(ctx context.Context) (Tier, error) {
	start := time.Now()
	err := u.healthCheck(ctx)
	elapsed := time.Since(start)
	tier, terr := calcTier(err, elapsed)

	return tier, terr
}

// healthCheck executes a single HTTP GET to /readyz and interprets the
// response. A non-200 status is treated as an error.
func (u *Upstream) healthCheck(ctx context.Context) error {
	url := "https://" + u.Addr + "/readyz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	kubernetesConfig, err := u.configGetter()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %s", err)
	}

	if kubernetesConfig.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+kubernetesConfig.BearerToken)
	}

	transportConfig, err := kubernetesConfig.TransportConfig()
	if err != nil {
		return err
	}

	tlsConfig, err := transport.TLSConfigFor(transportConfig)
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz endpoint returned status: %d", resp.StatusCode)
	}

	return nil
}

// Address returns the upstream Addr in host:port form.
func (u *Upstream) Address() string {
	return u.Addr
}

var tierBucket = []time.Duration{
	0,
	time.Millisecond / 10,
	time.Millisecond,
	10 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
}

// calcTier maps the measured latency to a discrete Tier bucket. If err is not
// nil, the caller is expected to keep the previous tier value.
func calcTier(err error, elapsed time.Duration) (Tier, error) {
	if err != nil {
		// preserve old tier
		return -1, err
	}

	for i := len(tierBucket) - 1; i >= 0; i-- {
		if elapsed >= tierBucket[i] {
			return Tier(i), nil
		}
	}

	return Tier(len(tierBucket)), err
}
