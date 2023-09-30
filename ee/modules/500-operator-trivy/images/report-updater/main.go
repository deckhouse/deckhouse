/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"log"
	"os"

	"report-updater/web"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	newServer, err := web.NewServer(logger)
	if err != nil {
		logger.Fatal(err)
	}
	if err := newServer.Run(); err != nil {
		logger.Fatal(err)
	}
}
