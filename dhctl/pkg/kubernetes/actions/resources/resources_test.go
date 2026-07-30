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

package resources

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func TestReadinessListing(t *testing.T) {
	const content = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-config
  namespace: d8-system
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: cluster-x
spec:
  nodeType: Static
---
`

	resources, err := template.ParseResourcesContent(t.Context(), content, nil)
	require.NoError(t, err)

	// Replicas turn on the cluster bootstrap check, which is not backed by an object.
	cnf := &config.MetaConfig{
		TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{{Replicas: 1, Name: "terra"}},
	}

	checkers, err := GetCheckers(t.Context(), nil, resources, cnf)
	require.NoError(t, err)
	require.Len(t, checkers, 4)
	require.Equal(t, "cluster", checkers[0].Name())

	for _, tc := range []struct {
		name     string
		checkers []Checker
		want     []string
	}{
		{
			name:     "no checks",
			checkers: nil,
			want:     []string{},
		},
		{
			name:     "timeout listing keeps the cluster check",
			checkers: checkers,
			want: []string{
				"Cluster bootstrap:                          waiting for a Ready worker node",
				"ConfigMap:                                  d8-system/cluster-config",
				"ClusterAuthorizationRule (deckhouse.io/v1): admin",
				"NodeGroup (deckhouse.io/v1):                cluster-x",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, checkerLines(tc.checkers))
		})
	}

	t.Run("each listing holds what it was given", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&buf, nil)))

		logResources(ctx, checkers[:2], checkers[2:])

		notReady, ready, split := strings.Cut(buf.String(), `"Starting: Resource ready"`)
		require.True(t, split, "both listings are printed")

		// The handler escapes the joined lines, so the listing arrives as one \n-separated message.
		require.Contains(t, notReady, `"Starting: Resource not ready"`)
		require.Contains(t, notReady, strings.Join(checkerLines(checkers[1:2]), `\n`))
		require.Contains(t, ready, strings.Join(checkerLines(checkers[2:]), `\n`))

		require.NotContains(t, buf.String(), "Cluster bootstrap:", "the cluster check is dropped")
		require.Contains(t, buf.String(), "cluster-x", "a resource named like the cluster check is not")

		// CreateResourcesLoop reuses both slices on every iteration.
		require.Len(t, checkers, 4)
		require.Equal(t, "cluster", checkers[0].Name())
	})

	t.Run("nothing but the cluster check", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&buf, nil)))

		logResources(ctx, checkers[:1], nil)

		require.Empty(t, buf.String(), "no empty framed block")
	})
}
