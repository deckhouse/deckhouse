/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package configwriter

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	//nolint: gci
	"github.com/go-logr/logr"
	//nolint: gci
	//nolint: goimports
	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

const (
	DefaultConfigDir = "/etc/containerd/integrity"
	NsTomlFileName   = "ns.toml"
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
		configDir = DefaultConfigDir
	}
	return &Writer{ConfigDir: configDir}
}

// AggregatePolicies builds desired configuration from all policies.
func AggregatePolicies(logger logr.Logger, policies []deckhousev1alpha1.ContainerdIntegrityPolicy) *DesiredConfig {
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

type nsTOML struct {
	Namespaces []string `toml:"namespaces"`
	CACert     []string `toml:"ca_cert"`
}

// RenderNsToml renders the ns.toml content for containerd.
func RenderNsToml(cfg *DesiredConfig) ([]byte, error) {
	return toml.Marshal(nsTOML{Namespaces: cfg.Namespaces, CACert: cfg.CACerts})
}

// Apply writes or removes configuration files on disk.
func (w *Writer) Apply(config *DesiredConfig) error {
	if config == nil || (len(config.Namespaces) == 0 && len(config.CACerts) == 0) {
		return w.removeConfig()
	}

	if err := os.MkdirAll(w.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config dir %q: %w", w.ConfigDir, err)
	}

	nsToml, err := RenderNsToml(config)
	if err != nil {
		return fmt.Errorf("render ns.toml: %w", err)
	}
	nsTomlPath := filepath.Join(w.ConfigDir, NsTomlFileName)
	if existing, err := os.ReadFile(nsTomlPath); err == nil {
		if bytes.Equal(existing, nsToml) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read ns.toml: %w", err)
	}
	if err := pkiutil.WriteFileAtomically(nsTomlPath, nsToml, 0o644); err != nil {
		return fmt.Errorf("write ns.toml: %w", err)
	}

	return nil
}

func (w *Writer) removeConfig() error {
	path := filepath.Join(w.ConfigDir, NsTomlFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %q: %w", path, err)
	}
	return nil
}
