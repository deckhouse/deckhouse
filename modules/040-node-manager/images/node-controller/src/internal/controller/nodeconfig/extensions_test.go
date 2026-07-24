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

package nodeconfig

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

const drbdDigest = "sha256:5b6821d5e191ed505ece10ecfc3d17a85d7f57c360e1ace11f3d46aad6203842"

func ner(name string, spec deckhousev1alpha1.NodeExtensionRequestSpec, labels map[string]string) deckhousev1alpha1.NodeExtensionRequest {
	return deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       spec,
	}
}

func nodeWith(labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: labels}}
}

func moduleLabel(module string) map[string]string {
	return map[string]string{moduleNameLabel: module}
}

func TestNodeExtensions(t *testing.T) {
	moduleSourceRepos := map[string]string{
		"deckhouse": "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
	}

	// The extension the drbd requests below resolve to: repository is the
	// ModuleSource repo (the proxy's auth key), the path is the module name, and
	// the name is the logical sysext.
	drbdExtension := internalv1alpha1.Extension{
		Name:           "drbd",
		Repository:     "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
		AdditionalPath: "sds-replicated-volume",
		Digest:         drbdDigest,
		RequestedBy:    "sds-replicated-volume",
	}

	tests := []struct {
		name           string
		ners           []deckhousev1alpha1.NodeExtensionRequest
		node           *corev1.Node
		ngName         string
		wantExtensions []internalv1alpha1.Extension
		wantModules    []internalv1alpha1.KernelModule
	}{
		{
			name: "path defaults to the module name and matched by nodeGroupSelector",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:            deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"worker"}},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:           nodeWith(nil),
			ngName:         "worker",
			wantExtensions: []internalv1alpha1.Extension{drbdExtension},
		},
		{
			name: "not matched when nodeGroupSelector names another group",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:            deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"storage"}},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "matched by nodeSelector labels",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:       deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:           nodeWith(map[string]string{"storage": "drbd"}),
			ngName:         "worker",
			wantExtensions: []internalv1alpha1.Extension{drbdExtension},
		},
		{
			name: "not matched when a required label is missing",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext:       deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:   nodeWith(map[string]string{"role": "worker"}),
			ngName: "worker",
		},
		{
			name: "explicit path overrides the module name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest, Path: "custom/drbd-path"},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:           "drbd",
				Repository:     "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
				AdditionalPath: "custom/drbd-path",
				Digest:         drbdDigest,
				RequestedBy:    "sds-replicated-volume",
			}},
		},
		{
			name: "unknown ModuleSource drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest, ModuleSource: "nonexistent"},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "no path and no module label drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
				}, nil),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "missing digest drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd"},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "extensions deduplicated by name, first wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd-a", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
				}, moduleLabel("sds-replicated-volume")),
				ner("drbd-b", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000"},
				}, moduleLabel("other-module")),
			},
			node:           nodeWith(nil),
			ngName:         "worker",
			wantExtensions: []internalv1alpha1.Extension{drbdExtension},
		},
		{
			name: "kernel modules collected and deduplicated by name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					Sysext: deckhousev1alpha1.Sysext{Name: "drbd", Digest: drbdDigest},
					KernelModules: []deckhousev1alpha1.KernelModule{
						{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
						{Name: "drbd_transport_tcp"},
					},
				}, moduleLabel("sds-replicated-volume")),
			},
			node:           nodeWith(nil),
			ngName:         "worker",
			wantExtensions: []internalv1alpha1.Extension{drbdExtension},
			wantModules: []internalv1alpha1.KernelModule{
				{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
				{Name: "drbd_transport_tcp"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensions, modules := nodeExtensions(tt.ners, tt.node, tt.ngName, moduleSourceRepos)
			if !reflect.DeepEqual(extensions, tt.wantExtensions) {
				t.Fatalf("extensions = %#v, want %#v", extensions, tt.wantExtensions)
			}
			if !reflect.DeepEqual(modules, tt.wantModules) {
				t.Fatalf("modules = %#v, want %#v", modules, tt.wantModules)
			}
		})
	}
}
