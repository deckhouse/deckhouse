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

	// init template file
	tpl, err := os.ReadFile("templates/webhook.tpl")
	if err != nil {
		panic(err)
	}

	return &ValidationWebhookReconciler{
		Client:   k8sClient,
		Scheme:   sch,
		Logger:   log.NewLogger(log.WithLevel(slog.LevelDebug)),
		Template: string(tpl),
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

// ------------------
// --- TEST-CASES ---
// ------------------

func TestTemplateNoError(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/sample.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/golden/validationwebhook-sample.py")
	assert.NoError(t, err)
	res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))
}

func TestTemplateNoContext(t *testing.T) {
	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/sample_without_context.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/golden/sample_without_context.py")
	assert.NoError(t, err)
	res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))
}

func TestTemplateTwoContext(t *testing.T) {
	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/sample_two_context.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	// test equality
	ref, err := os.ReadFile("testdata/golden/sample_two_context.py")
	assert.NoError(t, err)
	res, err := os.ReadFile("hooks/validationwebhook-sample/webhooks/validating/validationwebhook-sample.py")
	assert.NoError(t, err)
	assert.Equal(t, string(ref), string(res))
}

// TODO: complete compare logic
func TestTemplateEqual(t *testing.T) {
	r := setupTestReconciler()

	vh, err := getStructFromYamlFile("testdata/prometheusremotewrite.yaml")
	assert.NoError(t, err)

	_, err = r.handleProcessValidatingWebhook(context.TODO(), vh)
	assert.NoError(t, err)

	ref, err := os.ReadFile("testdata/golden/prometheusremotewrite.py")
	assert.NoError(t, err)

	res, err := os.ReadFile("hooks/prometheusremotewrite/webhooks/validating/prometheusremotewrite.py")
	assert.NoError(t, err)

	assert.Equal(t, string(ref), string(res))
}
