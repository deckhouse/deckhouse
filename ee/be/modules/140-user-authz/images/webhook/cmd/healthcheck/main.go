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

	"user-authz-webhook/web"
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

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		io.Copy(log.Writer(), response.Body)
		log.Fatalln()
	}

	io.Copy(io.Discard, response.Body)
}

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
