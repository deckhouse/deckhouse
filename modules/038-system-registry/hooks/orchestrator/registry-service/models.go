/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registryservice

type Inputs bool

type Mode string

const (
	ModeDisabled       Mode = ""
	ModeNodeServices   Mode = "node-services"
	ModeInClusterProxy Mode = "incluster-proxy"
)
