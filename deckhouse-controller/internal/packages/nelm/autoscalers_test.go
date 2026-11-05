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

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
)

// autoscaledManifests renders a VerticalPodAutoscaler ahead of its Deployment, whose "*"
// policy narrows the resources and whose sidecar policy is Off; a VerticalPodAutoscaler in
// update mode Off; one without policies for a CronJob; a HorizontalPodAutoscaler of each
// API version; and one of a foreign group.
const autoscaledManifests = `
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: web
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  updatePolicy:
    updateMode: InPlaceOrRecreate
  resourcePolicy:
    containerPolicies:
      - containerName: "*"
        controlledResources: [memory, ephemeral-storage]
      - containerName: sidecar
        mode: "Off"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      initContainers:
        - name: migrate
      containers:
        - name: app
        - name: sidecar
---
# A document of comments only.
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: db
spec:
  template:
    spec:
      containers:
        - name: postgres
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: db
spec:
  targetRef:
    apiVersion: apps/v1
    kind: StatefulSet
    name: db
  updatePolicy:
    updateMode: "Off"
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: backup
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: backup
spec:
  targetRef:
    apiVersion: batch/v1
    kind: CronJob
    name: backup
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  maxReplicas: 5
---
apiVersion: autoscaling/v1
kind: HorizontalPodAutoscaler
metadata:
  name: db
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: StatefulSet
    name: db
  maxReplicas: 3
---
apiVersion: autoscaling.example.com/v1
kind: HorizontalPodAutoscaler
metadata:
  name: foreign
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: foreign
`

// VPA sizes only the regular containers of its target, by the policy VPA picks for each,
// and nothing in update mode Off; every HorizontalPodAutoscaler of the autoscaling group
// scales its target.
func TestReadAutoscaled(t *testing.T) {
	autoscaled, err := readAutoscaled(autoscaledManifests)
	require.NoError(t, err)

	require.Equal(t, resourcesize.Autoscaled{
		Replicas: []resourcesize.AutoscaledReplicas{
			{Kind: "Deployment", Name: "web"},
			{Kind: "StatefulSet", Name: "db"},
		},
		Resources: []resourcesize.AutoscaledResources{
			{Kind: "Deployment", Name: "web", Container: "app", Resources: []string{"memory"}},
			{Kind: "CronJob", Name: "backup", Container: "backup", Resources: []string{"cpu", "memory"}},
		},
	}, autoscaled)
}

func TestReadAutoscaledWithoutAutoscalers(t *testing.T) {
	autoscaled, err := readAutoscaled(`
apiVersion: v1
kind: Pod
metadata:
  name: probe
spec:
  containers:
    - name: probe
`)
	require.NoError(t, err)
	require.Equal(t, resourcesize.Autoscaled{}, autoscaled)
}

// A document the release cannot be read past fails the read rather than leave the
// autoscalers after it unknown, which would fall their ranges back for nothing.
func TestReadAutoscaledFailsOnUndecodableDocument(t *testing.T) {
	_, err := readAutoscaled("kind: [Deployment")
	require.ErrorContains(t, err, "decode rendered manifests")
}
