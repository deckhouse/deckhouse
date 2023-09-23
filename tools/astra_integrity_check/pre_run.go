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
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func ensureAstraIntCheckInstalled() {
	_, err := os.Lstat("/usr/bin/astra-int-check")
	switch {
	case errors.Is(err, fs.ErrNotExist):
		log.Println("astra-int-check was not found on the system, trying to install")
		if err := installAstraIntChecker(); err != nil {
			log.Fatalln("astra-int-check installation failed:", err)
		}
	case err != nil:
		log.Fatalln(fmt.Errorf("os.Lstat: %w", err))
	}
}

func installAstraIntChecker() error {
	aptUpdateCmd := exec.Command("apt-get", "update")
	aptUpdateCmd.Env = append(aptUpdateCmd.Env, "DEBIAN_FRONTEND=noninteractive")

	aptCmd := exec.Command("apt-get", "install", "-y", "astra-int-check")
	aptCmd.Env = append(aptCmd.Env, "DEBIAN_FRONTEND=noninteractive")

	if err := aptUpdateCmd.Run(); err != nil {
		return fmt.Errorf("apt-get update: %w", err)
	}
	if err := aptCmd.Run(); err != nil {
		return fmt.Errorf("apt-get install: %w", err)
	}
	return nil
}

func ensureGostsums() {
	gostsumsFilePath := filepath.Clean(*gostsumsPath)
	stat, err := os.Lstat(gostsumsFilePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		log.Fatalf("%v: %s", err, gostsumsFilePath)
	case err != nil:
		log.Fatalln(fmt.Errorf("os.Lstat: %w", err))
	case stat.IsDir():
		log.Fatalln("gostsums path points to a directory")
	case !stat.Mode().IsRegular():
		log.Fatalln("gostsums path should point to a regular file")
	}

	gostsumsPath = &gostsumsFilePath
}
