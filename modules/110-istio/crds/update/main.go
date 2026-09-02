// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command update splits upstream Istio CRD bundles into the installed
// compatibility bundle and validates the result.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	defaultOutputDir = "modules/110-istio/crds/vendor"
	maxDownloadSize  = 16 << 20
	istioVersion     = "1.29.6"
	sailVersion      = "1.25.2"

	// This compatibility CRD was retained by Deckhouse when upstream Istio
	// removed the in-cluster operator chart. Keep it embedded so regeneration
	// does not depend on a historical Deckhouse checkout.
	legacyIstioOperatorCRD = `# SYNC WITH manifests/charts/istio-operator/templates
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: istiooperators.install.istio.io
  labels:
    release: istio
spec:
  conversion:
    strategy: None
  group: install.istio.io
  names:
    kind: IstioOperator
    listKind: IstioOperatorList
    plural: istiooperators
    singular: istiooperator
    shortNames:
    - iop
    - io
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: Istio control plane revision
      jsonPath: .spec.revision
      name: Revision
      type: string
    - description: IOP current state
      jsonPath: .status.status
      name: Status
      type: string
    - description: 'CreationTimestamp is a timestamp representing the server time
        when this object was created. It is not guaranteed to be set in happens-before
        order across separate operations. Clients may not set this value. It is represented
        in RFC3339 form and is in UTC. Populated by the system. Read-only. Null for
        lists. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata'
      jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    subresources:
      status: {}
    name: v1alpha1
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
    served: true
    storage: true
`
)

type remoteSource struct {
	name   string
	url    string
	sha256 string
}

var remoteSources = []remoteSource{
	{
		name:   "Istio configuration CRDs",
		url:    "https://raw.githubusercontent.com/istio/istio/" + istioVersion + "/manifests/charts/base/files/crd-all.gen.yaml",
		sha256: "53fd74da78d4d3ecb2e7e369e53101ea4e81b00b40f03eb75894d2e97dd5b19a",
	},
	{
		name:   "Sail IstioCNI CRD",
		url:    sailURL("istiocnis"),
		sha256: "764e4f10218c11e9051e87a40c4fe1ecded88fe83271c8522d61dbe811b95406",
	},
	{
		name:   "Sail IstioRevision CRD",
		url:    sailURL("istiorevisions"),
		sha256: "7395579fd9d34773f9ccf2dfe7c3c41c8a7c24880f72d56d2712e2d984e6c32e",
	},
	{
		name:   "Sail IstioRevisionTag CRD",
		url:    sailURL("istiorevisiontags"),
		sha256: "261b78aea81cd79bf4f2062a2f02ed4e117ad296ef59d8c0b6a3e4159a243966",
	},
	{
		name:   "Sail Istio CRD",
		url:    sailURL("istios"),
		sha256: "1a40c6e95880c6de571d3f64154ceaaf51ff20bd1742344514c8be05070ea16f",
	},
	{
		name:   "Sail ZTunnel CRD",
		url:    sailURL("ztunnels"),
		sha256: "1e5b11e3d4722fe8e1deed97ed922693c2daf86ceaa7d58ccc5779961edbf486",
	},
}

var requiredServedVersions = map[string]map[string]struct{}{
	"authorizationpolicies.security.istio.io":  stringSet("v1", "v1beta1"),
	"destinationrules.networking.istio.io":     stringSet("v1", "v1alpha3", "v1beta1"),
	"envoyfilters.networking.istio.io":         stringSet("v1alpha3"),
	"gateways.networking.istio.io":             stringSet("v1", "v1alpha3", "v1beta1"),
	"peerauthentications.security.istio.io":    stringSet("v1", "v1beta1"),
	"proxyconfigs.networking.istio.io":         stringSet("v1beta1"),
	"requestauthentications.security.istio.io": stringSet("v1", "v1beta1"),
	"serviceentries.networking.istio.io":       stringSet("v1", "v1alpha3", "v1beta1"),
	"sidecars.networking.istio.io":             stringSet("v1", "v1alpha3", "v1beta1"),
	"telemetries.telemetry.istio.io":           stringSet("v1", "v1alpha1"),
	"virtualservices.networking.istio.io":      stringSet("v1", "v1alpha3", "v1beta1"),
	"wasmplugins.extensions.istio.io":          stringSet("v1alpha1"),
	"workloadentries.networking.istio.io":      stringSet("v1", "v1alpha3", "v1beta1"),
	"workloadgroups.networking.istio.io":       stringSet("v1", "v1alpha3", "v1beta1"),
}

