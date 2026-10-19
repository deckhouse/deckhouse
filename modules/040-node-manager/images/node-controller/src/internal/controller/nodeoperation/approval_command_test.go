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

package nodeoperation

import (
	"testing"

	"sigs.k8s.io/yaml"
)

// The message is read in kubectl -o yaml: it must come out of the YAML emitter verbatim,
// on one line, so that whatever the operator copies from it is what was written.
func TestWaitingForApprovalMessageSurvivesKubectlYAML(t *testing.T) {
	out, err := yaml.Marshal(map[string]string{"message": waitingForApprovalMessage})
	if err != nil {
		t.Fatal(err)
	}
	if want := "message: " + waitingForApprovalMessage + "\n"; string(out) != want {
		t.Fatalf("kubectl -o yaml would print\n%s\nwant\n%s", out, want)
	}
}
