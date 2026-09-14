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

package helm

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func parseManifest(raw string) (*unstructured.Unstructured, bool, error) {
	object := new(unstructured.Unstructured)
	if err := yaml.Unmarshal([]byte(raw), object); err != nil {
		return nil, false, err
	}
	if object.GetAPIVersion() == "" || object.GetKind() == "" {
		return nil, false, nil
	}

	// Normalize fields that can differ between serializers/versions.
	unstructured.RemoveNestedField(object.Object, "metadata", "creationTimestamp")

	return object, true, nil
}

func read[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return new(T), nil
		}
		return nil, err
	}
	object := new(T)
	if err = yaml.Unmarshal(data, object); err != nil {
		return nil, err
	}
	return object, nil
}

func TestReleaseName(t *testing.T) {
	t.Parallel()

	// Names within Helm's limit are used verbatim (no migration for existing releases).
	passthrough := []struct {
		name    string
		project string
	}{
		{name: "short name", project: "t-proj"},
		{name: "exactly the limit", project: strings.Repeat("a", helmReleaseNameMaxLen)},
	}
	for _, tt := range passthrough {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.project, ReleaseName(tt.project))
		})
	}

	// A name above the limit is shortened to a valid, deterministic, <=53-char release name.
	long := "t-" + strings.Repeat("z", 59) // 61 chars, above the 53 limit
	got := ReleaseName(long)
	assert.LessOrEqual(t, len(got), helmReleaseNameMaxLen, "release name must fit Helm's limit")
	assert.Equal(t, got, ReleaseName(long), "release name must be deterministic")
	assert.Regexp(t, `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, got, "must be a valid Helm release name")

	// Distinct long names map to distinct release names (the suffix hashes the full name).
	other := "t-" + strings.Repeat("y", 59)
	assert.NotEqual(t, ReleaseName(long), ReleaseName(other))
}
