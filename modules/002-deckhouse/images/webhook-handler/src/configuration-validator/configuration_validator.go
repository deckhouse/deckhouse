/*
Copyright 2025 Flant JSC

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
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var secretNameToConfig = map[string]string{
	"d8-cluster-configuration":          "cluster-configuration.yaml",
	"d8-provider-cluster-configuration": "cloud-provider-cluster-configuration.yaml",
	"d8-static-cluster-configuration":   "static-cluster-configuration.yaml",
}

func main() {
	if len(os.Args) > 1 {
		help()
		os.Exit(0)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Printf("reading input: %v", err)
		os.Exit(1)
	}

	log.InitLoggerWithOptions("silent", log.LoggerOptions{})
	schemaStore := config.NewSchemaStore()

	err = validate(schemaStore, data)
	if err != nil {
		fmt.Printf("validating object: %v", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func help() {
	fmt.Println(`Usage: configuration-validator [OPTION]
Validates D8 configuration against spec.
Reads input from STDIN.

  --help	display this help and exit.`)
}

func validate(schemaStore *config.SchemaStore, data []byte) error {
	secret, err := secretFromBytes(data)
	if err != nil {
		return err
	}

	cfgDataKey, ok := secretNameToConfig[secret.Name]
	if !ok {
		return fmt.Errorf("config key for secret %s not found", secret.Name)
	}

	cfg, ok := secret.Data[cfgDataKey]
	if !ok {
		return fmt.Errorf("config data for key %s not found", cfgDataKey)
	}

	_, err = schemaStore.Validate(&cfg, config.ValidateOptionOmitDocInError(true))
	if err != nil {
		return err
	}

	return nil
}

func secretFromBytes(data []byte) (*corev1.Secret, error) {
	decoder := scheme.Codecs.UniversalDeserializer()

	obj, gvk, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decoding object: %w", err)
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("object is not of type corev1.Secret: %s", gvk.String())
	}

	return secret, nil
}
