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
	"os"
	"time"

	"d8_shutdown_inhibitor/pkg/app"
)

func ListPods(podLabel string) {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("START Error: get hostname: %v\n", err)
		os.Exit(1)
	}

	// Create application.
	app := app.NewApp(app.AppConfig{
		InhibitDelayMax:       30 * time.Minute,
		WallBroadcastInterval: 30 * time.Second,
		PodLabel:              podLabel,
		NodeName:              nodeName,
	})

	app.ListPods()
}
