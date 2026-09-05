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

// A registrypackage image name doubles as the digest map key its consumers use
// to look the package digest up. A missed key does not break the build: `index`
// yields an empty string, producing `bb-package-install "kubelet:"`, which only
// fails at node bootstrap.
//
// This test catches drift between werf image names (which become the keys in
// images_tags_generated.go) and the keys the consumers build. The list is
// deliberately explicit rather than parsed out of the consumers: it is the
// contract, and a consumer that stops asking for a key is a change to review,
// not a reason for the test to follow along.
//
// Consumers, and where each one is guarded:
//   - bashible steps, by this test: 001_prefetch_registry_packages,
//     031_install_containerd, 035_install_kubelet,
//     062_install_kubelet_and_his_friends;
//   - the sysext keys, by this test for existence and by the Go tests of the two
//     readers for the lookup itself — dhctl/pkg/immutable (TestSysextDigests)
//     and node-controller's nodeconfig (TestPickKubeletDigest, TestSoleDigest).
func TestRegistrypackageDigestKeysExist(t *testing.T) {
	digests, ok := DefaultImagesDigests["registrypackages"].(map[string]interface{})
	if !ok {
		t.Fatal("DefaultImagesDigests has no registrypackages section")
	}

	// Derive the built k8s minors from the crictl keys themselves: crictl has always been
	// named by minor, which makes it the reference for what a minor-only key looks like.
	// That is also why crictl itself is absent from the list below — asserting the keys
	// the minors were read from would assert nothing.
	crictlKey := regexp.MustCompile(`^crictl(\d{3})$`)
	var minors []string
	for key := range digests {
		if m := crictlKey.FindStringSubmatch(key); m != nil {
			minors = append(minors, m[1])
		}
	}
	if len(minors) == 0 {
		t.Fatal("no crictl<minor> key found: the minor reference is broken")
	}

	// containerd is named by major, and its sysext is built for v2 only.
	// DefaultImagesDigests is generated with WERF_ENV=FE, where both majors are
	// built; a CSE render carries containerd2 alone.
	required := []string{"containerd1", "containerd2", "containerdSysext2"}
	for _, minor := range minors {
		required = append(required,
			fmt.Sprintf("kubelet%s", minor),
			fmt.Sprintf("kubectl%s", minor),
			fmt.Sprintf("kubeletSysext%s", minor),
		)
	}

	for _, key := range required {
		if _, found := digests[key]; !found {
			t.Errorf("registrypackages.%s is missing from the digest map: its consumer would get an empty digest", key)
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
