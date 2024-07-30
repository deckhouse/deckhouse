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
	"flag"
	"fmt"
	"os"
	"syscall"
)

var (
	dirMode     string
	path        string
	sizeInBytes string
	action      string
)

const (
	SETUP    = "create"
	TEARDOWN = "delete"
)

func init() {
	flag.StringVar(&path, "p", "", "Absolute path")
	flag.StringVar(&sizeInBytes, "s", "", "Size in bytes")
	flag.StringVar(&dirMode, "m", "", "Dir mode")
	flag.StringVar(&action, "a", "", fmt.Sprintf("Action name. Can be '%s' or '%s'", SETUP, TEARDOWN))
}

func main() {
	flag.Parse()
	if action != SETUP && action != TEARDOWN {
		fmt.Fprintf(os.Stderr, "Error. Incorrect action: %s\n", action)
		os.Exit(1)
	}

	if path == "" {
		fmt.Fprintf(os.Stderr, "Error. Path is empty\n")
		os.Exit(2)
	}

	if path == "/" {
		fmt.Fprintf(os.Stderr, "Error. Path cannot be '/'\n")
		os.Exit(3)
	}

	fmt.Printf("Do action '%s' for path %s \n", action, path)

	if action == TEARDOWN {
		err := os.RemoveAll(path)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error. Cannot remove directory %s: %s\n", path, err)
			os.Exit(4)
		}
		return
	}

	syscall.Umask(0)

	err := os.MkdirAll(path, 0777)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error. Cannot create directory %s: %s\n", path, err)
		os.Exit(5)
	}
}
