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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
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
// +kubebuilder:printcolumn:name=Applied,jsonPath=.status.appliedNodes,type=integer
// +kubebuilder:printcolumn:name=Failed,jsonPath=.status.failedNodes,type=integer
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

// podDeserializer knows core/v1 and nothing else, so "is this a Pod?" is one
// question. Not strict: kubelet runs the document with its own types, and a
// field newer than the vendored k8s.io/api must not make the manifest invalid.
var podDeserializer = func() runtime.Decoder {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	return serializer.NewCodecFactory(scheme).UniversalDeserializer()
}()

// ValidateStaticPodManifest checks the document is a Pod and returns its "namespace/name",
// the identity two objects may not share. Deliberately not strict: kubelet runs it with its
// own types, and an unknown field must not cost the node its whole config.
func ValidateStaticPodManifest(manifest string) (string, error) {
	if err := refuseMultipleDocuments(manifest); err != nil {
		return "", err
	}
	// into=nil on purpose: a destination would let the decoder fill a missing
	// kind in from the type, so a document naming no kind would pass as a Pod.
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

// refuseMultipleDocuments refuses a manifest carrying more than one YAML document:
// the decoder reads the first and stops, so a second escaped every check. Mirrors
// refuseMultipleDocuments in nodelet internal/config/loader.go, message included.
func refuseMultipleDocuments(manifest string) error {
	reader := utilyaml.NewYAMLReader(bufio.NewReader(strings.NewReader(manifest)))
	documents := 0
	for {
		document, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("manifest is not a valid Pod: %w", err)
		}
		if len(bytes.TrimSpace(document)) == 0 {
			continue
		}
		documents++
		if documents > 1 {
			return errors.New("manifest is not a valid Pod: contains more than one document")
		}
	}
}

func init() {
	SchemeBuilder.Register(&NodeStaticPodRequest{}, &NodeStaticPodRequestList{})
}
