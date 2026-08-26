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

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func forbiddenByPolicy() error {
	return apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "d8-cloud-provider-dvp",
		apierrors.NewBadRequest("ValidatingAdmissionPolicy 'system-ns.deckhouse.io' with binding "+
			"'system-ns.deckhouse.io' denied request: Creation of system namespaces is forbidden"))
}

// Deckhouse's own policy denies CREATE of a d8-* namespace to everyone but
// Deckhouse, and admission answers before the API server can report the
// conflict. Reading that refusal as "the object is missing" leaves every re-run
// of the install retrying a call that can never succeed.
func TestCreateForbiddenFallsThroughToUpdate(t *testing.T) {
	updated := false
	task := ManifestTask{
		Name:       "Namespace \"d8-cloud-provider-dvp\"",
		Manifest:   func() any { return nil },
		CreateFunc: func(context.Context, any) error { return forbiddenByPolicy() },
		UpdateFunc: func(context.Context, any) error { updated = true; return nil },
	}

	require.NoError(t, task.CreateOrUpdateSilent(context.Background()))
	require.True(t, updated, "the update is what tells a refused create from a missing object")
}

// A create the operator genuinely may not make reports its own refusal, not the
// update's: the update fails for the same reason and says nothing new.
func TestCreateForbiddenReportsTheCreateWhenUpdateFailsToo(t *testing.T) {
	task := ManifestTask{
		Name:       "Namespace \"d8-cloud-provider-dvp\"",
		Manifest:   func() any { return nil },
		CreateFunc: func(context.Context, any) error { return forbiddenByPolicy() },
		UpdateFunc: func(context.Context, any) error { return apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "d8-cloud-provider-dvp") },
	}

	err := task.CreateOrUpdateSilent(context.Background())
	require.ErrorContains(t, err, "Creation of system namespaces is forbidden")
	require.NotContains(t, err.Error(), "not found")
}
