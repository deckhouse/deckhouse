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

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestRemovePlkAnnotations(t *testing.T) {
	t.Run("Remove plk_ annotations", func(t *testing.T) {
		annotations := model.LabelSet{
			"test":            "test",
			"plk_annotation":  "1",
			"plk_2annotation": "2",
		}
		removePlkAnnotations(annotations)
		require.Equal(t, len(annotations), 1)
	})
}
