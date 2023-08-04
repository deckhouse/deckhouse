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
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	var enabledModules = strings.Fields(os.Getenv("ENABLED_MODULES"))
	var availableModules []string
	err := filepath.Walk("/available_hooks", func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, "/webhooks") {
			availableModules = append(availableModules, strings.TrimSuffix(path, "/webhooks"))
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, module := range enabledModules {
		moduleDir := findModuleDir(module, availableModules)
		if len(moduleDir) > 0 {
			err := copyModule(moduleDir, "/hooks")
			if err != nil {
				log.Println(err)
			}
		}
	}

	err = executeShellOperator()
	if err != nil {
		log.Fatal(err)
	}
}

func findModuleDir(module string, availableModules []string) string {
	for _, availableModule := range availableModules {
		if strings.HasSuffix(availableModule, fmt.Sprintf("/%s", module)) {
			return availableModule
		}
	}
	return ""
}

func copyModule(srcDir string, destDir string) error {
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			err := os.MkdirAll(destPath, info.Mode())
			if err != nil {
				return err
			}
		} else {
			err := copyFile(path, destPath, info)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func copyFile(srcFile string, destFile string, fileInfo os.FileInfo) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer src.Close()

	dest, err := os.OpenFile(destFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileInfo.Mode())
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, src)
	if err != nil {
		return err
	}

	return nil
}

func executeShellOperator() error {
	cmd := exec.Command("/sbin/tini", "--", "/shell-operator")
	cmd.Args = append(cmd.Args, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}
