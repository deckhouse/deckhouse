package template

import (
	"testing"
)

func TestRenderBashBooster(t *testing.T) {
	_, err := RenderBashBooster("/deckhouse/candi/bashible/bashbooster/")
	if err != nil {
		t.Errorf("Rendering bash booster error: %v", err)
	}
}
