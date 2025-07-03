/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package system

import "os/exec"

func WallMessage(msg string) error {
	return exec.Command("wall", msg).Run()
}
