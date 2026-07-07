/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package containerdintegrityconfigurator

//nolint:goimports,gci
import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"

	"github.com/BurntSushi/toml"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

const (
	integrityConfigDir  = "/etc/containerd/integrity"
	integrityConfigFile = "integrity.toml"
)

var _ reconcile.Reconciler = (*reconciler)(nil)

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

type desiredConfig struct {
	Namespaces []string
	CACerts    []string
}

type writeResult struct {
	Updated      bool
	Removed      bool
	Namespaces   []string
	CACertsCount int
}

type integrityTOML struct {
	Namespaces []string `toml:"namespaces"`
	CACerts    []string `toml:"ca_certs"`
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies/status,verbs=get

func (r *reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	return r.reconcileIntegrityConfig(ctx)
}

func (r *reconciler) reconcileIntegrityConfig(ctx context.Context) (reconcile.Result, error) {
	policies, err := r.getContainerdIntegrityPolicies(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get ContainerdIntegrityPolicies: %w", err)
	}

	desired := makeDesiredConfig(policies)

	writeResult, err := r.writeIntegrityConfig(desired)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("write containerd integrity config: %w", err)
	}

	switch {
	case writeResult.Removed:
		log.FromContext(ctx).Info("Removed containerd integrity config")
	case writeResult.Updated:
		log.FromContext(ctx).Info(
			"Updated containerd integrity config",
			"namespaces", writeResult.Namespaces,
			"ca_certs_count", writeResult.CACertsCount,
		)
	default:
		log.FromContext(ctx).Info("Containerd integrity config is in sync")
	}

	return reconcile.Result{}, nil
}

func makeDesiredConfig(policies []deckhousev1alpha1.ContainerdIntegrityPolicy) *desiredConfig {
	if len(policies) == 0 {
		return &desiredConfig{
			Namespaces: []string{},
			CACerts:    []string{},
		}
	}

	namespacesSet := make(map[string]struct{})
	caCertsSet := make(map[string]struct{})

	for i := range policies {
		policy := &policies[i]

		for _, ns := range policy.Status.ProtectedNamespaces {
			namespacesSet[ns] = struct{}{}
		}

		caCertsSet[base64.StdEncoding.EncodeToString([]byte(policy.Spec.CA))] = struct{}{}
	}

	namespaces := make([]string, 0, len(namespacesSet))
	for ns := range namespacesSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	caCerts := make([]string, 0, len(caCertsSet))
	for ca := range caCertsSet {
		caCerts = append(caCerts, ca)
	}
	sort.Strings(caCerts)

	return &desiredConfig{
		Namespaces: namespaces,
		CACerts:    caCerts,
	}
}

func renderIntegrityToml(cfg *desiredConfig) ([]byte, error) {
	return toml.Marshal(integrityTOML{Namespaces: cfg.Namespaces, CACerts: cfg.CACerts})
}

func isIntegrityConfigFileInSync(existing, desired []byte) bool {
	return bytes.Equal(existing, desired)
}

func (r *reconciler) writeIntegrityConfig(config *desiredConfig) (*writeResult, error) {
	if config == nil || len(config.Namespaces) == 0 {
		return r.removeIntegrityConfig()
	}

	if err := os.MkdirAll(integrityConfigDir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir %q: %w", integrityConfigDir, err)
	}

	integrityToml, err := renderIntegrityToml(config)
	if err != nil {
		return nil, fmt.Errorf("render integrity.toml: %w", err)
	}

	integrityTomlPath := filepath.Join(integrityConfigDir, integrityConfigFile)
	if existing, err := os.ReadFile(integrityTomlPath); err == nil {
		if isIntegrityConfigFileInSync(existing, integrityToml) {
			return &writeResult{}, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read integrity.toml: %w", err)
	}

	if err := pkiutil.WriteFileAtomically(integrityTomlPath, integrityToml, 0o644); err != nil {
		return nil, fmt.Errorf("write integrity.toml: %w", err)
	}

	return &writeResult{
		Updated:      true,
		Namespaces:   config.Namespaces,
		CACertsCount: len(config.CACerts),
	}, nil
}

func (r *reconciler) removeIntegrityConfig() (*writeResult, error) {
	path := filepath.Join(integrityConfigDir, integrityConfigFile)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return &writeResult{}, nil
		}
		return nil, fmt.Errorf("remove %q: %w", path, err)
	}

	return &writeResult{Removed: true}, nil
}

// Kubernetes I/O helpers.

func (r *reconciler) getContainerdIntegrityPolicies(
	ctx context.Context,
) ([]deckhousev1alpha1.ContainerdIntegrityPolicy, error) {
	policyList := &deckhousev1alpha1.ContainerdIntegrityPolicyList{}
	if err := r.client.List(ctx, policyList); err != nil {
		return nil, err
	}

	return policyList.Items, nil
}
