// Copyright 2021 Flant JSC
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
	"fmt"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManifestValidation(t *testing.T) {
	mt := ManifestTask{
		Name: "Test",
		Manifest: func() interface{} {
			return manifests.NewManifestWrapper(
				&apiv1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "loooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong-name",
					},
				},
				manifests.SecretNameLenghtValidator,
			)
		},
	}

	_, err := mt.GetValidManifest()
	if err == nil {
		t.Error(fmt.Errorf("manifest should be invalid"))
	}

	mt = ManifestTask{
		Name: "Test",
		Manifest: func() interface{} {
			return manifests.NewManifestWrapper(
				&apiv1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "short-name",
					},
				},
				manifests.SecretNameLenghtValidator,
			)
		},
	}

	_, err = mt.GetValidManifest()
	if err != nil {
		t.Error(err)
	}
}
