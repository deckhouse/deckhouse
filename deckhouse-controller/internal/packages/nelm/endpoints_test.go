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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

func TestExtractEndpointURLs(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
		expected []status.URL
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
    packages.deckhouse.io/is-application-endpoint: "true"
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
			expected: []status.URL{
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
    packages.deckhouse.io/is-application-endpoint: "Web UI"
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
			expected: []status.URL{
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
    packages.deckhouse.io/is-application-endpoint: "true"
spec:
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
`,
			expected: []status.URL{{URL: "http://app.example.com/"}},
		},
		{
			name: "rule without paths defaults to root",
			rendered: `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    packages.deckhouse.io/is-application-endpoint: "true"
spec:
  tls:
  - hosts:
    - app.example.com
  rules:
  - host: app.example.com
`,
			expected: []status.URL{{URL: "https://app.example.com/"}},
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
    packages.deckhouse.io/is-application-endpoint: "false"
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
    packages.deckhouse.io/is-application-endpoint: "true"
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
    packages.deckhouse.io/is-application-endpoint: "true"
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
    packages.deckhouse.io/is-application-endpoint: "Dashboard"
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
			expected: []status.URL{
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
    packages.deckhouse.io/is-application-endpoint: "true"
spec:
  rules:
  - host: app.example.com
    http:
      paths:
      - path: ui
`,
			expected: []status.URL{{URL: "http://app.example.com/ui"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractEndpointURLs(tt.rendered))
		})
	}
}
