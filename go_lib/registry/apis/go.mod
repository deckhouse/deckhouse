// A module of its own, and not part of go_lib/registry.
//
// These are Kubernetes API types, so they need apimachinery; go_lib/registry is also
// used by bashible-apiserver, which is pinned to an older Kubernetes. Keeping the types
// here means that pin is not dragged forward by the new implementation, and that
// go_lib/registry keeps no Kubernetes dependency at all.
//
// The directory sits under go_lib/registry all the same, because that is where the
// import path says it belongs and where anyone would look for it.
module github.com/deckhouse/deckhouse/go_lib/registry/apis

go 1.24.0

toolchain go1.24.2

require (
	github.com/stretchr/testify v1.11.1
	k8s.io/apimachinery v0.34.8
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)
