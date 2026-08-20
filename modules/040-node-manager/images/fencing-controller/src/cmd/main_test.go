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

package main

import (
	"testing"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

func TestNewSchemeRegistersFencingKinds(t *testing.T) {
	s, err := newScheme()
	if err != nil {
		t.Fatalf("new scheme: %v", err)
	}

	for _, kind := range []string{
		"FencingFailedNodeState",
		"FencingFailedNodeStateList",
		"FencingSLAProfile",
		"FencingSLAProfileList",
	} {
		gvk := v1alpha1.GroupVersion.WithKind(kind)
		if !s.Recognizes(gvk) {
			t.Errorf("scheme does not recognize %s", gvk)
		}
	}
}
