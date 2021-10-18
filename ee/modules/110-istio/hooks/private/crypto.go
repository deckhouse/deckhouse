/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package private

type IstioCA struct {
	Key   string `json:"key"`
	Cert  string `json:"cert"`
	Chain string `json:"chain"`
	Root  string `json:"root"`
}

type Keypair struct {
	Pub  string `json:"pub"`
	Priv string `json:"priv"`
}
