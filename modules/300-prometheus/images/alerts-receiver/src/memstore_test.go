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
		store.insertAlert(alert)
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
		store.insertAlert(alert)
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
		store.insertAlert(alert)
		require.Equal(t, len(store.alerts), 2)
		require.Equal(t, store.alerts[fingerprint(alert)].Alert.Labels[severityLabel], model.LabelValue("9"))
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
		store.insertAlert(alert)
		require.Equal(t, len(store.alerts), 2)
		require.Equal(t, store.alerts[fingerprint(alert)].Alert.Labels[severityLabel], model.LabelValue("9"))
	})

}
