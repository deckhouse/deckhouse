/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"log"
	"os"

	"user-authz-webhook/web"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	if err := web.NewServer(logger).Run(); err != nil {
		logger.Fatal(err)
	}
}
