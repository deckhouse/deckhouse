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

// Package testenv starts a real API server for the envtest tests. The tests
// skip when the envtest binaries are not available. "make envtest" installs
// them.
package testenv

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// Env is a running API server.
type Env struct {
	*envtest.Environment
	Config *rest.Config
	// Client is a client with the rights of system:masters.
	Client client.WithWatch
}

// Start starts the API server with the CRDs of the fencing API and the given
// CRD files. The paths are relative to the directory of the node-manager
// module. The API server stops at the end of the test.
func Start(t *testing.T, crds ...string) *Env {
	t.Helper()

	assets := assetsDir()
	if assets == "" {
		t.Skip("envtest binaries are not found: run make envtest or set KUBEBUILDER_ASSETS")
	}

	logf.SetLogger(logr.Discard())

	paths := make([]string, 0, len(crds)+2)
	for _, crd := range append([]string{"crds/fencingfailednodestates.yaml", "crds/fencingslaprofiles.yaml"}, crds...) {
		paths = append(paths, ModuleFile(crd))
	}

	env := &Env{Environment: &envtest.Environment{
		CRDDirectoryPaths:     paths,
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: assets,
	}}

	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}

	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})

	env.Config = cfg
	env.Client = NewClient(t, cfg)

	return env
}

// NewClient returns a client that knows the core and the fencing API.
func NewClient(t *testing.T, cfg *rest.Config) client.WithWatch {
	t.Helper()

	scheme := k8sruntime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("register client-go scheme: %v", err)
	}

	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("register fencing scheme: %v", err)
	}

	c, err := client.NewWithWatch(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	return c
}

// ModuleFile returns the path of a file of the node-manager module. The path
// is relative to the directory of the module.
func ModuleFile(path string) string {
	return filepath.Join(sourceDir(), "..", "..", "..", "..", "..", path)
}

// Manifests reads the objects of a Helm template of the node-manager module.
// The function puts the values of the variables of the template into the text
// and drops the other lines with Helm actions. Thus the template must give
// plain YAML without them.
func Manifests(t *testing.T, path string) []*unstructured.Unstructured {
	t.Helper()

	raw, err := os.ReadFile(ModuleFile(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	variables := map[string]string{}
	for _, match := range variable.FindAllStringSubmatch(string(raw), -1) {
		variables[match[1]] = match[2]
	}

	var plain []string

	for _, line := range strings.Split(string(raw), "\n") {
		for name, value := range variables {
			line = strings.ReplaceAll(line, "{{ $"+name+" }}", value)
		}

		if !strings.Contains(line, "{{") {
			plain = append(plain, line)
		}
	}

	var objects []*unstructured.Unstructured

	for _, document := range strings.Split(strings.Join(plain, "\n"), "\n---") {
		object := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(document), &object.Object); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		if len(object.Object) > 0 {
			objects = append(objects, object)
		}
	}

	return objects
}

// variable finds the assignments {{- $name := "value" }} of a template.
var variable = regexp.MustCompile(`\{\{-? \$(\w+) := "([^"]*)" -?\}\}`)

// assetsDir returns the directory of the envtest binaries: KUBEBUILDER_ASSETS,
// or the directory that make envtest fills for ENVTEST_K8S_VERSION of the
// Makefile. It returns an empty string when there are no binaries.
func assetsDir() string {
	if dir := os.Getenv("KUBEBUILDER_ASSETS"); dir != "" {
		return dir
	}

	src := filepath.Join(sourceDir(), "..", "..")

	makefile, err := os.ReadFile(filepath.Join(src, "Makefile"))
	if err != nil {
		return ""
	}

	version := envtestVersion.FindSubmatch(makefile)
	if version == nil {
		return ""
	}

	binaries, _ := filepath.Glob(filepath.Join(src, "bin", "k8s", string(version[1])+"-*", "kube-apiserver"))
	if len(binaries) == 0 {
		return ""
	}

	return filepath.Dir(binaries[0])
}

// envtestVersion finds the version of the envtest binaries in the Makefile.
var envtestVersion = regexp.MustCompile(`(?m)^ENVTEST_K8S_VERSION \?= (\S+)$`)

// sourceDir returns the directory of this file. The paths of the module start
// there, so they do not depend on the directory of the test.
func sourceDir() string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	return filepath.Dir(self)
}
