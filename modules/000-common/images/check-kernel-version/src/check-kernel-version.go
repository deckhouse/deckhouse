/*
Copyright 2022 Flant JSC

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

	"github.com/Masterminds/semver/v3"
	"golang.org/x/sys/unix"
)

func main() {
	kernelConstraint := os.Getenv("KERNEL_CONSTRAINT")
	if kernelConstraint == "" {
		log.Fatal("ENV variable KERNEL_CONSTRAINT must be set")
	}
	c, err := semver.NewConstraint(kernelConstraint)
	if err != nil {
		log.Fatal(err)
	}

	utsname := unix.Utsname{}
	unix.Uname(&utsname)
	kernelVersion := string(utsname.Release[:])
	/* Kernel version should be splitted to parts because versions `5.15.0-52-generic`
	parses by semver as prerelease version. Prerelease versions by default come before stable versions
	in the order of precedence, so in semver terms `5.15.0-52-generic` less than `5.15`.
	More info - https://github.com/Masterminds/semver#working-with-prerelease-versions */
	v, err := semver.NewVersion(strings.Split(kernelVersion, "-")[0])
	if err != nil {
		log.Fatal(err)
	}

	if !c.Check(v) {
		log.Fatalf("the kernel %s does not meet the requirements: %s", kernelVersion, kernelConstraint)
	}
	log.Printf("the kernel %s meets the requirements: %s", kernelVersion, kernelConstraint)
}
