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

package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// NamespaceLabel marks a namespace of a project that has a ProjectQuota. The admission policies of
	// the quota select namespaces by it: a namespace the controller creates for a ProjectNamespace
	// gets LabelPending in the request that creates it, and a namespace gets LabelEnforced once its
	// d8-project-quota ResourceQuota is calculated.
	NamespaceLabel = "projects.deckhouse.io/quota"
	// LabelPending: the d8-project-quota ResourceQuota of the namespace is not there or not calculated
	// yet, and the guard policy refuses the objects of the namespace until it is.
	LabelPending = "pending"
	// LabelEnforced: the quota applies in the namespace.
	LabelEnforced = "enforced"

	// ConfigMapName is the ConfigMap Helm renders with the facts of the cluster the quota depends on.
	ConfigMapName = "quota-ledger-config"
	// ConfigNamespace is the namespace of the module.
	ConfigNamespace = "d8-multitenancy-manager"

	configControlPlaneManagedKey = "controlPlaneManaged"

	// configTTL bounds how long a read of the ConfigMap is reused: every reconcile asks for it.
	configTTL = 5 * time.Second
)

// Config is what the quota depends on in the cluster, as Helm rendered it.
type Config struct {
	// ControlPlaneManaged is false on a cluster whose control plane Deckhouse does not manage; the sum
	// of a project is not enforced there.
	ControlPlaneManaged bool
}

// MetricMode is the value of the mode label of the enforcement metric.
func (c Config) MetricMode() string {
	if !c.ControlPlaneManaged {
		return "NotSupported"
	}
	return "Enforce"
}

// ParseConfig reads the ConfigMap. Without it the control plane counts as managed: the policies Helm
// renders with the ConfigMap are not there yet either.
func ParseConfig(cm *corev1.ConfigMap) Config {
	if cm == nil {
		return Config{ControlPlaneManaged: true}
	}
	return Config{ControlPlaneManaged: cm.Data[configControlPlaneManagedKey] != "false"}
}

// ConfigReader reads the ConfigMap through the API, reusing a read for a few seconds. The reader is
// the direct API reader: the cache of the manager would watch every ConfigMap of the cluster for one.
type ConfigReader struct {
	Reader client.Reader
	// Now is time.Now unless a test sets it.
	Now func() time.Time

	mu     sync.Mutex
	cached Config
	readAt time.Time
}

// Read returns what the quota depends on in the cluster.
func (r *ConfigReader) Read(ctx context.Context) (Config, error) {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.readAt.IsZero() && now().Sub(r.readAt) < configTTL {
		return r.cached, nil
	}

	cm := new(corev1.ConfigMap)
	err := r.Reader.Get(ctx, client.ObjectKey{Namespace: ConfigNamespace, Name: ConfigMapName}, cm)
	switch {
	case apierrors.IsNotFound(err):
		cm = nil
	case err != nil:
		return Config{}, fmt.Errorf("get the %s/%s ConfigMap: %w", ConfigNamespace, ConfigMapName, err)
	}
	r.cached, r.readAt = ParseConfig(cm), now()
	return r.cached, nil
}
