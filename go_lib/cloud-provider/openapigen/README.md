# openapigen

Go library for generating OpenAPI YAML schemas and Kubernetes CRDs from Go types, with support for kubebuilder and Deckhouse markers.

## Schema Generation

```go
out, err := openapigen.GenerateDeckhouseOpenAPISchema(MySpec{})
```

## CRD Generation

### Prerequisites

Each CRD root type must:

1. Embed `metav1.TypeMeta` and `metav1.ObjectMeta` (anonymous embedding, not named fields)
2. Have `// +groupName=<group>` on the package
3. Have `// +kubebuilder:object:root=true` on the type
4. Have `// +kubebuilder:resource:scope=Cluster` or `scope=Namespaced`
5. Exactly one version must carry `// +kubebuilder:storageversion`

This matches the `controller-gen` CLI requirements exactly.

### Minimal example

```go
// Package v1alpha1 contains the CRD types.
//
// +groupName=example.io
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// MyResource is a cluster-scoped resource.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
type MyResource struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec MyResourceSpec `json:"spec"`
}

type MyResourceSpec struct {
    // Host is the target hostname.
    //
    // +kubebuilder:validation:MaxLength=253
    // +deckhouse:XDocSearch=true
    Host string `json:"host"`
}
```

### Usage

```go
import (
    "openapigen"
    v1alpha1 "example.com/myproject/api/v1alpha1"
)

// GenerateCRD: full CRD YAML with kubebuilder + deckhouse x-* extensions
out, err := openapigen.GenerateCRD([]openapigen.VersionSpec{
    {Root: &v1alpha1.MyResource{}},
})

// CRDGenerator: fine-grained control
gen, err := openapigen.NewCRDGenerator(openapigen.SchemaConfig{
    EnableKubebuilderMarkers: true,
    EnableDeckhouseMarkers:   true,
})
out, err = gen.GenerateYAML(openapigen.CRDMeta{}, []openapigen.VersionSpec{
    {Root: &v1alpha1.MyResource{}},
})
```

### controller-gen compatibility

`GenerateCRD` uses the same `crd.Parser.NeedCRDFor` pipeline as `controller-gen`. Group, kind, scope, version name, served, and storage flags are all derived from kubebuilder markers on the types — not from any config struct. Deckhouse `x-*` extensions are merged on top of the kubebuilder schema in the final YAML output.
