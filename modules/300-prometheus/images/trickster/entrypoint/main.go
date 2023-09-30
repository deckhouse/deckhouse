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
	"log"
	"os"
	"strings"
	"syscall"
)

func main() {
	content, err := os.ReadFile("/etc/trickster/trickster.conf")
	if err != nil {
		log.Fatalf("reading conf: %v", err)
	}

	tmpfile, err := os.CreateTemp("", "*-trickster.conf")
	if err != nil {
		log.Fatalf("create tmp conf: %v", err)
	}

	if _, err := tmpfile.Write([]byte(os.ExpandEnv(string(content)))); err != nil {
		log.Fatalf("write tmp conf: %v", err)
	}
	tmpfile.Close()

	builder := strings.Builder{}
	builder.WriteString("--config=")
	builder.WriteString(tmpfile.Name())

	err = syscall.Exec("/usr/local/bin/trickster", []string{"trickster", builder.String()}, os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}
