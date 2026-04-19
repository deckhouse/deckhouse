/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"webhook/internal/web"
)

func main() {
	client, err := web.NewClient()
	check(err)

	addr := url.URL{
		Scheme: "https",
		Host:   web.ListenAddr,
		Path:   "healthz",
	}
	response, err := client.Get(addr.String())
	check(err)

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(log.Writer(), response.Body)
		response.Body.Close()
		os.Exit(1)
	}

	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()
}

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
