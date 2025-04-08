/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

type registryConfig struct {
	Mode       string
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string
}
