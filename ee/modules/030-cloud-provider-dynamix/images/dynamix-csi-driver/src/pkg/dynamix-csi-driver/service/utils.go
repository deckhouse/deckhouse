/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package service

func convertBytesToGigabytes(b uint64) uint64 {
	return b / 1024 / 1024 / 1024
}

func convertGigabytesToBytes(g uint64) uint64 {
	return g * 1024 * 1024 * 1024
}
