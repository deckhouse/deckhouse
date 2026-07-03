/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package configwriter

//nolint: gci
//nolint: goimports
import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"

	"github.com/BurntSushi/toml"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

const (
	IntegrityNSConfigDir = "/etc/containerd/integrity"
	IntegrityConfigFile  = "integrity.toml"
)

// DesiredConfig is the aggregated containerd integrity configuration.
type DesiredConfig struct {
	Namespaces []string
	CACerts    []string
}

// ApplyResult describes the outcome of Writer.Apply.
type ApplyResult struct {
	Updated      bool
	Removed      bool
	Namespaces   []string
	CACertsCount int
}

// Writer writes containerd integrity configuration to the local filesystem.
type Writer struct {
	ConfigDir string
}

// NewWriter creates a Writer with the given config directory.
func NewWriter(configDir string) *Writer {
	if configDir == "" {
		configDir = IntegrityNSConfigDir
	}
	return &Writer{ConfigDir: configDir}
}

// AggregatePolicies builds desired configuration from all policies.
func AggregatePolicies(policies []deckhousev1alpha1.ContainerdIntegrityPolicy) *DesiredConfig {
	if len(policies) == 0 {
		return &DesiredConfig{
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

	return &DesiredConfig{
		Namespaces: namespaces,
		CACerts:    caCerts,
	}
}

type integrityTOML struct {
	Namespaces []string `toml:"namespaces"`
	CACerts    []string `toml:"ca_certs"`
}

// RenderIntegrityToml renders the integrity.toml content for containerd.
func RenderIntegrityToml(cfg *DesiredConfig) ([]byte, error) {
	return toml.Marshal(integrityTOML{Namespaces: cfg.Namespaces, CACerts: cfg.CACerts})
}

// Apply writes or removes configuration files on disk.
func (w *Writer) Apply(config *DesiredConfig) (*ApplyResult, error) {
	if config == nil || len(config.Namespaces) == 0 {
		return w.removeConfig()
	}

	if err := os.MkdirAll(w.ConfigDir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir %q: %w", w.ConfigDir, err)
	}

	integrityToml, err := RenderIntegrityToml(config)
	if err != nil {
		return nil, fmt.Errorf("render integrity.toml: %w", err)
	}
	integrityTomlPath := filepath.Join(w.ConfigDir, IntegrityConfigFile)
	if existing, err := os.ReadFile(integrityTomlPath); err == nil {
		if bytes.Equal(existing, integrityToml) {
			return &ApplyResult{}, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read integrity.toml: %w", err)
	}
	if err := pkiutil.WriteFileAtomically(integrityTomlPath, integrityToml, 0o644); err != nil {
		return nil, fmt.Errorf("write integrity.toml: %w", err)
	}

	return &ApplyResult{
		Updated:      true,
		Namespaces:   config.Namespaces,
		CACertsCount: len(config.CACerts),
	}, nil
}

func (w *Writer) removeConfig() (*ApplyResult, error) {
	path := filepath.Join(w.ConfigDir, IntegrityConfigFile)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return &ApplyResult{}, nil
		}
		return nil, fmt.Errorf("remove %q: %w", path, err)
	}

	return &ApplyResult{Removed: true}, nil
}
