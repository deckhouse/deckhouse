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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConverting(t *testing.T) {
	t.Run("Convert equality-based label to set-based, simple case", func(t *testing.T) {
		out, err := toSet([]byte(`node-role.kubernetes.io/node2=`))
		require.NoError(t, err)
		require.Equal(t, `{"matchLabels":{"node-role.kubernetes.io/node2":""}}`, string(out))
	})
	t.Run("Convert equality-based label to set-based, complex case", func(t *testing.T) {
		out, err := toSet([]byte(`node-role.kubernetes.io/node2 in (test1, test2)`))
		require.NoError(t, err)
		require.Equal(t, `{"matchExpressions":[{"key":"node-role.kubernetes.io/node2","operator":"In","values":["test1","test2"]}]}`, string(out))
	})

	t.Run("Convert set-based label to equality-based, simple case", func(t *testing.T) {
		out, err := toEquality([]byte(`{"matchLabels":{"node-role.kubernetes.io/node2":""}}`))
		require.NoError(t, err)
		require.Equal(t, `node-role.kubernetes.io/node2=`, string(out))
	})
	t.Run("Convert set-based label to equality-based, complex case", func(t *testing.T) {
		out, err := toSet([]byte(`{"matchExpressions":[{"key":"node-role.kubernetes.io/node2","operator":"In","values":["test1","test2"]}]}`))
		require.NoError(t, err)
		require.Equal(t, `node-role.kubernetes.io/node2 in (test1, test2)`, string(out))
	})

}
