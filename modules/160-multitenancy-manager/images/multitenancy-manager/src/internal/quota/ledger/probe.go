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

package ledger

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// SuspendConfigMap is the ConfigMap in the namespace of the module whose presence suspends the
	// ledger: every write passes uncounted. An administrator creates and deletes it.
	SuspendConfigMap = "quota-ledger-suspend"
	// WakeAnnotation is the annotation the ledger writes on the quota of a namespace to wake it.
	WakeAnnotation = "quota-ledger.deckhouse.io/wake-up"
	// apiServerName is the name the certificate of every API server carries.
	apiServerName = "kubernetes.default.svc"
	probeTimeout  = 2 * time.Second
)

// apiServerClients sends the canary writes through one API server each, past the Service.
type apiServerClients struct {
	config *rest.Config

	mu      sync.Mutex
	clients map[string]kubernetes.Interface
}

func (c *apiServerClients) client(server string) (kubernetes.Interface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cs, ok := c.clients[server]; ok {
		return cs, nil
	}
	config := rest.CopyConfig(c.config)
	config.Host = "https://" + server
	config.TLSClientConfig.ServerName = apiServerName
	config.Timeout = probeTimeout
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create a client of %s: %w", server, err)
	}
	c.clients[server] = cs
	return cs, nil
}

// Probe writes the status of the canary quota as a dry run through the API server, with the nonce in
// an annotation the webhook reads.
func (c *apiServerClients) Probe(ctx context.Context, server, nonce string) error {
	cs, err := c.client(server)
	if err != nil {
		return err
	}
	quotas := cs.CoreV1().ResourceQuotas(Namespace)
	rq, err := quotas.Get(ctx, CanaryName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get the canary quota through %s: %w", server, err)
	}
	if rq.Annotations == nil {
		rq.Annotations = map[string]string{}
	}
	rq.Annotations[CanaryNonceAnnotation] = nonce
	if _, err := quotas.UpdateStatus(ctx, rq, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}}); err != nil {
		return fmt.Errorf("write the canary quota through %s: %w", server, err)
	}
	return nil
}
