/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checker

type deckhouseImagesModel struct {
	InitContainers map[string]string
	Containers     map[string]string
}

type queueItem struct {
	Repository string
	Image      string
	Info       string
	Error      string
}
