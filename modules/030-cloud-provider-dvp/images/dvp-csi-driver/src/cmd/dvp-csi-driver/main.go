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

package main

import (
	"dvp-csi-driver/internal/config"
	"flag"
	"fmt"
	"os"
	"path"

	"k8s.io/klog/v2"

	dvpcsidriver "dvp-csi-driver/pkg/dvp-csi-driver"
)

// Set by the build process
var version = ""

func main() {
	cfg, err := config.NewCSIConfig(version)
	if err != nil {
		fmt.Printf("Failed to initialize driver: %s\n", err.Error())
		os.Exit(1)
	}

	showVersion := flag.Bool("version", false, "Show version.")

	klog.InitFlags(nil)
	flag.Parse()

	if *showVersion {
		baseName := path.Base(os.Args[0])
		fmt.Println(baseName, version)
		return
	}

	driver, err := dvpcsidriver.NewDriver(cfg)
	if err != nil {
		fmt.Printf("Failed to initialize driver: %s\n", err.Error())
		os.Exit(1)
	}
	if err := driver.Run(); err != nil {
		fmt.Printf("Failed to run driver: %s\n", err.Error())
		os.Exit(1)

	}
}
