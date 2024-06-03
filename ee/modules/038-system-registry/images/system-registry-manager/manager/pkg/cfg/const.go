/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cfg

var (
	SeaweedfsMasterPort              = 9333
	SeaweedfsFilerPort               = 8888
	SeaweedfsStaticPodLabelsSelector = []string{"component=system-registry", "tier=control-plane"}
)
