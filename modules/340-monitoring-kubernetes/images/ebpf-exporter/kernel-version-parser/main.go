/*
Copyright 2021 Flant JSC

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
	"regexp"

	goVersion "github.com/hashicorp/go-version"
	"golang.org/x/sys/unix"
)

var kernelVersionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func main() {
	constraintStr := os.Args[1]

	var uname unix.Utsname
	err := unix.Uname(&uname)
	if err != nil {
		log.Fatal(err)
	}

	kernelVersionRaw := kernelVersionRegex.FindString(string(uname.Release[:]))
	if len(kernelVersionRaw) == 0 {
		log.Fatal(fmt.Errorf("failed to parse kernel release: %q", kernelVersionRaw))
	}

	kernelVersion, err := goVersion.NewVersion(kernelVersionRaw)
	if err != nil {
		log.Fatal(err)
	}

	constraint, err := goVersion.NewConstraint(constraintStr)
	if err != nil {
		log.Fatal(err)
	}

	if constraint.Check(kernelVersion) {
		os.Exit(0)
	}

	os.Exit(13)
}
