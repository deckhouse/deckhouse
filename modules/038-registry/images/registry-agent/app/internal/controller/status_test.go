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

package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// makeStatusObj builds an *unstructured.Unstructured whose status block matches
// the provided values, mirroring what setStatus writes.
func makeStatusObj(ready bool, message string, observedGeneration int64) *unstructured.Unstructured {
	status := map[string]interface{}{
		"ready":              ready,
		"observedGeneration": observedGeneration,
	}
	if message != "" {
		status["message"] = message
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": status,
		},
	}
}

func TestStatusUnchanged_Match(t *testing.T) {
	obj := makeStatusObj(true, "", 3)
	if !statusUnchanged(obj, true, "", 3) {
		t.Error("expected statusUnchanged=true when all fields match")
	}
}

func TestStatusUnchanged_MatchWithMessage(t *testing.T) {
	obj := makeStatusObj(false, "parse error", 2)
	if !statusUnchanged(obj, false, "parse error", 2) {
		t.Error("expected statusUnchanged=true when all fields (incl. message) match")
	}
}

func TestStatusUnchanged_ReadyDiffers(t *testing.T) {
	obj := makeStatusObj(false, "", 1)
	if statusUnchanged(obj, true, "", 1) {
		t.Error("expected statusUnchanged=false when ready differs")
	}
}

func TestStatusUnchanged_GenerationDiffers(t *testing.T) {
	obj := makeStatusObj(true, "", 1)
	if statusUnchanged(obj, true, "", 2) {
		t.Error("expected statusUnchanged=false when observedGeneration differs")
	}
}

func TestStatusUnchanged_MessageDiffers(t *testing.T) {
	obj := makeStatusObj(false, "old error", 0)
	if statusUnchanged(obj, false, "new error", 0) {
		t.Error("expected statusUnchanged=false when message differs")
	}
}

func TestStatusUnchanged_MessageAdded(t *testing.T) {
	// obj has no message; desired has one.
	obj := makeStatusObj(false, "", 0)
	if statusUnchanged(obj, false, "new error", 0) {
		t.Error("expected statusUnchanged=false when message added")
	}
}

func TestStatusUnchanged_MessageRemoved(t *testing.T) {
	// obj has a message; desired clears it.
	obj := makeStatusObj(false, "old error", 0)
	if statusUnchanged(obj, false, "", 0) {
		t.Error("expected statusUnchanged=false when message removed")
	}
}

func TestStatusUnchanged_EmptyObject(t *testing.T) {
	// An object with no status block: all fields are zero-value.
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	if !statusUnchanged(obj, false, "", 0) {
		t.Error("expected statusUnchanged=true for empty object vs zero desired values")
	}
}
