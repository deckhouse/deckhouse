/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

type OvirtModuleConfiguration struct {
	Connection *provider `json:"connection,omitempty" yaml:"connection,omitempty"`
}
