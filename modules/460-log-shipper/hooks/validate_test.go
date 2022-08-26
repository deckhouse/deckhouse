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

package hooks

import (
	"os"
	"os/exec"
	"testing"
)

func TestValidateConfigWithVector(t *testing.T) {

	dockerImage := "timberio/vector:0.23.3-debian"

	script := `
	set -e

	path="/deckhouse/modules/460-log-shipper/hooks/testdata"

	for file in $(find ${path}/*); do
	  vector validate --config-json $file --config-json "${path}/default-config.json";
	done`

	cmd := exec.Command(
		"docker",
		"run",
		"-t",
		"-v", "/deckhouse:/deckhouse",
		"-e", "VECTOR_SELF_POD_NAME=test", // to avoid warnings, this variable is set in the container env section
		"-e", "VECTOR_SELF_NODE_NAME=test",
		"--entrypoint", "bash",
		// Kubernetes in-cluster config values required for validation
		"-v", "/dev/null:/var/run/secrets/kubernetes.io/serviceaccount/token",
		"-v", "/dev/null:/var/run/secrets/kubernetes.io/serviceaccount/namespace",
		"-v", "/dev/null:/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		dockerImage,
		"-c", script,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf(err.Error())
	}
}
