/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"log"
	"os"

	"webhook/internal/web"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	server, err := web.NewServer(logger)
	if err != nil {
		logger.Fatal(err)
	}

	if err = server.Run(); err != nil {
		logger.Fatal(err)
	}
}
