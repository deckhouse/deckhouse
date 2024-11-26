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
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type stapelManifest struct {
	Artifact     string `yaml:"artifact"`
	FromArtifact string `yaml:"fromArtifact"`
	Shell        struct {
		BeforeInstall []string `yaml:"beforeInstall"`
		Install       []string `yaml:"install"`
		BeforeSetup   []string `yaml:"beforeSetup"`
		Setup         []string `yaml:"setup"`
	} `yaml:"shell"`
}

var (
	cloneRegexp = regexp.MustCompile(`git(\s+-{1,2}[\w-]+)*\s+clone`)
)

func main() {
	log.SetFlags(0)
	yamlDecoder := yaml.NewDecoder(os.Stdin)
	stapel := stapelManifest{}

	exitCode := 0
	for {
		stapel = stapelManifest{}
		if err := yamlDecoder.Decode(&stapel); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalln("invalid yaml:", err)
		}
		if stapel.FromArtifact != "common/src-artifact" {
			continue
		}

		validators := []func(manifest *stapelManifest) error{
			validateSrcArtifactSuffix, validateGitCloneNotUsed,
		}

		for _, validator := range validators {
			if err := validator(&stapel); err != nil {
				exitCode = 1
				log.Println(err)
			}
		}
	}
	os.Exit(exitCode)
}

func validateSrcArtifactSuffix(stapel *stapelManifest) error {
	if !strings.HasSuffix(stapel.Artifact, "-src-artifact") {
		return fmt.Errorf(
			"[SRC-M1] %s: Artifact is based on common/src-artifact but does not have a -src-artifact suffix",
			stapel.Artifact)
	}
	return nil
}

func validateGitCloneNotUsed(stapel *stapelManifest) error {
	commands := append(stapel.Shell.BeforeInstall,
		append(stapel.Shell.Install,
			append(stapel.Shell.BeforeSetup, stapel.Shell.Setup...)...)...,
	)

	for _, cmd := range commands {
		if strings.Contains(cmd, "git clone") || cloneRegexp.MatchString(cmd) {
			return fmt.Errorf(
				"[SRC-M2] %s: instead of 'git' werf directive, 'git clone' is used in the 'shell' directive as %q",
				stapel.Artifact, cmd)
		}
	}

	return nil
}
