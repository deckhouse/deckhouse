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

const testKernelVersion = "6.12.85-lvc19"

func ner(name string, spec deckhousev1alpha1.NodeExtensionRequestSpec, labels map[string]string) deckhousev1alpha1.NodeExtensionRequest {
	return deckhousev1alpha1.NodeExtensionRequest{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       spec,
	}
}

func nodeWith(labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: labels}}
}

func TestNodeExtensions(t *testing.T) {
	const (
		drbdImage = "registry.example.com/deckhouse/sysext/drbd:${DRBD_VERSION}-k${KERNEL_VERSION}"
		fooImage  = "registry.example.com/deckhouse/sysext/foo:${FOO_VERSION}"
	)
	digests := map[string]string{
		"drbd": "sha256:aaa",
		"foo":  "sha256:bbb",
	}
	moduleSourceRepos := map[string]string{
		"deckhouse": "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
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
			name: "matched by nodeGroupSelector match names",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate:     drbdImage,
					Params:            map[string]string{"DRBD_VERSION": "9.2.14"},
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"worker"}},
				}, nil),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:           "drbd",
				Repository:     "registry.example.com",
				AdditionalPath: "deckhouse/sysext",
				Digest:         "sha256:aaa",
				RequestedBy:    "drbd",
			}},
		},
		{
			name: "not matched when nodeGroupSelector names another group",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate:     drbdImage,
					Params:            map[string]string{"DRBD_VERSION": "9.2.14"},
					NodeGroupSelector: deckhousev1alpha1.NodeGroupSelector{MatchNames: []string{"storage"}},
				}, nil),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "matched by nodeSelector match labels",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: drbdImage,
					Params:        map[string]string{"DRBD_VERSION": "9.2.14"},
					NodeSelector:  deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
				}, nil),
			},
			node:   nodeWith(map[string]string{"storage": "drbd"}),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:           "drbd",
				Repository:     "registry.example.com",
				AdditionalPath: "deckhouse/sysext",
				Digest:         "sha256:aaa",
				RequestedBy:    "drbd",
			}},
		},
		{
			name: "not matched when a required label is missing",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: drbdImage,
					Params:        map[string]string{"DRBD_VERSION": "9.2.14"},
					NodeSelector:  deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
				}, nil),
			},
			node:   nodeWith(map[string]string{"role": "worker"}),
			ngName: "worker",
		},
		{
			name: "param and KERNEL_VERSION substitution resolves the reference",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: drbdImage,
					Params:        map[string]string{"DRBD_VERSION": "9.2.14"},
				}, map[string]string{moduleNameLabel: "csi-drbd"}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:           "drbd",
				Repository:     "registry.example.com",
				AdditionalPath: "deckhouse/sysext",
				Digest:         "sha256:aaa",
				RequestedBy:    "csi-drbd",
			}},
		},
		{
			name: "unresolved placeholder skips the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: drbdImage,
					// DRBD_VERSION missing -> ${DRBD_VERSION} stays in the ref.
				}, nil),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "missing digest skips the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("bar", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: "registry.example.com/deckhouse/sysext/bar:${BAR_VERSION}",
					Params:        map[string]string{"BAR_VERSION": "1.0"},
				}, nil),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
		{
			name: "extensions deduplicated by name, first wins",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd-a", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: drbdImage,
					Params:        map[string]string{"DRBD_VERSION": "9.2.14"},
				}, map[string]string{moduleNameLabel: "first"}),
				ner("drbd-b", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: drbdImage,
					Params:        map[string]string{"DRBD_VERSION": "9.9.99"},
				}, map[string]string{moduleNameLabel: "second"}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:           "drbd",
				Repository:     "registry.example.com",
				AdditionalPath: "deckhouse/sysext",
				Digest:         "sha256:aaa",
				RequestedBy:    "first",
			}},
		},
		{
			name: "kernel modules collected and deduplicated by name",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: drbdImage,
					Params:        map[string]string{"DRBD_VERSION": "9.2.14"},
					KernelModules: []deckhousev1alpha1.KernelModule{
						{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
						{Name: "drbd_transport_tcp"},
					},
				}, nil),
				ner("foo", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: fooImage,
					Params:        map[string]string{"FOO_VERSION": "1.0"},
					KernelModules: []deckhousev1alpha1.KernelModule{
						{Name: "drbd", Params: []string{"ignored=1"}},
						{Name: "foo_mod"},
					},
				}, nil),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{
				{Name: "drbd", Repository: "registry.example.com", AdditionalPath: "deckhouse/sysext", Digest: "sha256:aaa", RequestedBy: "drbd"},
				{Name: "foo", Repository: "registry.example.com", AdditionalPath: "deckhouse/sysext", Digest: "sha256:bbb", RequestedBy: "foo"},
			},
			wantModules: []internalv1alpha1.KernelModule{
				{Name: "drbd", Params: []string{"usermode_helper=disabled"}},
				{Name: "drbd_transport_tcp"},
				{Name: "foo_mod"},
			},
		},
		{
			name: "module image resolves via ModuleSource repo with a pinned digest",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: "${MODULE_SOURCE_REPO}/sds-replicated-volume/drbd@sha256:cafe",
				}, map[string]string{moduleNameLabel: "sds-replicated-volume"}),
			},
			node:   nodeWith(nil),
			ngName: "worker",
			wantExtensions: []internalv1alpha1.Extension{{
				Name:           "drbd",
				Repository:     "dev-registry.deckhouse.io/sys/deckhouse-oss/modules",
				AdditionalPath: "sds-replicated-volume",
				Digest:         "sha256:cafe",
				RequestedBy:    "sds-replicated-volume",
			}},
		},
		{
			name: "unknown ModuleSource leaves the placeholder unresolved and drops the request",
			ners: []deckhousev1alpha1.NodeExtensionRequest{
				ner("drbd-sysext", deckhousev1alpha1.NodeExtensionRequestSpec{
					ImageTemplate: "${MODULE_SOURCE_REPO}/sds-replicated-volume/drbd-sysext@sha256:cafe",
					Params:        map[string]string{"MODULE_SOURCE": "nonexistent"},
				}, nil),
			},
			node:   nodeWith(nil),
			ngName: "worker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensions, modules := nodeExtensions(tt.ners, tt.node, tt.ngName, digests, testKernelVersion, moduleSourceRepos)
			if !reflect.DeepEqual(extensions, tt.wantExtensions) {
				t.Fatalf("extensions = %#v, want %#v", extensions, tt.wantExtensions)
			}
			if !reflect.DeepEqual(modules, tt.wantModules) {
				t.Fatalf("modules = %#v, want %#v", modules, tt.wantModules)
			}
		})
	}
}
