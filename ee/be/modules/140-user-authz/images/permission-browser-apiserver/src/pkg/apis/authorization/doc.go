/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// +k8s:deepcopy-gen=package
// +groupName=authorization.deckhouse.io

// Package authorization is the internal (hub) version of the API.
// This follows the standard Kubernetes API versioning pattern where:
// - The internal version (this package) is used for in-memory operations
// - External versions (v1alpha1, etc.) are used for serialization/API wire format
// - Conversion between versions happens through this internal hub
// See: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#api-versioning
package authorization // import "permission-browser-apiserver/pkg/apis/authorization"
