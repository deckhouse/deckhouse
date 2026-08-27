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

package deckhouse

import (
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// The lock's UpdateFunc is a deliberate no-op, so anything that routes a failed create to it
// turns a denial into "OK!" and leaves the installer believing the Deckhouse queue is locked.
func TestALockThatWasDeniedIsNotReportedAsTaken(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	kubeCl.CoreV1().(interface {
		PrependReactor(verb, resource string, reaction k8stesting.ReactionFunc)
	}).PrependReactor("create", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"},
			"deckhouse-bootstrap-lock", apierrors.NewBadRequest("denied by admission"))
	})

	task, err := LockDeckhouseQueueBeforeCreatingModuleConfigs(t.Context(), kubeCl)
	require.NoError(t, err)
	require.NotNil(t, task)

	require.Error(t, task.CreateOrUpdate(t.Context()), "a denied create must not pass through the no-op update")
}
