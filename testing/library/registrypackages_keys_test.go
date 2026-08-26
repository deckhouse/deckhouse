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

package library

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A registrypackage image name doubles as the digest map key that bashible uses to look
// up the package digest. A missed key does not break the build: `index` yields an empty
// string, producing `bb-package-install "kubelet:"`, which only fails at node bootstrap.
//
// This test catches drift between werf image names (which become the keys in
// images_tags_generated.go) and the keys the bashible steps build.
func TestBashibleRegistrypackageKeysExist(t *testing.T) {
	digests, ok := DefaultImagesDigests["registrypackages"].(map[string]interface{})
	if !ok {
		t.Fatal("DefaultImagesDigests has no registrypackages section")
	}

	// Derive the built k8s minors from the crictl keys themselves: crictl has always been
	// named by minor, which makes it the reference for what a minor-only key looks like.
	var minors []string
	for key := range digests {
		if m := regexp.MustCompile(`^crictl(\d{3})$`).FindStringSubmatch(key); m != nil {
			minors = append(minors, m[1])
		}
	}
	if len(minors) == 0 {
		t.Fatal("no crictl<minor> key found: the minor reference is broken")
	}

	required := []string{"containerd1", "containerd2"}
	for _, minor := range minors {
		required = append(required,
			fmt.Sprintf("kubelet%s", minor),
			fmt.Sprintf("kubectl%s", minor),
			fmt.Sprintf("crictl%s", minor),
		)
	}

	for _, key := range required {
		if _, found := digests[key]; !found {
			t.Errorf("registrypackages.%s is missing from the digest map: bashible would get an empty digest", key)
		}
	}
}

// Checks that no bashible step still pins a key with a patch version in the image name
// (kubelet13410, containerd227 and so on): such names change on every patch bump.
func TestBashibleStepsDoNotPinPatchVersions(t *testing.T) {
	root := filepath.Join("..", "..", "candi", "bashible")
	// kubelet13410 / kubectl1357 / containerd227 / containerd1734
	bad := regexp.MustCompile(`\b(kubelet|kubectl|kubeletSysext)\d{4,5}\b|\bcontainerd(Sysext)?\d{2,}\b`)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".tpl") {
			return err
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(body), "\n") {
			if m := bad.FindString(line); m != "" {
				t.Errorf("%s:%d: key %q pins a patch version; use the minor/major-only image name",
					path, i+1, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
}
