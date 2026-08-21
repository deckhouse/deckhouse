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

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	PasswordGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "Password",
	}
	OfflineSessionsGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "OfflineSessions",
	}
	RefreshTokenGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "RefreshToken",
	}
	UserGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "User",
	}
	GroupGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "Group",
	}
	DexProviderGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "DexProvider",
	}
	UserOperationGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "UserOperation",
	}
	NamespaceGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}
	SecretGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Secret",
	}
)
