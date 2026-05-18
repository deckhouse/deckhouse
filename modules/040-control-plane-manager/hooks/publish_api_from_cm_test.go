package hooks

import (
	"encoding/json"
	"testing"
)

func TestPublishAPIConfigUnmarshalJSON_SupportsLegacyEnable(t *testing.T) {
	t.Parallel()

	var config Config
	if err := json.Unmarshal([]byte(`{"enable":true}`), &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if config.Enabled == nil {
		t.Fatal("expected enabled value to be populated from legacy enable")
	}
	if !*config.Enabled {
		t.Fatalf("expected enabled=true, got %v", *config.Enabled)
	}
}

func TestPublishAPIConfigUnmarshalJSON_PrefersEnabledOverLegacyEnable(t *testing.T) {
	t.Parallel()

	var config Config
	if err := json.Unmarshal([]byte(`{"enabled":true,"enable":false}`), &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if config.Enabled == nil {
		t.Fatal("expected enabled value to be populated")
	}
	if !*config.Enabled {
		t.Fatalf("expected enabled=true to win over legacy enable, got %v", *config.Enabled)
	}
}
