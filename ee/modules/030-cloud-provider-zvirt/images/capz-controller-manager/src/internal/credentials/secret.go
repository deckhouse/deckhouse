/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package credentials

import (
	"os"
	"strings"
)

type ZvirtCredentials struct {
	URL      string
	User     string
	Password string
	Insecure bool
}

func LoadZvirtCredentialsFromEnv() *ZvirtCredentials {
	return &ZvirtCredentials{
		URL:      os.Getenv("ZVIRT_API_URL"),
		User:     os.Getenv("ZVIRT_USER"),
		Password: os.Getenv("ZVIRT_PASSWORD"),
		Insecure: strings.ToLower(os.Getenv("ZVIRT_CONNECT_INSECURE")) == "true",
	}
}
