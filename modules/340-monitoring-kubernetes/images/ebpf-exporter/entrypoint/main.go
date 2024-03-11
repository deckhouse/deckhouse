/*
Copyright 2024 Flant JSC

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
	"strings"
	"syscall"
)

func main() {
	binPath := os.Getenv("EBPF_EXPORTER_BIN_PATH")
	if binPath == "" {
		binPath = "/usr/local/bin/ebpf_exporter"
	}

	bits := strings.Split(binPath, "/")
	if len(bits) == 0 {
		log.Fatalf("Incorrect bin path: %s", binPath)
	}
	binName := bits[len(bits)-1]

	configDir := os.Getenv("EBPF_EXPORTER_CONFIG_DIR")
	if configDir == "" {
		configDir = "/metrics"
	}

	configNames := os.Getenv("EBPF_EXPORTER_CONFIG_NAMES")
	if configNames == "" {
		configNames = "oomkill"
	}

	listenAddress := os.Getenv("EBPF_EXPORTER_LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = "127.0.0.1:9435"
	}

	args := []string{
		binName,
		fmt.Sprintf("--config.dir=%s", configDir),
		fmt.Sprintf("--config.names=%s", configNames),
		fmt.Sprintf("--web.listen-address=%s", listenAddress),
	}

	err := syscall.Exec(binPath, args, os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}
