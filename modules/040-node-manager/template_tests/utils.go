package template_tests

import (
	"encoding/base64"
	"fmt"

	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func getDecodedSecretValue(s *object_store.KubeObject, path string) string {
	fullPath := fmt.Sprintf("data.%s", path)
	return decodeK8sObjField(s, fullPath)
}

func decodeK8sObjField(o *object_store.KubeObject, fullPath string) string {
	encodedVal := o.Field(fullPath).String()

	decodedArray, err := base64.StdEncoding.DecodeString(encodedVal)

	decodedVal := ""
	if err == nil {
		decodedVal = string(decodedArray)
	}

	return decodedVal
}
