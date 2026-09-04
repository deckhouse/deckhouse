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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/go_lib/project"
)

func TestModuleVersion(t *testing.T) {
	sc, err := project.Scheme()
	require.NoError(t, err)

	cli := fake.NewClientBuilder().WithScheme(sc).WithObjects(
		&v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "installed"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "deckhouse-modules", PackageVersion: "v1.2.3"},
		},
		&v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "dev", Annotations: map[string]string{v1alpha2.ModuleAnnotationDev: "true"}},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "deckhouse-modules", PackageVersion: "pr123"},
		},
		&v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{Name: "offered"},
			Spec:       v1alpha2.ModuleSpec{PackageRepositoryName: "deckhouse-modules"},
		},
	).Build()

	version, err := moduleVersion(context.Background(), cli, "installed")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", version)

	version, err = moduleVersion(context.Background(), cli, "dev")
	require.NoError(t, err)
	assert.Equal(t, defaultModuleVersion, version)

	// a module a source offers and nothing installed satisfies no dependency, like a module without an object
	_, err = moduleVersion(context.Background(), cli, "offered")
	require.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err), err.Error())

	_, err = moduleVersion(context.Background(), cli, "absent")
	require.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err), err.Error())

	var _ client.Client = cli
}
