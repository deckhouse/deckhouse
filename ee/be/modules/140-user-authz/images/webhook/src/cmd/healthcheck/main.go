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
	"strings"
	"time"

	"webhook/internal/web"
)

func main() {
	client, err := web.NewClient()
	check(err)

	// The probe decides which question to ask: healthz for liveness, readyz for readiness.
	path := "healthz"
	if len(os.Args) > 1 && os.Args[1] != "" {
		path = strings.TrimPrefix(os.Args[1], "/")
	}

	// Without this the request has no deadline of its own and the probe can only end by the
	// kubelet killing it, which reports nothing useful.
	client.Timeout = 4 * time.Second

	addr := url.URL{
		Scheme: "https",
		Host:   web.ListenAddr,
		Path:   path,
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
