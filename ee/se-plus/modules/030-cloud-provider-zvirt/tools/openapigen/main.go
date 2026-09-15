/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"flag"
	"fmt"
	"os"

	"zvirt-tools/openapigen/bundle"
)

var (
	moduleRoot = flag.String("module-root", "../..", "path to the cloud-provider-zvirt module root")
)

func main() {
	flag.Parse()

	if err := bundle.GenerateBundle(*moduleRoot); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "generate bundle: %v\n", err)
		os.Exit(1)
	}
}
