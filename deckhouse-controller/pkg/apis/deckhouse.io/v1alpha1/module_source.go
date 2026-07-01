/*
Copyright 2023 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ModuleSourceResource = "modulesources"
	ModuleSourceKind     = "ModuleSource"

	ModuleSourcePhaseActive      = "Active"
	ModuleSourcePhaseTerminating = "Terminating"

	ModuleSourceMessageErrors = "Some errors occurred. Inspect status for details"

	ModuleSourceFinalizerReleaseExists = "modules.deckhouse.io/release-exists"
	ModuleSourceFinalizerModuleExists  = "modules.deckhouse.io/module-exists"

	ModuleSourceAnnotationForceDelete      = "modules.deckhouse.io/force-delete"
	ModuleSourceAnnotationRegistryChecksum = "modules.deckhouse.io/registry-spec-checksum"
)

var (
	ModuleSourceGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleSourceResource,
	}
	ModuleSourceGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleSourceKind,
	}
)

var _ runtime.Object = (*ModuleSource)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=ms
// +kubebuilder:printcolumn:name="count",type="integer",JSONPath=".status.modulesCount",description="The number of modules available."
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.phase",description="The current phase."
// +kubebuilder:printcolumn:name="sync",type="date",format="date-time",JSONPath=".status.syncTime",description="When the repository was synchronized."
// +kubebuilder:printcolumn:name="msg",type="string",JSONPath=".status.message",description="The error message if exists."
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/name=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=deckhouse"
// +kubebuilder:metadata:labels="backup.deckhouse.io/cluster-config=true"
// +crd-enricher:crd:preserveUnknownFields=false
// +crd-enricher:crd:minimal=true
// +crd-enricher:crd:stripFormat=true
// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: ModuleSource, metadata: {name: example}, spec: {registry: {repo: registry.example.io/modules-source, dockerCfg: "<base64 encoded credentials>"}}}

// Defines the configuration of a source of Deckhouse modules.
//
// For more information about installing the module from the source, see the section ["Running the module in the DKP cluster"](../../architecture/module-development/run/#running-the-module-in-the-dkp-cluster).
type ModuleSource struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleSourceSpec `json:"spec"`

	Status ModuleSourceStatus `json:"status,omitempty"`
}

type ModuleSourceSpec struct {
	// Desirable default release channel for modules in the current source.
	// +crd-enricher:deckhouse:documentation:deprecated=true
	ReleaseChannel string `json:"releaseChannel,omitempty"`

	// Interval for registry scan.
	//
	// Defines the frequency of checking the container registry for new modules and their versions.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern=`^(\d+h)?(\d+m)?(\d+s)?$`
	// +crd-enricher:deckhouse:documentation:default=3m
	// +crd-enricher:deckhouse:documentation:examples=5m
	// +crd-enricher:deckhouse:documentation:examples=1h
	// +crd-enricher:deckhouse:documentation:examples=6h30m
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`

	Registry ModuleSourceSpecRegistry `json:"registry"`
}

type ModuleSourceSpecRegistry struct {
	// Protocol to access the registry.
	// +kubebuilder:default=HTTPS
	// +kubebuilder:validation:Enum=HTTP;HTTPS
	Scheme string `json:"scheme,omitempty"`

	// URL of the container registry.
	// +crd-enricher:deckhouse:documentation:examples=registry.example.io/deckhouse/modules
	Repo string `json:"repo"`

	// Container registry access token in Base64. If using anonymous access to the container registry, do not fill in this field.
	// +crd-enricher:deckhouse:sensitive-data
	DockerCFG string `json:"dockerCfg,omitempty"`

	// Root CA certificate (PEM format) to validate the registry’s HTTPS certificate (if self-signed certificates are used).
	// > Creating a ModuleSource resource with the CA certificate spec will cause the container to restart on all nodes.
	CA string `json:"ca,omitempty"`
}

type ModuleSourceStatus struct {
	// When the repository was synchronized.
	SyncTime metav1.Time `json:"syncTime,omitempty"`
	// The number of modules available.
	ModulesCount int `json:"modulesCount,omitempty"`
	// The list of modules available from the source and their update policies.
	AvailableModules []AvailableModule `json:"modules,omitempty"`
	// The current phase.
	// +kubebuilder:validation:Enum=Active;Terminating
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

type AvailableModule struct {
	// The module name.
	Name string `json:"name,omitempty"`
	// The module version.
	Version string `json:"version,omitempty"`
	// The module policy name.
	Policy string `json:"policy,omitempty"`
	// The module checksum.
	Checksum string `json:"checksum,omitempty"`
	// The module processing error.
	Error string `json:"error,omitempty"`
	// Deprecated: use Error instead
	PullError string `json:"pullError,omitempty"` // Deprecated: use Error instead
	// If ModulePullOverride for this module exists.
	Overridden bool `json:"overridden,omitempty"`
}

// +kubebuilder:object:root=true

// ModuleSourceList is a list of ModuleSource resources
type ModuleSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleSource `json:"items"`
}
