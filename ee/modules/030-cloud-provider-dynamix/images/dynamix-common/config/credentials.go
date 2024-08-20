/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

type Credentials struct {
	AppID         string
	AppSecret     string
	OAuth2URL     string
	ControllerURL string
	Insecure      bool
}
