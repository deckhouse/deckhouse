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

package hooks

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

// The Dex Passwords of local Users, which the chart of 1.76 rendered from
// userAuthn.internal.dexUsersCRDs. Pruned, they are recreated by the controller from
// User.spec.password, the password the User was created with: every password changed in Dex since
// then, the password history and the lockout state are lost.
//
// Only the Passwords that are not protected yet are returned. The controller keeps the keep policy
// on every Password it reconciles and never takes it off, so a protected Password stays protected,
// and counting it as owned would keep the hook listing Passwords on every run.

const dexNamespace = "d8-user-authn"

var passwordGVR = schema.GroupVersionResource{Group: "dex.coreos.com", Version: "v1", Resource: "passwords"}

var dexPasswords = handover.Kind{
	Name:     "Password",
	Resource: passwordGVR,
	Rendered: func(ctx context.Context, c dynamic.Interface) ([]handover.Ref, error) {
		return handover.List(ctx, c, passwordGVR, dexNamespace, "", func(obj *unstructured.Unstructured) bool {
			annotations := obj.GetAnnotations()
			return annotations[handover.ReleaseNameAnnotation] == handover.ReleaseName &&
				annotations[handover.ResourcePolicyAnnotation] != handover.ResourcePolicyKeep
		})
	},
}
