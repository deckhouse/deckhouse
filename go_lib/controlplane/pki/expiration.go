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

package pki

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

type ExpirationOption func(*expirationOptions)

type expirationOptions struct {
	certificatesDir  string
	leafCertificates []LeafCertName
	rootCertificates []RootCertName
	ignoreReadErrors bool
}

type CertificateExpiration struct {
	Name      string
	Path      string
	NotAfter  time.Time
	Authority RootCertName
	IsCA      bool
}

type certificateInventoryItem struct {
	name      string
	relPath   string
	authority RootCertName
}

// WithCertificatesDir overrides the directory used by ListCertificateExpirations.
func WithCertificatesDir(dir string) ExpirationOption {
	return func(o *expirationOptions) {
		o.certificatesDir = dir
	}
}

// WithLeafCertificates restricts ListCertificateExpirations to the provided leaf certificates.
func WithLeafCertificates(names ...LeafCertName) ExpirationOption {
	return func(o *expirationOptions) {
		o.leafCertificates = append(o.leafCertificates, names...)
	}
}

// WithRootCertificates restricts ListCertificateExpirations to the provided root certificates.
func WithRootCertificates(names ...RootCertName) ExpirationOption {
	return func(o *expirationOptions) {
		o.rootCertificates = append(o.rootCertificates, names...)
	}
}

// WithIgnoreReadErrors enables partial success and returns read failures as errors.Join(...).
func WithIgnoreReadErrors() ExpirationOption {
	return func(o *expirationOptions) {
		o.ignoreReadErrors = true
	}
}

func ListCertificateExpirations(opts ...ExpirationOption) ([]CertificateExpiration, error) {
	options := newExpirationOptions(opts...)

	inventory, err := buildCertificateInventory(options)
	if err != nil {
		return nil, err
	}

	result := make([]CertificateExpiration, 0, len(inventory))
	var errs []error

	for _, item := range inventory {
		expiration, err := loadCertificateExpiration(filepath.Join(options.certificatesDir, item.relPath), item)
		if err != nil {
			if !options.ignoreReadErrors {
				return nil, err
			}

			errs = append(errs, err)

			continue
		}

		result = append(result, expiration)
	}

	return result, errors.Join(errs...)
}

func GetCertificateExpiration(path string) (CertificateExpiration, error) {
	item, ok := lookupKnownCertificate(path)
	if !ok {
		item = inventoryItemFromPath(path)
	}

	return loadCertificateExpiration(path, item)
}

func newExpirationOptions(opts ...ExpirationOption) *expirationOptions {
	options := &expirationOptions{
		certificatesDir: constants.DefaultCertificatesDir,
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}

func buildCertificateInventory(options *expirationOptions) ([]certificateInventoryItem, error) {
	rootItems, leafItems := defaultCertificateInventory()

	selected := make(map[string]certificateInventoryItem)

	if len(options.rootCertificates) == 0 && len(options.leafCertificates) == 0 {
		for _, item := range rootItems {
			selected[item.relPath] = item
		}
		for _, item := range leafItems {
			selected[item.relPath] = item
		}

		return sortedInventory(selected), nil
	}

	for _, name := range options.rootCertificates {
		item, ok := rootItems[name]
		if !ok {
			return nil, fmt.Errorf("unknown root certificate %q", name)
		}
		selected[item.relPath] = item
	}

	for _, name := range options.leafCertificates {
		item, ok := leafItems[name]
		if !ok {
			return nil, fmt.Errorf("unknown leaf certificate %q", name)
		}
		selected[item.relPath] = item
	}

	return sortedInventory(selected), nil
}

func loadCertificateExpiration(path string, item certificateInventoryItem) (CertificateExpiration, error) {
	cert, err := pkiutil.LoadCert(path)
	if err != nil {
		return CertificateExpiration{}, fmt.Errorf("failed to load certificate %q: %w", path, err)
	}

	return CertificateExpiration{
		Name:      item.name,
		Path:      filepath.Clean(path),
		NotAfter:  cert.NotAfter,
		Authority: item.authority,
		IsCA:      cert.IsCA,
	}, nil
}

func defaultCertificateInventory() (map[RootCertName]certificateInventoryItem, map[LeafCertName]certificateInventoryItem) {
	rootItems := make(map[RootCertName]certificateInventoryItem, len(defaultCertTreeScheme))
	leafItems := make(map[LeafCertName]certificateInventoryItem)

	for rootName, leafNames := range defaultCertTreeScheme {
		rootItems[rootName] = certificateInventoryItem{
			name:    string(rootName),
			relPath: certificateRelPath(string(rootName)),
		}

		for _, leafName := range leafNames {
			leafItems[leafName] = certificateInventoryItem{
				name:      string(leafName),
				relPath:   certificateRelPath(string(leafName)),
				authority: rootName,
			}
		}
	}

	return rootItems, leafItems
}

func sortedInventory(items map[string]certificateInventoryItem) []certificateInventoryItem {
	paths := make([]string, 0, len(items))
	for path := range items {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	result := make([]certificateInventoryItem, 0, len(paths))
	for _, path := range paths {
		result = append(result, items[path])
	}

	return result
}

// knownCertsByRelPath is a precomputed lookup map built once from the static
// defaultCertTreeScheme. It maps each certificate's relative path to its
// inventory item so that lookupKnownCertificate avoids rebuilding maps on
// every call.
var knownCertsByRelPath = func() map[string]certificateInventoryItem {
	rootItems, leafItems := defaultCertificateInventory()
	m := make(map[string]certificateInventoryItem, len(rootItems)+len(leafItems))
	for _, item := range rootItems {
		m[item.relPath] = item
	}
	for _, item := range leafItems {
		m[item.relPath] = item
	}
	return m
}()

func lookupKnownCertificate(path string) (certificateInventoryItem, bool) {
	for _, suffix := range pathSuffixes(path) {
		if item, ok := knownCertsByRelPath[suffix]; ok {
			return item, true
		}
	}

	return certificateInventoryItem{}, false
}

func inventoryItemFromPath(path string) certificateInventoryItem {
	cleanPath := filepath.Clean(path)
	base := filepath.Base(cleanPath)

	return certificateInventoryItem{
		name:    strings.TrimSuffix(base, filepath.Ext(base)),
		relPath: cleanPath,
	}
}

func certificateRelPath(name string) string {
	return filepath.Join(strings.Split(name, "/")...) + ".crt"
}

func pathSuffixes(path string) []string {
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(cleanPath, "/")
	suffixes := make([]string, 0, len(parts))

	for i := range parts {
		suffix := strings.Join(parts[i:], "/")
		if suffix == "" {
			continue
		}

		suffixes = append(suffixes, suffix)
	}

	return suffixes
}
