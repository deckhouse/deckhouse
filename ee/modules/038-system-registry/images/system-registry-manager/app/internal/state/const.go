/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import "errors"

const (
	RegistryNamespace = "d8-system"

	RegistryModuleName = "system-registry"

	RegistrySvcName = "embedded-registry"

	LabelTypeKey         = "type"
	LabelModuleKey       = "module"
	LabelHeritageKey     = "heritage"
	LabelHeritageValue   = "deckhouse"
	LabelNodeIsMasterKey = "node-role.kubernetes.io/master"
	LabelManagedBy       = "app.kubernetes.io/managed-by"
)

var (
	ErrConfigMapIsNil = errors.New("configmap is nil")
	ErrSecretIsNil    = errors.New("secret is nil")
	ErrInvalid        = errors.New("data is invalid")
	ErrIsNil          = errors.New("data is nil")
)
