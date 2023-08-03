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
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading standard input: %v", err)
	}

	tmpfile, err := os.CreateTemp("", "trickster.conf-*")
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

	cmd := exec.Command("trickster", builder.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	log.Println(cmd.String())
	log.Println(os.ExpandEnv(string(content)))

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
