package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestTemplateNoError(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	r := &ValidationWebhookReconciler{
		Logger: log.NewLogger(log.WithLevel(slog.LevelDebug)),
	}

	sampleFile, err := os.ReadFile("testdata/sample.yaml")
	assert.NoError(t, err)

	jsonData, err := yaml.YAMLToJSON(sampleFile)
	assert.NoError(t, err)
	// fmt.Println(string(jsonData))

	var vh *deckhouseiov1alpha1.ValidationWebhook
	err = json.Unmarshal(jsonData, &vh)
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)
}
