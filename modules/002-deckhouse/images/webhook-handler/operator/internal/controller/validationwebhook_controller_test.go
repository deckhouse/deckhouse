package controller

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"text/template"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
)

func TestTemplateNoError(t *testing.T) {
	// hooks/002-deckhouse/webhooks/validating
	// os.MkdirAll("/hooks/"+vh.Name+"/webhooks/validating/", 0777)

	sampleFile, err := os.ReadFile("testdata/sample.yaml")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	var vh *deckhouseiov1alpha1.ValidationWebhook
	err = json.Unmarshal(sampleFile, &vh)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
		return
	}

	templateFile := "templates/webhook.tpl"

	tpl, err := template.ParseFiles(templateFile)
	// template.
	// Funcs(template.FuncMap{
	// 	"toYaml": func(str string) string {
	// 		res, err := yaml.Marshal(str)
	// 		if err != nil {
	// 			return err.Error()
	// 		}
	// 		return string(res)
	// 	},
	// }).

	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
		return
	}

	err = tpl.Execute(os.Stdout, vh)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
		return
	}
}
