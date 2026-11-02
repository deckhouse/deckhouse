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
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
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
	// VMLabel marks a namespace of a project whose quota has keys of the vm family: the webhook of the
	// family is called there. Its value is LabelEnforced.
	VMLabel = "quota.projects.deckhouse.io/vm"
	// LBLabel marks a namespace of a project whose quota has keys of the lb family.
	LBLabel = "quota.projects.deckhouse.io/lb"

	// ConfigMapName is the ConfigMap Helm renders with the facts of the cluster the quota depends on.
	ConfigMapName = "quota-ledger-config"
	// ConfigNamespace is the namespace of the module.
	ConfigNamespace = "d8-multitenancy-manager"

	// SuspendConfigMapName is the ConfigMap an administrator creates to suspend the ledger.
	SuspendConfigMapName = "quota-ledger-suspend"
	// LedgerWebhookName is the ValidatingWebhookConfiguration of the ledger.
	LedgerWebhookName = "d8-multitenancy-manager-quota-ledger"
	// LedgerCertsSecretName is the Secret with the certificate of the ledger.
	LedgerCertsSecretName = "quota-ledger-certs"
	// LedgerLeaseName is the lease of the ledger.
	LedgerLeaseName = "multitenancy-manager-quota-ledger"
	// LedgerStateConfigMapName is the ConfigMap the leader of the ledger writes what its canary found
	// into.
	LedgerStateConfigMapName = "quota-ledger-state"
	// LedgerStateNotCalledKey lists, comma-separated, the API servers that write the status of the
	// canary quota without calling the webhook of the ledger.
	LedgerStateNotCalledKey = "webhookNotCalledBy"
	// LedgerStateSingleProbesKey holds, as JSON, when a leader last stepped down for each set of API
	// servers that alone did not reach it, so that the next leader does not step down for it too soon.
	LedgerStateSingleProbesKey = "singleProbes"

	configControlPlaneManagedKey = "controlPlaneManaged"
	configFamiliesKey            = "families"
	familyVirtualMachines        = "vm"

	// configTTL bounds how long a read of the ConfigMap is reused: every reconcile asks for it.
	configTTL = 5 * time.Second
)

// Config is what the quota depends on in the cluster, as Helm rendered it.
type Config struct {
	// ControlPlaneManaged is false on a cluster whose control plane Deckhouse does not manage; the sum
	// of a project is not enforced there.
	ControlPlaneManaged bool
	// Suspended is true while the quota-ledger-suspend ConfigMap is there: the ledger lets every write
	// through uncounted.
	Suspended bool
	// WebhookMissing is true where the sum is enforced and the webhook of the ledger is not configured,
	// or not with the CA of the certificate of the ledger.
	WebhookMissing bool
	// WebhookMisconfigured names the field of the webhook of the ledger that keeps writes of
	// d8-project-quota from reaching it, empty when none does.
	WebhookMisconfigured string
	// WebhookNotCalledBy are the API servers that the canary of the ledger found writing the status of
	// a quota without calling the webhook.
	WebhookNotCalledBy []string
	// LeaderChangedAt is when the current holder took the lease of the ledger, zero when unknown.
	LeaderChangedAt time.Time
	// Families are the families of objects the ledger counts, as the module renders them; nil when
	// the ConfigMap does not say.
	Families []string
}

// CountsVirtualMachines reports whether the ledger counts the virtual machines: the module that
// serves them is enabled. Its resources may still be served after it is disabled, and nothing counts
// them then.
func (c Config) CountsVirtualMachines() bool {
	return c.Families == nil || slices.Contains(c.Families, familyVirtualMachines)
}

// MetricMode is the value of the mode label of the enforcement metric.
func (c Config) MetricMode() string {
	switch {
	case !c.ControlPlaneManaged:
		return "NotSupported"
	case c.Suspended:
		return "Suspended"
	case c.WebhookMissing:
		return "WebhookMissing"
	case c.WebhookMisconfigured != "":
		return "WebhookMisconfigured"
	}
	return "Enforce"
}

