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

package immutabletest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"
)

// NoRetryCollapse restores the real retry loop: the test environment collapses it
// to a single attempt, and a single attempt cannot show a wait working.
func NoRetryCollapse(t *testing.T) {
	t.Helper()

	inTestEnvironment := libretry.InTestEnvironment
	libretry.InTestEnvironment = false
	t.Cleanup(func() { libretry.InTestEnvironment = inTestEnvironment })
}

// PayloadDocuments returns the documents a cloud payload carries, in order: the
// node unwraps the write_files of the #cloud-config and files their contents by
// kind (documentParts, images/init/src/0.1/acquire.go of the initramfs).
func PayloadDocuments(t *testing.T, payload []byte) []string {
	t.Helper()

	var envelope struct {
		WriteFiles []struct {
			Content string `json:"content"`
		} `json:"write_files"`
	}
	require.NoError(t, yaml.Unmarshal(payload, &envelope))

	documents := make([]string, 0, len(envelope.WriteFiles))
	for _, file := range envelope.WriteFiles {
		documents = append(documents, file.Content)
	}
	return documents
}
