/*
Copyright 2026 Flant JSC

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
package template_tests

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// proxyFlagsFile lists the flags of the upstream oauth2-proxy release the image is built from.
	proxyFlagsFile = "testdata/oauth2-proxy-flags.txt"
	// proxyPatches are the patches of the image, proxyVersions the file with the version of the release.
	proxyPatches  = "../images/dex-authenticator/patches/*.patch"
	proxyVersions = "../oss.yaml"
)

var (
	// A patch defines a flag on the flag set of the options or of main.
	patchFlag    = regexp.MustCompile(`^\+.*flagSet\.[A-Za-z0-9]+\("([a-z0-9-]+)"`)
	proxyVersion = regexp.MustCompile(`(?m)^\s*id: oauth2-proxy\n\s*version: (\S+)$`)
)

// proxyFlags returns the flags the dex-authenticator proxy of the image knows: those of the
// upstream release and those the patches of the image add. A flag of a template that is not among
// them makes the container exit at start, which no other test of the repository sees: nothing
// here builds the proxy.
func proxyFlags() (map[string]struct{}, error) {
	versions, err := os.ReadFile(proxyVersions)
	if err != nil {
		return nil, err
	}
	version := proxyVersion.FindSubmatch(versions)
	if version == nil {
		return nil, fmt.Errorf("%s has no version of oauth2-proxy", proxyVersions)
	}

	list, err := os.ReadFile(proxyFlagsFile)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(list), "\n")
	if want := "# oauth2-proxy v" + string(version[1]); lines[0] != want {
		return nil, fmt.Errorf("%s is the list of %q, and the image is built from %q: write the list anew", proxyFlagsFile, lines[0], want)
	}
	flags := map[string]struct{}{}
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			flags[line] = struct{}{}
		}
	}

	patches, err := filepath.Glob(proxyPatches)
	if err != nil {
		return nil, err
	}
	if len(patches) == 0 {
		return nil, fmt.Errorf("no patches match %s", proxyPatches)
	}
	for _, patch := range patches {
		file, err := os.Open(patch)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
		for scanner.Scan() {
			if match := patchFlag.FindStringSubmatch(scanner.Text()); match != nil {
				flags[match[1]] = struct{}{}
			}
		}
		err = scanner.Err()
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", patch, err)
		}
	}

	return flags, nil
}

// flagName returns the name of the flag an argument of the proxy sets, and false for an argument
// that is not of the form --name or --name=value.
func flagName(arg string) (string, bool) {
	name, found := strings.CutPrefix(arg, "--")
	if !found {
		return "", false
	}
	name, _, _ = strings.Cut(name, "=")

	return name, name != ""
}
