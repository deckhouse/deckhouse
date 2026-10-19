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

// Package nodeletseed builds the NodeConfig the node agent starts from on the first
// master, where no API exists yet to hand it one.
package nodeletseed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"sigs.k8s.io/yaml"
)

const (
	// templatePath is the platform's document of a bashible node, under the candi
	// directory. node-controller renders the same file for every node after the first.
	templatePath = "nodelet/nodeconfig-mutable.yaml.tpl"

	// installerProxyToken mirrors the default in
	// candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl.
	installerProxyToken = "passthrough"
)

// Document is the seed NodeConfig. It carries no name: the node puts its own in.
type Document struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       Spec   `json:"spec"`
}

type Spec struct {
	SystemType            string                `json:"systemType"`
	RegistryPackagesProxy RegistryPackagesProxy `json:"registryPackagesProxy"`
	FeatureGates          map[string]bool       `json:"featureGates"`
	Extensions            []Extension           `json:"extensions"`
	StaticPods            []StaticPod           `json:"staticPods,omitempty"`
}

// RegistryPackagesProxy is how the agent reaches the installer's packages proxy.
type RegistryPackagesProxy struct {
	Endpoints []string `json:"endpoints,omitempty"`
	// AccessToken holds the raw token; JSON carries it base64-encoded.
	AccessToken []byte `json:"accessToken,omitempty"`
}

// Extension is a platform package: no repository, and no requestedBy.
type Extension struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type StaticPod struct {
	Name     string `json:"name"`
	Manifest string `json:"manifest"`
}

// FirstMaster fills the release's digests into the platform's document for the installer's
// proxy; a digest missing, the agent's included, fails it. Mirrors renderPlatformDocument in
// modules/040-node-manager/images/node-controller/src/internal/controller/nodeconfig/platform.go.
func FirstMaster(_ context.Context, candiDir string, images map[string]any, proxyEndpoints []string) (*Document, error) {
	if len(proxyEndpoints) == 0 {
		return nil, errors.New("no packages proxy endpoints to point the agent at")
	}
	path := filepath.Join(candiDir, templatePath)
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the platform node document: %w", err)
	}
	tpl, err := template.New(templatePath).Option("missingkey=error").Parse(string(text))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, map[string]any{"images": images}); err != nil {
		return nil, fmt.Errorf("render %s: %w", path, err)
	}
	doc := &Document{}
	if err := yaml.UnmarshalStrict(rendered.Bytes(), doc); err != nil {
		return nil, fmt.Errorf("parse rendered %s: %w", path, err)
	}

	for i, ext := range doc.Spec.Extensions {
		if ext.Name == "" || ext.Digest == "" {
			return nil, fmt.Errorf("%s: extension %d has no name or no digest", path, i)
		}
	}
	doc.Spec.RegistryPackagesProxy = RegistryPackagesProxy{
		Endpoints:   proxyEndpoints,
		AccessToken: []byte(installerProxyToken),
	}
	return doc, nil
}
