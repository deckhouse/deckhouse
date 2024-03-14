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
