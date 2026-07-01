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
	"strings"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"

	"github.com/BurntSushi/toml"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
	"github.com/deckhouse/deckhouse/pkg/log"
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
func AggregatePolicies(logger *log.Logger, policies []deckhousev1alpha1.ContainerdIntegrityPolicy) *DesiredConfig {
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

		policyCA := strings.TrimSpace(policy.Spec.CA)
		if policyCA == "" {
			logger.Info("Skipping policy with empty spec.ca", "policy", policy.Name)
			continue
		}

		for _, ns := range policy.Status.ProtectedNamespaces {
			namespacesSet[ns] = struct{}{}
		}

		caCertsSet[base64.StdEncoding.EncodeToString([]byte(policyCA))] = struct{}{}
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
func (w *Writer) Apply(logger *log.Logger, config *DesiredConfig) error {
	if config == nil || len(config.Namespaces) == 0 {
		return w.removeConfig(logger)
	}

	if err := os.MkdirAll(w.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config dir %q: %w", w.ConfigDir, err)
	}

	integrityToml, err := RenderIntegrityToml(config)
	if err != nil {
		return fmt.Errorf("render integrity.toml: %w", err)
	}
	integrityTomlPath := filepath.Join(w.ConfigDir, IntegrityConfigFile)
	if existing, err := os.ReadFile(integrityTomlPath); err == nil {
		if bytes.Equal(existing, integrityToml) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read integrity.toml: %w", err)
	}
	if err := pkiutil.WriteFileAtomically(integrityTomlPath, integrityToml, 0o644); err != nil {
		return fmt.Errorf("write integrity.toml: %w", err)
	}

	logger.Info("Updated containerd integrity config", "namespaces", config.Namespaces, "ca_certs_count", len(config.CACerts))

	return nil
}

func (w *Writer) removeConfig(logger *log.Logger) error {
	path := filepath.Join(w.ConfigDir, IntegrityConfigFile)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove %q: %w", path, err)
	}

	logger.Info("Found no namespaces matching the policies' selectors, removing containerd integrity config")

	return nil
}
