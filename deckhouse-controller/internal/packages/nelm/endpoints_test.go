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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
)

func TestExtractEndpointURLs(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
		expected []observed.URL
	}{
		{
			name:     "empty manifest",
			rendered: "",
			expected: nil,
		},
		{
			name: "annotated ingress with tls and multiple paths",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  tls:
  - hosts:
    - app.example.com
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /ui
      - path: /api
`,
			expected: []observed.URL{
				{URL: "https://app.example.com/api"},
				{URL: "https://app.example.com/ui"},
			},
		},
		{
			name: "annotation value becomes description",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Web UI"
spec:
  tls:
  - hosts:
    - app.example.com
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /ui
`,
			expected: []observed.URL{
				{URL: "https://app.example.com/ui", Description: "Web UI"},
			},
		},
		{
			name: "no tls yields http scheme",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
`,
			expected: []observed.URL{{URL: "http://app.example.com/"}},
		},
		{
			name: "rule without paths defaults to root",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  tls:
  - hosts:
    - app.example.com
  rules:
  - host: app.example.com
`,
			expected: []observed.URL{{URL: "https://app.example.com/"}},
		},
		{
			name: "ingress without annotation is skipped",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app
spec:
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
`,
			expected: nil,
		},
		{
			name: "annotation set to false is skipped",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "false"
spec:
  rules:
  - host: app.example.com
`,
			expected: nil,
		},
		{
			name: "rule without host is skipped",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  rules:
  - http:
      paths:
      - path: /
`,
			expected: nil,
		},
		{
			name: "non-ingress documents are ignored",
			rendered: `
apiVersion: v1
kind: Service
metadata:
  name: app
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
`,
			expected: nil,
		},
		{
			name: "multiple documents with dedup",
			rendered: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Dashboard"
spec:
  tls:
  - hosts:
    - a.example.com
  rules:
  - host: a.example.com
    http:
      paths:
      - path: /
      - path: /
  - host: b.example.com
    http:
      paths:
      - path: /metrics
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: not-endpoint
spec:
  rules:
  - host: c.example.com
`,
			expected: []observed.URL{
				{URL: "http://b.example.com/metrics", Description: "Dashboard"},
				{URL: "https://a.example.com/", Description: "Dashboard"},
			},
		},
		{
			name: "path without leading slash is normalized",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  rules:
  - host: app.example.com
    http:
      paths:
      - path: ui
`,
			expected: []observed.URL{{URL: "http://app.example.com/ui"}},
		},
		{
			name: "annotated httproute on an https listener of a rendered listenerset",
			rendered: `
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app
spec:
  parentRef:
    name: public-gw
    namespace: d8-alb
  listeners:
  - name: web
    port: 443
    protocol: HTTPS
    hostname: app.example.com
  - name: web-redirect
    port: 80
    protocol: HTTP
    hostname: app.example.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Web UI"
spec:
  parentRefs:
  - group: gateway.networking.k8s.io
    kind: ListenerSet
    name: app
    sectionName: web
  hostnames:
  - app.example.com
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /ui
    - path:
        type: Exact
        value: /api
`,
			expected: []observed.URL{
				{URL: "https://app.example.com/api", Description: "Web UI"},
				{URL: "https://app.example.com/ui", Description: "Web UI"},
			},
		},
		{
			name: "httproute before its listenerset takes the listener hostnames",
			rendered: `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  parentRefs:
  - kind: ListenerSet
    name: app
  rules:
  - backendRefs:
    - name: app
      port: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app
spec:
  listeners:
  - name: web
    port: 443
    protocol: HTTPS
    hostname: app.example.com
  - name: web-redirect
    port: 80
    protocol: HTTP
    hostname: app.example.com
`,
			expected: []observed.URL{{URL: "https://app.example.com/"}},
		},
		{
			name: "sectionName and port pick the http listener",
			rendered: `
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app
spec:
  listeners:
  - name: web
    port: 443
    protocol: HTTPS
    hostname: app.example.com
  - name: web-redirect
    port: 80
    protocol: HTTP
    hostname: app.example.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  parentRefs:
  - kind: ListenerSet
    name: app
    sectionName: web-redirect
  hostnames:
  - a.example.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  parentRefs:
  - kind: ListenerSet
    name: app
    port: 80
  hostnames:
  - b.example.com
`,
			expected: []observed.URL{
				{URL: "http://a.example.com/"},
				{URL: "http://b.example.com/"},
			},
		},
		{
			name: "httproute without a rendered listenerset yields http",
			rendered: `
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app
  namespace: app-ns
spec:
  listeners:
  - name: web
    port: 443
    protocol: HTTPS
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Gateway"
spec:
  parentRefs:
  - name: public-gw
    namespace: d8-alb
    sectionName: d8-https
  hostnames:
  - a.example.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Other namespace"
spec:
  parentRefs:
  - kind: ListenerSet
    name: app
    namespace: other-ns
  hostnames:
  - b.example.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "No hostnames"
spec:
  parentRefs:
  - name: public-gw
    namespace: d8-alb
`,
			expected: []observed.URL{
				{URL: "http://a.example.com/", Description: "Gateway"},
				{URL: "http://b.example.com/", Description: "Other namespace"},
			},
		},
		{
			name: "wildcard listener hostname covers the route host",
			rendered: `
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: app
spec:
  listeners:
  - name: web
    port: 443
    protocol: HTTPS
    hostname: "*.example.com"
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  parentRefs:
  - kind: ListenerSet
    name: app
  hostnames:
  - app.example.com
  - example.com
`,
			expected: []observed.URL{
				{URL: "http://example.com/"},
				{URL: "https://app.example.com/"},
			},
		},
		{
			name: "regular expression match is skipped, a match without a path is the root",
			rendered: `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  hostnames:
  - app.example.com
  rules:
  - matches:
    - path:
        type: RegularExpression
        value: /v[0-9]+
    - headers:
      - name: x-version
        value: "2"
  - matches:
    - path:
        type: PathPrefix
        value: api
`,
			expected: []observed.URL{
				{URL: "http://app.example.com/"},
				{URL: "http://app.example.com/api"},
			},
		},
		{
			name: "httproute without annotation or of another group is skipped",
			rendered: `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app
spec:
  hostnames:
  - a.example.com
---
apiVersion: example.io/v1
kind: HTTPRoute
metadata:
  annotations:
    packages.deckhouse.io/application-endpoint-description: "true"
spec:
  hostnames:
  - b.example.com
`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractEndpointURLs(tt.rendered))
		})
	}
}

func TestHasWorkloads(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
		expected bool
	}{
		{name: "empty", rendered: "", expected: false},
		{
			name:     "config only",
			rendered: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n---\napiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\n",
			expected: false,
		},
		{name: "deployment", rendered: "apiVersion: v1\nkind: ConfigMap\n---\napiVersion: apps/v1\nkind: Deployment\n", expected: true},
		{name: "statefulset", rendered: "---\napiVersion: apps/v1\nkind: StatefulSet\n", expected: true},
		{name: "unwatched workload kind", rendered: "apiVersion: apps/v1\nkind: DaemonSet\n", expected: false},
		{name: "foreign group", rendered: "apiVersion: example.io/v1\nkind: Deployment\n", expected: false},
		{name: "json syntax error terminates", rendered: `{"kind":"A"} {"kind":"B"} {bad`, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, hasWorkloads(tt.rendered))
		})
	}
}
