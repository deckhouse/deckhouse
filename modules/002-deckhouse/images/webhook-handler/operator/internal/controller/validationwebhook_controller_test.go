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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func setupTestReconciler() *ValidationWebhookReconciler {
	// create fake kubernetes client
	sch := runtime.NewScheme()
	deckhouseiov1alpha1.AddToScheme(sch)
	k8sClient := fake.NewClientBuilder().WithScheme(sch).Build()

	return &ValidationWebhookReconciler{
		Client: k8sClient,
		Scheme: sch,
		Logger: log.NewLogger(log.WithLevel(slog.LevelDebug)),
	}
}

func getStructFromYamlFile(filename string) (*deckhouseiov1alpha1.ValidationWebhook, error) {
	// open sample yaml
	sampleFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// convert sample to json (to unmarshal)
	jsonData, err := yaml.YAMLToJSON(sampleFile)
	if err != nil {
		return nil, err
	}
	// fmt.Println(string(jsonData))

	// unmarshal sample
	var vh *deckhouseiov1alpha1.ValidationWebhook
	err = json.Unmarshal(jsonData, &vh)
	if err != nil {
		return nil, err
	}

	return vh, nil
}

func TestTemplateNoError(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/sample.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)
}

func TestTemplateNoContext(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/sample_without_context.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)
}

func TestTemplateTwoContext(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/sample_two_context.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)
}

// TODO: complete compare logic
func TestTemplateEqual(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/prometheusremotewrites.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)
}
