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
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// NodeStaticPodRequest runs one static pod on the selected nodes without the scheduler.
// node-manager adds the manifest to the NodeConfig of each selected node. A user who can
// create the object runs any manifest as root on every node it selects.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=nspr
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Phase,jsonPath=.status.phase,type=string
// +kubebuilder:printcolumn:name=Matched,jsonPath=.status.matchedNodes,type=integer
// +kubebuilder:printcolumn:name=Applied,jsonPath=.status.appliedNodes,type=integer
// +kubebuilder:printcolumn:name=Failed,jsonPath=.status.failedNodes,type=integer
// +kubebuilder:printcolumn:name=Age,jsonPath=.metadata.creationTimestamp,type=date
type NodeStaticPodRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeStaticPodRequestSpec   `json:"spec"`
	Status NodeStaticPodRequestStatus `json:"status,omitempty"`
}

// NodeStaticPodRequestSpec describes what to run and where. The platform ships the image
// (in a system extension) and decides who owns containerd's registry.d. The spec has no
// fields for either.
type NodeStaticPodRequestSpec struct {
	// NodeGroupSelector narrows the pod to the named NodeGroups. Empty selects
	// every group.
	// +optional
	NodeGroupSelector NodeGroupSelector `json:"nodeGroupSelector,omitempty"`

	// Manifest is the complete Pod manifest. $MY_IP in it is replaced with the node's
	// address. Its metadata.name and metadata.namespace must be set and do not depend on
	// the object name, which is only the manifest file name on the node. If two objects
	// define the same pod, the younger one is not accepted.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32768
	Manifest string `json:"manifest"`
}

