/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"os"

	"d8_shutdown_inhibitor/pkg/tools"
)

func main() {
	tools.Run(os.Args)
}
