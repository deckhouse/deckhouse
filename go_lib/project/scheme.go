// Copyright 2024 Flant JSC
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

package project

import (
	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

var schemaBuilder = runtime.NewSchemeBuilder(
	v1alpha1.AddToScheme,
	v1alpha2.AddToScheme,
	coordv1.AddToScheme,
	appsv1.AddToScheme,
	corev1.AddToScheme,
	apiextensionsv1.AddToScheme,
)

func AddToScheme(scheme *runtime.Scheme) error {
	return schemaBuilder.AddToScheme(scheme)
}

func Scheme() (*runtime.Scheme, error) {
	sc := runtime.NewScheme()
	return sc, AddToScheme(sc)
}
