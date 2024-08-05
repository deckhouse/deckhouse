/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"dynamix-csi-driver/internal/config"
	"k8s.io/klog/v2"

	dynamixcsidriver "dynamix-csi-driver/pkg/dynamix-csi-driver"
	"flag"
	"fmt"
	"os"
	"path"
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

	driver, err := dynamixcsidriver.NewDriver(cfg)
	if err != nil {
		fmt.Printf("Failed to initialize driver: %s\n", err.Error())
		os.Exit(1)
	}
	if err := driver.Run(); err != nil {
		fmt.Printf("Failed to run driver: %s\n", err.Error())
		os.Exit(1)

	}
}