// ParseConfig reads the ConfigMap. Without it the control plane counts as managed: the policies Helm
// renders with the ConfigMap are not there yet either.
func ParseConfig(cm *corev1.ConfigMap) Config {
	if cm == nil {
		return Config{ControlPlaneManaged: true}
	}
	cfg := Config{ControlPlaneManaged: cm.Data[configControlPlaneManagedKey] != "false"}
	if families, ok := cm.Data[configFamiliesKey]; ok {
		cfg.Families = []string{}
		for _, family := range strings.Split(families, ",") {
			if family = strings.TrimSpace(family); family != "" {
				cfg.Families = append(cfg.Families, family)
			}
		}
	}
	return cfg
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
	cfg := ParseConfig(cm)
	if err := r.readLedger(ctx, &cfg); err != nil {
		return Config{}, err
	}
	r.cached, r.readAt = cfg, now()
	return r.cached, nil
}

// Invalidate makes the next Read go to the API: an object it reads has changed.
func (r *ConfigReader) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readAt = time.Time{}
}

// IsConfigObject reports whether an object of the namespace and the name is one Read reads: a
// change of it is a change of the enforcement of every quota.
func IsConfigObject(namespace, name string) bool {
	switch {
	case namespace == "" && name == LedgerWebhookName:
		return true
	case namespace != ConfigNamespace:
		return false
	}
	return name == ConfigMapName || name == SuspendConfigMapName || name == LedgerStateConfigMapName || name == LedgerCertsSecretName
}

// readLedger reads the state of the ledger: the suspend ConfigMap, the webhook and its CA, and when
// the lease last changed hands.
func (r *ConfigReader) readLedger(ctx context.Context, cfg *Config) error {
	suspend := new(corev1.ConfigMap)
	switch err := r.Reader.Get(ctx, client.ObjectKey{Namespace: ConfigNamespace, Name: SuspendConfigMapName}, suspend); {
	case err == nil:
		cfg.Suspended = true
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("get the %s ConfigMap: %w", SuspendConfigMapName, err)
	}

	lease := new(coordinationv1.Lease)
	switch err := r.Reader.Get(ctx, client.ObjectKey{Namespace: ConfigNamespace, Name: LedgerLeaseName}, lease); {
	case err == nil:
		if lease.Spec.AcquireTime != nil {
			cfg.LeaderChangedAt = lease.Spec.AcquireTime.Time
		}
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("get the %s lease: %w", LedgerLeaseName, err)
	}

	if !cfg.ControlPlaneManaged {
		return nil
	}
	state := new(corev1.ConfigMap)
	switch err := r.Reader.Get(ctx, client.ObjectKey{Namespace: ConfigNamespace, Name: LedgerStateConfigMapName}, state); {
	case err == nil:
		if notCalled := state.Data[LedgerStateNotCalledKey]; notCalled != "" {
			cfg.WebhookNotCalledBy = strings.Split(notCalled, ",")
		}
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("get the %s ConfigMap: %w", LedgerStateConfigMapName, err)
	}

	webhook := new(admissionregistrationv1.ValidatingWebhookConfiguration)
	switch err := r.Reader.Get(ctx, client.ObjectKey{Name: LedgerWebhookName}, webhook); {
	case apierrors.IsNotFound(err):
		cfg.WebhookMissing = true
		return nil
	case err != nil:
		return fmt.Errorf("get the %s webhook: %w", LedgerWebhookName, err)
	}
	secret := new(corev1.Secret)
	if err := r.Reader.Get(ctx, client.ObjectKey{Namespace: ConfigNamespace, Name: LedgerCertsSecretName}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			cfg.WebhookMissing = true
			return nil
		}
		return fmt.Errorf("get the %s Secret: %w", LedgerCertsSecretName, err)
	}
	entry := ledgerWebhook(webhook)
	cfg.WebhookMissing = entry == nil || !bytes.Equal(entry.ClientConfig.CABundle, secret.Data["ca.crt"])
	if entry != nil {
		cfg.WebhookMisconfigured = webhookDrift(entry)
	}
	return nil
}
