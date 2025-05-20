/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
