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

package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// Two controllers measure by this bound: the one that runs the eviction and the
// one that waits for it. nodeDrainTimeoutSecond is a plain integer, so past
// roughly 9.2e9 seconds the multiplication into a Duration wraps negative,
// which would put the deadline before the drain started.
func TestDrainTimeout(t *testing.T) {
	tests := []struct {
		name    string
		ngName  string
		seconds *int
		exp     time.Duration
	}{
		{name: "a node in no group", ngName: "", exp: DefaultDrainTimeout},
		{name: "a group that does not exist", ngName: "gone", exp: DefaultDrainTimeout},
		{name: "a group naming no timeout", ngName: "worker", exp: DefaultDrainTimeout},
		{name: "zero", ngName: "worker", seconds: ptr.To(0), exp: DefaultDrainTimeout},
		{name: "negative", ngName: "worker", seconds: ptr.To(-1), exp: DefaultDrainTimeout},
		{name: "ordinary", ngName: "worker", seconds: ptr.To(300), exp: 300 * time.Second},
		{name: "overflowing a duration", ngName: "worker", seconds: ptr.To(9223372037), exp: maxDrainTimeout},
		{name: "the largest int", ngName: "worker", seconds: ptr.To(int(^uint(0) >> 1)), exp: maxDrainTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1.AddToScheme(scheme))
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic, NodeDrainTimeoutSecond: tc.seconds},
			}).Build()

			got := DrainTimeout(context.Background(), c, tc.ngName)

			require.Equal(t, tc.exp, got)
			require.Positive(t, got, "a drain bound must never be zero or negative")
		})
	}
}
