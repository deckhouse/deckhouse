/*
Copyright 2025 Flant JSC

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
	PackageRepositoryResource = "packagerepositories"
	PackageRepositoryKind     = "PackageRepository"

	PackageRepositoryPhaseActive      = "Active"
	PackageRepositoryPhaseTerminating = "Terminating"

	PackageRepositoryFinalizerPackageVersionExists = "packages.deckhouse.io/package-version-exists"

	PackageRepositoryConditionLastScanSucceeded = "LastScanSucceeded"
)

var (
	PackageRepositoryGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: PackageRepositoryResource,
	}
	PackageRepositoryGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    PackageRepositoryKind,
	}
)

var _ runtime.Object = (*PackageRepository)(nil)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name=Phase,type=string,JSONPath=.status.phase
// +kubebuilder:printcolumn:name=Scan,type=date,JSONPath=.status.lastScanTime
// +kubebuilder:printcolumn:name=Repository,type=string,JSONPath=.spec.registry.repo
// +kubebuilder:printcolumn:name=Packages,type=integer,JSONPath=.status.packagesCount
// +kubebuilder:printcolumn:name=MSG,type=string,JSONPath=.status.conditions[?(@.type=='LastScanSucceeded')].message
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nServers should convert recognized schemas to the latest internal value, and\nmay reject unrecognized values.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)."
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nServers may infer this from the endpoint the client submits requests to.\nCannot be updated.\nIn CamelCase.\n\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)."
// +crd-enricher:raw:properties.apiVersion.x-doc-skip=true
// +crd-enricher:raw:properties.kind.x-doc-skip=true
// +crd-enricher:raw:properties.metadata.x-doc-skip=true

// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: PackageRepository, metadata: {name: example}, spec: {registry: {ca: <PEM-encoded CA certificate>, dockerCfg: <base64 encoded credentials>, login: admin, password: <password>, repo: registry.example.io/packages, scheme: https}, scanInterval: 5m}}
// PackageRepository defines a package repository available in Deckhouse Platform.
type PackageRepository struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Package repository configuration.
	Spec PackageRepositorySpec `json:"spec"`

	// Current state of the package repository.
	Status PackageRepositoryStatus `json:"status,omitempty"`
}

type PackageRepositorySpec struct {
	// Interval between automatic scans of the package repository for discovering new packages and versions.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern=`^(\d+h)?(\d+m)?(\d+s)?$`
	// +crd-enricher:deckhouse:documentation:default=6h
	// +crd-enricher:deckhouse:documentation:examples=5m
	// +crd-enricher:deckhouse:documentation:examples=1h
	// +crd-enricher:deckhouse:documentation:examples=6h30m
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
	// Access parameters for the container registry where the package repository is located.
	Registry PackageRepositorySpecRegistry `json:"registry"`
}

type PackageRepositorySpecRegistry struct {
	// Protocol used to access the package repository, for example, `https`.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=https
	Scheme string `json:"scheme,omitempty"`

	// Address of the package repository in the container registry.
	// +crd-enricher:deckhouse:documentation:examples=registry.example.io/packages
	Repo string `json:"repo"`

	// Container registry access configuration in `~/.docker/config.json` format, encoded in Base64.
	//
	// Leave this field empty for anonymous access.
	// +optional
	// +crd-enricher:deckhouse:sensitive-data
	// +crd-enricher:deckhouse:documentation:examples=<base64 encoded credentials>
	DockerCFG string `json:"dockerCfg,omitempty"`

	// Root certificate in PEM format used to verify the TLS certificate of the container registry when connecting over HTTPS.
	//
	// Used if the registry uses self-signed SSL certificates.
	// +optional
	// +crd-enricher:deckhouse:documentation:examples=<PEM-encoded CA certificate>
	CA string `json:"ca,omitempty"`

	// Username for authenticating to the container registry.
	// +optional
	// +crd-enricher:deckhouse:sensitive-data
	// +crd-enricher:deckhouse:documentation:examples=admin
	Login string `json:"login,omitempty"`

	// Password for authenticating to the container registry.
	// +optional
	// +crd-enricher:deckhouse:sensitive-data
	// +crd-enricher:deckhouse:documentation:examples=<password>
	Password string `json:"password,omitempty"`
}

type PackageRepositoryStatus struct {
	// Time of the last successfully completed scan.
	//
	// If a scan fails, the value is not updated, and the error is reflected in the `LastScanSucceeded` condition.
	// +optional
	LastScanTime *metav1.Time `json:"lastScanTime,omitempty"`

	// Time of the last scan that discovered at least one new package version.
	//
	// If no new versions are discovered, the value is not updated.
	// +optional
	LastChangeTime *metav1.Time `json:"lastChangeTime,omitempty"`

	// Number of new package versions discovered during the last scan.
	//
	// Equals `0` if no new versions were discovered.
	// +optional
	LastNewVersions int `json:"lastNewVersions,omitempty"`

	// List of packages available in the repository.
	// +optional
	Packages []PackageRepositoryStatusPackage `json:"packages,omitempty"`

	// Total number of packages in the repository.
	// +optional
	PackagesCount int `json:"packagesCount,omitempty"`

	// Current phase of the repository.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message with additional information about the state of the package repository.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions reflecting the current state of the package repository.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +crd-enricher:raw:items.description="Condition reflecting an aspect of the current state of the package repository."
	// +crd-enricher:raw:items.properties.lastTransitionTime.description="Time when the condition status last changed."
	// +crd-enricher:raw:items.properties.message.description="Message with additional information about the condition.\n\nMay be empty."
	// +crd-enricher:raw:items.properties.observedGeneration.description="Value of `.metadata.generation` for which the condition was set.\n\nIndicates whether the condition corresponds to the current generation of the instance."
	// +crd-enricher:raw:items.properties.reason.description="Programmatic identifier indicating the reason for the last condition status change."
	// +crd-enricher:raw:items.properties.status.description="Condition status."
	// +crd-enricher:raw:items.properties.type.description="Condition type in CamelCase or in the format `foo.example.com/CamelCase`."
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type PackageRepositoryStatusPackage struct {
	// Package name.
	Name string `json:"name"`

	// Package type.
	Type string `json:"type"`
}

// +kubebuilder:object:root=true

// PackageRepositoryList is a list of PackageRepository resources
type PackageRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PackageRepository `json:"items"`
}
