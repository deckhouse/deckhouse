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

func TestInsertAlertToStore(t *testing.T) {
	store := newMemStore(10)
	t.Run("Add first alert", func(t *testing.T) {
		alert := &model.Alert{
			Labels: model.LabelSet{
				"severity_level": "5",
				"instance":       "192.168.199.91:9650",
				"job":            "custom-my-app",
				"namespace":      "d8-system",
				"pod":            "deckhouse-aaa",
				"prometheus":     "deckhouse",
				"service":        "deckhouse",
			},
		}
		err := store.insertAlert(alert)
		require.NoError(t, err)
		require.Equal(t, len(store.alerts), 1)
	})

	t.Run("Add second alert", func(t *testing.T) {
		alert := &model.Alert{
			Labels: model.LabelSet{
				"severity_level": "5",
				"instance":       "192.168.199.91:9650",
				"job":            "custom-my-app",
				"namespace":      "d8-system",
				"pod":            "deckhouse-bbb",
				"prometheus":     "deckhouse",
				"service":        "deckhouse",
			},
		}
		err := store.insertAlert(alert)
		require.NoError(t, err)
		require.Equal(t, len(store.alerts), 2)
	})

	t.Run("Add alert like first, but with bigger severity", func(t *testing.T) {
		alert := &model.Alert{
			Labels: model.LabelSet{
				"severity_level": "9",
				"instance":       "192.168.199.91:9650",
				"job":            "custom-my-app",
				"namespace":      "d8-system",
				"pod":            "deckhouse-aaa",
				"prometheus":     "deckhouse",
				"service":        "deckhouse",
			},
		}
		err := store.insertAlert(alert)
		require.NoError(t, err)
		require.Equal(t, len(store.alerts), 2)
		require.Equal(t, store.alerts[fingerprintWithoutSeverity(alert)].Alert.Labels[severityLabel], model.LabelValue("9"))
	})

	t.Run("Add alert like third, but with lower severity", func(t *testing.T) {
		alert := &model.Alert{
			Labels: model.LabelSet{
				"severity_level": "5",
				"instance":       "192.168.199.91:9650",
				"job":            "custom-my-app",
				"namespace":      "d8-system",
				"pod":            "deckhouse-aaa",
				"prometheus":     "deckhouse",
				"service":        "deckhouse",
			},
		}
		err := store.insertAlert(alert)
		require.NoError(t, err)
		require.Equal(t, len(store.alerts), 2)
		require.Equal(t, store.alerts[fingerprintWithoutSeverity(alert)].Alert.Labels[severityLabel], model.LabelValue("9"))
	})
}
