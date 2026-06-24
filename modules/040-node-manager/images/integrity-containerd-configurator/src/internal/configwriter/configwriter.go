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

package configwriter

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
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
func AggregatePolicies(policies []deckhousev1alpha1.ContainerdIntegrityPolicy) (*DesiredConfig, error) {
	if len(policies) == 0 {
		return nil, nil
	}

	namespacesSet := make(map[string]struct{})
	caCertsSet := make(map[string]struct{})

	for i := range policies {
		policy := &policies[i]
		for _, ns := range policy.Status.ProtectedNamespaces {
			namespacesSet[ns] = struct{}{}
		}

		policyCA := strings.TrimSpace(policy.Spec.CA)
		if policyCA == "" {
			return nil, fmt.Errorf("policy %q has empty spec.ca", policy.Name)
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
	}, nil
}

// RenderNsToml renders the ns.toml content for containerd.
func RenderNsToml(namespaces, caCerts []string) string {
	var b strings.Builder
	b.WriteString("namespaces = ")
	b.WriteString(renderStringArray(namespaces, true))
	b.WriteString("\nca_cert = ")
	b.WriteString(renderStringArray(caCerts, false))
	b.WriteByte('\n')
	return b.String()
}

func renderStringArray(values []string, spaced bool) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			if spaced {
				b.WriteString(", ")
			} else {
				b.WriteByte(',')
			}
		}
		b.WriteByte('"')
		b.WriteString(value)
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}

// Apply writes or removes configuration files on disk.
func (w *Writer) Apply(config *DesiredConfig) error {
	if config == nil {
		return w.removeConfig()
	}

	if err := os.MkdirAll(w.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config dir %q: %w", w.ConfigDir, err)
	}

	nsToml := RenderNsToml(config.Namespaces, config.CACerts)
	nsTomlPath := filepath.Join(w.ConfigDir, NsTomlFileName)
	if err := writeFileAtomic(nsTomlPath, []byte(nsToml), 0o644); err != nil {
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

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}
	defer cleanup()

	if err := tmpFile.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmpFile.Write(data); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}
