/*
Copyright 2023 Flant JSC

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
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/unix"
)

func main() {
	gfPathsConfig := os.Getenv("GF_PATHS_CONFIG")
	if err := unix.Access(gfPathsConfig, unix.R_OK); err != nil {
		log.Fatalf("GF_PATHS_CONFIG='%s' is not readable.", gfPathsConfig)
	}

	gfPathsData := os.Getenv("GF_PATHS_DATA")
	if err := unix.Access(gfPathsData, unix.W_OK); err != nil {
		log.Fatalf("GF_PATHS_DATA='%s' is not writable.", gfPathsData)
	}

	gfPathsHome := os.Getenv("GF_PATHS_HOME")
	if err := unix.Access(gfPathsHome, unix.W_OK); err != nil {
		log.Fatalf("GF_PATHS_HOME='%s' is not readable.", gfPathsHome)
	}

	gfPathsLogs := os.Getenv("GF_PATHS_LOGS")
	gfPathsPlugins := os.Getenv("GF_PATHS_PLUGINS")
	gfPathsProvisioning := os.Getenv("GF_PATHS_PROVISIONING")

	grafanaArgs := []string{
		"grafana",
		fmt.Sprintf("--homepath=%s", gfPathsHome),
		fmt.Sprintf("--config=%s", gfPathsConfig),
		"--packaging=docker",
		"cfg:default.log.mode=console",
		fmt.Sprintf("cfg:default.paths.data=%s", gfPathsData),
		fmt.Sprintf("cfg:default.paths.logs=%s", gfPathsLogs),
		fmt.Sprintf("cfg:default.paths.plugins=%s", gfPathsPlugins),
		fmt.Sprintf("cfg:default.paths.provisioning=%s", gfPathsProvisioning),
	}

	err := unix.Exec("/usr/share/grafana/bin/grafana", grafanaArgs, os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}