var operatorCRDs = stringSet(
	"istiooperators.install.istio.io",
	"istiocnis.sailoperator.io",
	"istiorevisions.sailoperator.io",
	"istiorevisiontags.sailoperator.io",
	"istios.sailoperator.io",
	"ztunnels.sailoperator.io",
)

type options struct {
	check     bool
	outputDir string
}

type crd struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Versions []struct {
			Name    string `yaml:"name"`
			Served  bool   `yaml:"served"`
			Storage bool   `yaml:"storage"`
		} `yaml:"versions"`
	} `yaml:"spec"`
}

type object struct {
	node *yaml.Node
	crd  crd
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("istio-crd-update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	var opts options
	flags.BoolVar(&opts.check, "check", false, "validate the installed bundle")
	flags.StringVar(&opts.outputDir, "output-dir", defaultOutputDir, "installed compatibility bundle directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}

	if opts.check {
		return checkBundle(opts.outputDir)
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	objects, err := downloadSources(context.Background(), client, remoteSources)
	if err != nil {
		return err
	}
	if err := validate(objects); err != nil {
		return err
	}
	return replaceBundle(opts.outputDir, objects)
}

func sailURL(name string) string {
	return "https://raw.githubusercontent.com/istio-ecosystem/sail-operator/" + sailVersion + "/chart/crds/sailoperator.io_" + name + ".yaml"
}

func downloadSources(ctx context.Context, client *http.Client, sources []remoteSource) ([]object, error) {
	objects, err := loadReader("embedded legacy IstioOperator CRD", strings.NewReader(legacyIstioOperatorCRD))
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request for %s: %w", source.name, err)
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("download %s from %s: %w", source.name, source.url, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxDownloadSize+1))
		closeErr := response.Body.Close()
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("download %s from %s: HTTP %s", source.name, source.url, response.Status)
		}
		if readErr != nil {
			return nil, fmt.Errorf("read %s from %s: %w", source.name, source.url, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close response for %s: %w", source.name, closeErr)
		}
		if len(data) > maxDownloadSize {
			return nil, fmt.Errorf("download %s from %s exceeds %d bytes", source.name, source.url, maxDownloadSize)
		}
		actualChecksum := fmt.Sprintf("%x", sha256.Sum256(data))
		if actualChecksum != source.sha256 {
			return nil, fmt.Errorf("checksum mismatch for %s: expected %s, got %s", source.name, source.sha256, actualChecksum)
		}
		downloaded, err := loadReader(source.url, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		objects = append(objects, downloaded...)
	}
	return objects, nil
}

func load(path string) ([]object, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	return loadReader(path, file)
}

func loadReader(name string, reader io.Reader) ([]object, error) {
	decoder := yaml.NewDecoder(reader)
	var objects []object
	for document := 1; ; document++ {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode %s document %d: %w", name, document, err)
		}
		if emptyDocument(&node) {
			continue
		}

		var definition crd
		if err := node.Decode(&definition); err != nil {
			return nil, fmt.Errorf("decode CRD fields in %s document %d: %w", name, document, err)
		}
		objects = append(objects, object{node: &node, crd: definition})
	}
	return objects, nil
}

func emptyDocument(node *yaml.Node) bool {
	if len(node.Content) == 0 {
		return true
	}
	root := node.Content[0]
	return root.Tag == "!!null"
}

func validate(objects []object) error {
	expected := make(map[string]struct{}, len(requiredServedVersions)+len(operatorCRDs))
	for name := range requiredServedVersions {
		expected[name] = struct{}{}
	}
	for name := range operatorCRDs {
		expected[name] = struct{}{}
	}

	names := make([]string, 0, len(objects))
	seen := make(map[string]struct{}, len(objects))
	for _, object := range objects {
		name := object.crd.Metadata.Name
		names = append(names, name)
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate CRD %q", name)
		}
		seen[name] = struct{}{}
	}
	if len(seen) != len(expected) || !sameSet(seen, expected) {
		sort.Strings(names)
		return fmt.Errorf("expected exactly %d CRDs, got %d: %v", len(expected), len(seen), names)
	}

	for _, object := range objects {
		definition := object.crd
		name := definition.Metadata.Name
		if definition.APIVersion != "apiextensions.k8s.io/v1" || definition.Kind != "CustomResourceDefinition" {
			return fmt.Errorf("%s: not a v1 CRD", name)
		}

		storageVersions := 0
		served := make(map[string]struct{})
		for _, version := range definition.Spec.Versions {
			if version.Storage {
				storageVersions++
			}
			if version.Served {
				served[version.Name] = struct{}{}
			}
		}
		if storageVersions != 1 {
			return fmt.Errorf("%s: expected one storage version", name)
		}

		required, isConfigCRD := requiredServedVersions[name]
		if !isConfigCRD {
			continue
		}
		missing := difference(required, served)
		if len(missing) != 0 {
			return fmt.Errorf("%s: required served versions were removed: %v; served versions: %v", name, missing, sortedSet(served))
		}
	}
	return nil
}

