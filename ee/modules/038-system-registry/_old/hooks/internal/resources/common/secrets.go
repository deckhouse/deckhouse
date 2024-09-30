/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resources

type SecretData map[string][]byte

type SecretDataKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
