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

package nelm

import (
	"cmp"
	"io"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const ingressKind = "Ingress"

// endpointIngress is a minimal projection of an Ingress manifest, just enough
// to detect the endpoint annotation and build URLs from rules and TLS hosts.
type endpointIngress struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		TLS []struct {
			Hosts []string `json:"hosts"`
		} `json:"tls"`
		Rules []struct {
			Host string `json:"host"`
			HTTP struct {
				Paths []struct {
					Path string `json:"path"`
				} `json:"paths"`
			} `json:"http"`
		} `json:"rules"`
	} `json:"spec"`
}

// extractEndpointURLs collects application endpoints from a rendered
// multi-document YAML manifest. An endpoint is built for every host/path pair
// of networking.k8s.io Ingress resources annotated with
// packages.deckhouse.io/is-application-endpoint (any value except "false").
// The annotation value becomes the endpoint description ("true" means no
// description). The scheme is https when the host is covered by spec.tls,
// http otherwise. The result is deduplicated and sorted; undecodable
// documents are skipped.
func extractEndpointURLs(renderedManifests string) []status.URL {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedManifests), 4096)

	var urls []status.URL
	for {
		ing := new(endpointIngress)
		if err := dec.Decode(ing); err != nil {
			if err == io.EOF {
				break
			}
			// Skip empty or undecodable documents (e.g. standalone '---',
			// documents whose shape does not match the projection).
			continue
		}

		if ing.Kind != ingressKind || !strings.HasPrefix(ing.APIVersion, "networking.k8s.io/") {
			continue
		}

		value, set := ing.Metadata.Annotations[v1alpha1.ApplicationAnnotationEndpoint]
		if !set || value == "false" {
			continue
		}

		description := value
		if description == "true" {
			description = ""
		}

		tlsHosts := make(map[string]struct{})
		for _, tls := range ing.Spec.TLS {
			for _, host := range tls.Hosts {
				tlsHosts[host] = struct{}{}
			}
		}

		for _, rule := range ing.Spec.Rules {
			if rule.Host == "" {
				continue
			}

			scheme := "http"
			if _, ok := tlsHosts[rule.Host]; ok {
				scheme = "https"
			}

			paths := rule.HTTP.Paths
			if len(paths) == 0 {
				urls = append(urls, status.URL{URL: scheme + "://" + rule.Host + "/", Description: description})
				continue
			}

			for _, p := range paths {
				path := p.Path
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
				urls = append(urls, status.URL{URL: scheme + "://" + rule.Host + path, Description: description})
			}
		}
	}

	slices.SortFunc(urls, func(a, b status.URL) int {
		return cmp.Or(cmp.Compare(a.URL, b.URL), cmp.Compare(a.Description, b.Description))
	})

	return slices.Compact(urls)
}