func checkBundle(outputDir string) error {
	paths, err := filepath.Glob(filepath.Join(outputDir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("find installed CRDs: %w", err)
	}
	sort.Strings(paths)

	var objects []object
	for _, path := range paths {
		fileObjects, err := load(path)
		if err != nil {
			return err
		}
		if len(fileObjects) != 1 {
			return fmt.Errorf("%s: expected exactly one YAML object", path)
		}
		objects = append(objects, fileObjects...)
	}
	return validate(objects)
}

func replaceBundle(outputDir string, objects []object) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(outputDir, ".update-*")
	if err != nil {
		return fmt.Errorf("create temporary output directory: %w", err)
	}
	removeTempDir := true
	defer func() {
		if removeTempDir {
			_ = os.RemoveAll(tempDir)
		}
	}()

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].crd.Metadata.Name < objects[j].crd.Metadata.Name
	})
	for _, object := range objects {
		path := filepath.Join(tempDir, object.crd.Metadata.Name+".yaml")
		if err := writeYAML(path, object.node); err != nil {
			return err
		}
	}
	if err := checkBundle(tempDir); err != nil {
		return fmt.Errorf("validate generated bundle: %w", err)
	}

	oldPaths, err := filepath.Glob(filepath.Join(outputDir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("find old CRDs: %w", err)
	}
	backupDir := filepath.Join(tempDir, "backup")
	if err := os.Mkdir(backupDir, 0o755); err != nil {
		return fmt.Errorf("create CRD backup: %w", err)
	}
	rollback := func(operationErr error, rollbackErr error) error {
		if rollbackErr == nil {
			return operationErr
		}
		removeTempDir = false
		return fmt.Errorf("%w; rollback failed: %v; backup preserved at %s", operationErr, rollbackErr, backupDir)
	}
	for _, oldPath := range oldPaths {
		if err := os.Rename(oldPath, filepath.Join(backupDir, filepath.Base(oldPath))); err != nil {
			operationErr := fmt.Errorf("back up %s: %w", oldPath, err)
			return rollback(operationErr, restoreBundle(outputDir, backupDir))
		}
	}

	generatedPaths, err := filepath.Glob(filepath.Join(tempDir, "*.yaml"))
	if err != nil {
		operationErr := fmt.Errorf("find generated CRDs: %w", err)
		return rollback(operationErr, restoreBundle(outputDir, backupDir))
	}
	for _, generatedPath := range generatedPaths {
		if err := os.Rename(generatedPath, filepath.Join(outputDir, filepath.Base(generatedPath))); err != nil {
			operationErr := fmt.Errorf("install %s: %w", generatedPath, err)
			rollbackErr := errors.Join(removeYAML(outputDir), restoreBundle(outputDir, backupDir))
			return rollback(operationErr, rollbackErr)
		}
	}
	return nil
}

func writeYAML(path string, node *yaml.Node) error {
	var value any
	if err := node.Decode(&value); err != nil {
		return fmt.Errorf("decode %s for encoding: %w", path, err)
	}
	data, err := k8syaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	// Match Istio's protoc-gen-crd output, including the annotation quoting
	// that its generator retains for Helm compatibility.
	data = bytes.ReplaceAll(data, []byte("helm.sh/resource-policy: keep"), []byte(`"helm.sh/resource-policy": keep`))
	if comment := documentHeadComment(node); comment != "" {
		data = append([]byte(comment+"\n"), data...)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write %s: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", path, closeErr)
	}
	return nil
}

func documentHeadComment(node *yaml.Node) string {
	if node.Kind == yaml.DocumentNode && len(node.Content) != 0 {
		node = node.Content[0]
	}
	if node.Kind == yaml.MappingNode && len(node.Content) != 0 {
		return node.Content[0].HeadComment
	}
	return node.HeadComment
}

func restoreBundle(outputDir, backupDir string) error {
	paths, err := filepath.Glob(filepath.Join(backupDir, "*.yaml"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.Rename(path, filepath.Join(outputDir, filepath.Base(path))); err != nil {
			return err
		}
	}
	return nil
}

func removeYAML(dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func sameSet(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if _, exists := right[value]; !exists {
			return false
		}
	}
	return true
}

func difference(left, right map[string]struct{}) []string {
	var result []string
	for value := range left {
		if _, exists := right[value]; !exists {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
