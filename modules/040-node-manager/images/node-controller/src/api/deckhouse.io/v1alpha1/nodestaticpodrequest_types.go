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

package v1alpha1

import (
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// NodeStaticPodRequest asks kubelet on the selected nodes to run one pod outside the
// scheduler: it is started from a manifest on disk, so it runs before the API
// server answers and before CNI is up. The object carries the manifest and
// nothing else — whatever the pod needs beyond an image in containerd and a file
// in the manifests directory, it brings itself. Whoever may create one runs a
// manifest of their choosing, as root, on every node it selects; the object is
// the platform's, and the module's RBAC grants create to nobody else.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=nspr
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Phase,jsonPath=.status.phase,type=string
// +kubebuilder:printcolumn:name=Age,jsonPath=.metadata.creationTimestamp,type=date
type NodeStaticPodRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeStaticPodRequestSpec   `json:"spec"`
	Status NodeStaticPodRequestStatus `json:"status,omitempty"`
}

// NodeStaticPodRequestSpec describes what to run and where, and that is the whole of
// it. Preloading the image and deciding who owns containerd's registry.d are
// platform decisions node-controller makes from which modules are enabled, not
// things a module declares here.
type NodeStaticPodRequestSpec struct {
	// NodeGroupSelector narrows the pod to the named NodeGroups. Empty selects
	// every group.
	// +optional
	NodeGroupSelector NodeGroupSelector `json:"nodeGroupSelector,omitempty"`

	// Manifest is the whole Pod document; $MY_IP is the node's address. Its
	// metadata.name and metadata.namespace must be set and have nothing to do
	// with this object's name, which is only the manifest's file name on the
	// node. Two objects whose manifests name one pod are a document the node
	// would refuse whole, so the younger of them is refused here instead.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32768
	Manifest string `json:"manifest"`
}

// NodeStaticPodRequestStatus is written by node-controller as it checks the
// manifest, settles the pod it names against the other objects, and matches the
// selector. Mirrors the NodeExtensionRequest status shape: a phase, typed
// conditions, the observed generation and what the nodes report.
type NodeStaticPodRequestStatus struct {
	// ObservedGeneration is the generation of the spec this status reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is Ready when the pod resolved and the nodes accept it, Degraded
	// when it did not — the Ready condition carries the reason.
	// +kubebuilder:validation:Enum=Ready;Degraded
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions carry the details. Ready answers whether the pod resolved:
	// Resolved, ReservedName, InvalidManifest, Conflict or RefusedByNodes.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// MatchedNodeGroups are the NodeGroups the selector currently matches.
	// +optional
	MatchedNodeGroups []string `json:"matchedNodeGroups,omitempty"`

	// AppliedNodes is how many of the selected nodes report the manifest
	// written; FailedNodes how many report it refused. Reported by the nodes
	// themselves: a pod can resolve here and be refused by every node.
	// +optional
	AppliedNodes int32 `json:"appliedNodes,omitempty"`
	// +optional
	FailedNodes int32 `json:"failedNodes,omitempty"`

	// FailureMessage is what the nodes say about the refusal, taken from one of
	// them: a bad manifest fails the same way everywhere.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
}

// NodeStaticPodRequestList is a list of NodeStaticPodRequest objects.
//
// +kubebuilder:object:root=true
type NodeStaticPodRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeStaticPodRequest `json:"items"`
}

// IsReservedStaticPodName reports whether a static pod name belongs to the
// control plane, whose manifests the node agent writes itself. Kept next to the
// contract so the admission webhook and the controller backstop enforce one list.
func IsReservedStaticPodName(name string) bool {
	return slices.Contains([]string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"}, name)
}

// podDeserializer decodes a manifest the way the API server decodes a request
// body: apiVersion and kind pick the type, and the document is then read into
// it. The scheme knows core/v1 and nothing else, which is what makes "is this a
// Pod?" a single question — a Deployment names a group this scheme never heard
// of and fails to decode at all.
//
// Not strict: the document is executed by a kubelet of its own version, and a
// strict decode would refuse a Pod field newer than the k8s.io/api this binary
// vendors — on every node the document reaches, as a rejected NodeConfig rather
// than as one bad manifest.
var podDeserializer = func() runtime.Decoder {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	return serializer.NewCodecFactory(scheme).UniversalDeserializer()
}()

// ValidateStaticPodManifest checks that a manifest is a valid Pod with a name
// and a namespace, and returns the "namespace/name" of the pod it declares — the
// key two objects can collide on, handed back rather than decoded a second time.
//
// One question and one answer: either the document is a Pod or it is not, and
// the decoder's own error says why. Taking the document apart to report which
// field offended would be a second, worse copy of the validation the API server
// already performs — and every sentence of it would be one more thing to keep in
// step with the loader in nodelet, which asks exactly this.
//
// Decoded with a nil "into" on purpose. Handing it a &corev1.Pod{} would let the
// decoder fill a missing kind in from the destination type, so a document that
// names no kind at all would sail through as a Pod. With nil, the decoder answers
// "Object 'Kind' is missing in ..." itself, and the kind it did find comes back
// as the GroupVersionKind — which is what a wrong kind is reported as.
//
// The object's own name is deliberately not compared with the pod's: it is the
// manifest's file name on the node and nothing else. kubelet names the mirror
// pod from the document, never from the file.
func ValidateStaticPodManifest(manifest string) (string, error) {
	object, gvk, err := podDeserializer.Decode([]byte(manifest), nil, nil)
	if err != nil {
		return "", fmt.Errorf("manifest is not a valid Pod: %w", err)
	}
	pod, ok := object.(*corev1.Pod)
	if !ok {
		return "", fmt.Errorf("manifest is not a valid Pod: unexpected kind %s", gvk)
	}
	if pod.Name == "" {
		return "", errors.New("manifest is not a valid Pod: metadata.name is empty")
	}
	if pod.Namespace == "" {
		return "", errors.New("manifest is not a valid Pod: metadata.namespace is empty")
	}
	return pod.Namespace + "/" + pod.Name, nil
}

func init() {
	SchemeBuilder.Register(&NodeStaticPodRequest{}, &NodeStaticPodRequestList{})
}