// NodeStaticPodRequestStatus is written by node-controller. The Accepted condition shows
// whether node-controller accepted the object, with the reason Accepted, InvalidName,
// ReservedName, InvalidManifest, Conflict or LimitExceeded. The Ready condition shows
// whether the selected nodes wrote the current manifest.
type NodeStaticPodRequestStatus struct {
	// ObservedGeneration is the generation of the spec that corresponds to this status.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is Ready when the Ready condition is True. It is Degraded when Accepted is
	// False or Ready is False with RefusedByNodes. Otherwise it is Pending.
	// +kubebuilder:validation:Enum=Pending;Ready;Degraded
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions are Accepted and Ready. Ready is True with AllNodesApplied,
	// NoNodesSelected or ServedByBashible, and False with RolloutInProgress, RefusedByNodes
	// or NotAccepted. Its message names the refusing nodes or up to five pending ones and
	// is cut to 4096 bytes.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// MatchedNodeGroups are the NodeGroups the selector currently matches.
	// +optional
	MatchedNodeGroups []string `json:"matchedNodeGroups,omitempty"`

	// AppliedNodes is how many of the selected nodes report the current manifest written, by its
	// digest. FailedNodes is how many refused it or the revision that contains it, or had the
	// approvals for that revision fail. The counters have no omitempty, so the printer columns show 0.
	// +optional
	// +kubebuilder:validation:Minimum=0
	AppliedNodes int32 `json:"appliedNodes"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	FailedNodes int32 `json:"failedNodes"`

	// MatchedNodes is how many nodes of the selected groups the agent writes the
	// pod to, leaving out those of groups served by bashible; PendingNodes how many
	// of them have reported neither success nor refusal yet.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MatchedNodes int32 `json:"matchedNodes"`
	// +optional
	// +kubebuilder:validation:Minimum=0
	PendingNodes int32 `json:"pendingNodes"`

	// Nodes are the selected nodes that have not written the current manifest, at most
	// MaxStatusNodes. Failed nodes come first, then each state is sorted by name.
	// +optional
	// +kubebuilder:validation:MaxItems=32
	Nodes []RequestNodeStatus `json:"nodes,omitempty"`
}

// NodeStaticPodRequestList is a list of NodeStaticPodRequest objects.
//
// +kubebuilder:object:root=true
type NodeStaticPodRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeStaticPodRequest `json:"items"`
}

// reservedStaticPods maps each manifest that already has a writer of
// /etc/kubernetes/manifests to the pod it runs: the control-plane four (nodelet;
// bashible cluster-bootstrap 050, 072) and what the bashible steps 051, 052 and
// 020/070 write on every mutable node. Mirrors reservedPods in nodelet
// internal/controllers/staticpods/controller.go.
var reservedStaticPods = map[string]string{
	"etcd":                    "kube-system/etcd",
	"kube-apiserver":          "kube-system/kube-apiserver",
	"kube-controller-manager": "kube-system/kube-controller-manager",
	"kube-scheduler":          "kube-system/kube-scheduler",
	"kubernetes-api-proxy":    "kube-system/kubernetes-api-proxy",
	"registry-proxy":          "kube-system/registry-proxy",
	"registry-nodeservices":   "d8-system/registry-nodeservices",
}

// IsReservedStaticPodName reports whether a static pod name already has a writer.
func IsReservedStaticPodName(name string) bool {
	_, reserved := reservedStaticPods[name]
	return reserved
}

// IsReservedStaticPod reports whether a pod ("namespace/name") already has a
// manifest: kubelet collides on the pod, whatever the file is called.
func IsReservedStaticPod(pod string) bool {
	return slices.Contains(slices.Collect(maps.Values(reservedStaticPods)), pod)
}

// reservedStaticPodsBySystemType adds, per node system type, the manifests a
// bashible step writes there and nowhere else: registry-agent on the nodes bashible
// manages (candi/bashible/common-steps/all/053_configure_registry_agent.sh.tpl).
var reservedStaticPodsBySystemType = map[v1.SystemType]map[string]string{
	v1.SystemTypeMutable: {"registry-agent": "kube-system/registry-agent"},
}

// IsReservedStaticPodOn reports whether a manifest already has a writer on the nodes
// of this system type, by its file name or by its pod: kubelet collides on either.
func IsReservedStaticPodOn(systemType v1.SystemType, name, pod string) bool {
	reserved := reservedStaticPodsBySystemType[systemType]
	if _, byName := reserved[name]; byName {
		return true
	}
	for _, reservedPod := range reserved {
		if reservedPod == pod {
			return true
		}
	}
	return false
}

// ValidateStaticPodName refuses a name spec.staticPods[].name would not take: a
// DNS label of at most 63 characters (api/internal.deckhouse.io/v1alpha1
// nodeconfig_types.go, StaticPod.Name), which a CR name is free not to be.
func ValidateStaticPodName(name string) error {
	problems := validation.IsDNS1123Label(name)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%q cannot be a static pod file name: %s", name, strings.Join(problems, ", "))
}

// podDeserializer knows core/v1 and nothing else, so "is this a Pod?" is one
// question. Not strict: kubelet runs the document with its own types, and a
// field newer than the vendored k8s.io/api must not make the manifest invalid.
var podDeserializer = func() runtime.Decoder {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	return serializer.NewCodecFactory(scheme).UniversalDeserializer()
}()

// ValidateStaticPodManifest checks the document is a Pod kubelet can run as a static pod
// and returns its "namespace/name", the identity two objects may not share. Not strict:
// kubelet runs it with its own types, and an unknown field must not cost the node its config.
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
	if err := validateStaticPod(pod); err != nil {
		return "", fmt.Errorf("manifest is not a valid static pod: %w", err)
	}
	return pod.Namespace + "/" + pod.Name, nil
}

// validateStaticPod keeps the mirror pod out of tenant namespaces and refuses what kubelet
// cannot run without the API server: the NodeRestriction admission refuses the mirror pod
// of a pod that references API objects.
func validateStaticPod(pod *corev1.Pod) error {
	if problems := validation.IsDNS1123Subdomain(pod.Name); len(problems) > 0 {
		return fmt.Errorf("metadata.name %q is not a DNS-1123 subdomain: %s", pod.Name, strings.Join(problems, ", "))
	}
	if problems := validation.IsDNS1123Label(pod.Namespace); len(problems) > 0 {
		return fmt.Errorf("metadata.namespace %q is not a DNS-1123 label: %s", pod.Namespace, strings.Join(problems, ", "))
	}
	if pod.Namespace != "kube-system" && !strings.HasPrefix(pod.Namespace, "d8-") {
		return fmt.Errorf("metadata.namespace %q is neither kube-system nor a d8- namespace", pod.Namespace)
	}
	if pod.Spec.NodeName != "" {
		return errors.New("spec.nodeName is set: one manifest serves every selected node, and kubelet fills the field in")
	}
	if path := apiObjectReference(pod.Spec); path != "" {
		return fmt.Errorf("%s references an API object, which a static pod may not do", path)
	}
	return nil
}

// apiObjectReference returns the path of the first field that names an API object, or
// "" when there is none.
func apiObjectReference(spec corev1.PodSpec) string {
	if spec.ServiceAccountName != "" {
		return "spec.serviceAccountName"
	}
	if spec.DeprecatedServiceAccount != "" {
		return "spec.serviceAccount"
	}
	for i, volume := range spec.Volumes {
		for _, source := range []struct {
			field string
			set   bool
		}{
			{field: "secret", set: volume.Secret != nil},
			{field: "configMap", set: volume.ConfigMap != nil},
			{field: "persistentVolumeClaim", set: volume.PersistentVolumeClaim != nil},
			{field: "projected", set: volume.Projected != nil},
		} {
			if source.set {
				return fmt.Sprintf("spec.volumes[%d].%s", i, source.field)
			}
		}
	}
	for i, container := range spec.InitContainers {
		if path := envReference(fmt.Sprintf("spec.initContainers[%d]", i), container.Env, container.EnvFrom); path != "" {
			return path
		}
	}
	for i, container := range spec.Containers {
		if path := envReference(fmt.Sprintf("spec.containers[%d]", i), container.Env, container.EnvFrom); path != "" {
			return path
		}
	}
	return ""
}

// envReference returns the path of the first environment of a container taken from a
// Secret or a ConfigMap, or "" when there is none.
func envReference(container string, env []corev1.EnvVar, envFrom []corev1.EnvFromSource) string {
	if len(envFrom) > 0 {
		return container + ".envFrom[0]"
	}
	for i, variable := range env {
		if variable.ValueFrom == nil {
			continue
		}
		if variable.ValueFrom.SecretKeyRef != nil {
			return fmt.Sprintf("%s.env[%d].valueFrom.secretKeyRef", container, i)
		}
		if variable.ValueFrom.ConfigMapKeyRef != nil {
			return fmt.Sprintf("%s.env[%d].valueFrom.configMapKeyRef", container, i)
		}
	}
	return ""
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
