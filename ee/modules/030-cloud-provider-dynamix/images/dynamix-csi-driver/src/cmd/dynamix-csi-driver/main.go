/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"dynamix-csi-driver/pkg/dynamix"
	"flag"
	"fmt"
	"os"
	"path"
)

// Set by the build process
var version = ""

func main() {
	cfg := dynamix.Config{
		VendorVersion: version,
	}
	flag.StringVar(&cfg.Endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	flag.StringVar(&cfg.DriverName, "drivername", "dynamix.deckhouse.io", "name of the driver")
	flag.StringVar(&cfg.NodeID, "nodeid", "", "node id")
	showVersion := flag.Bool("version", false, "Show version.")

	if *showVersion {
		baseName := path.Base(os.Args[0])
		fmt.Println(baseName, version)
		return
	}

	driver, err := dynamix.NewCSIDriver(cfg)
	if err != nil {
		fmt.Printf("Failed to initialize driver: %s", err.Error())
		os.Exit(1)
	}
	if err := driver.Run(); err != nil {
		fmt.Printf("Failed to run driver: %s", err.Error())
		os.Exit(1)

	}
}
