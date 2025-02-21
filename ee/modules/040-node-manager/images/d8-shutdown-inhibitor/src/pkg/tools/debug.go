/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"fmt"

	"d8_shutdown_inhibitor/pkg/app"
	"d8_shutdown_inhibitor/pkg/inputdev"
)

func Run(args []string) bool {
	if len(args) < 2 {
		return false
	}

	switch args[1] {
	case "node-name":
		NodeName()
	case "node-cordon":
		NodeCordon()
	case "node-condition":
		if len(args) < 3 {
			fmt.Println("Choose stage: start, pods, or unlock")
			return false
		}
		NodeCondition(args[2])
	case "list-pods":
		ListPods(app.InhibitNodeShutdownLabel)
	case "list-input-devices":
		ListInputDevices()
	case "watch-for-key":
		fmt.Println("Use real tty (vm console) and press buttons Q W E or Enter")
		WatchForKey(inputdev.KEY_Q, inputdev.KEY_E, inputdev.KEY_W, inputdev.KEY_ENTER)
	default:
		return false
	}

	return true
}
