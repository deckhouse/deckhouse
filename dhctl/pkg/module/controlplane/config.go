// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"fmt"

	"github.com/go-openapi/spec"
	"github.com/name212/govalue"

	"github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const (
	NoSignatureMode      = ""
	defaultSignatureMode = "Rollback"
	moduleName           = "control-plane-manager"
)

type SchemaStore interface {
	GetModuleConfigSchema(string) (*spec.Schema, error)
}

type SettingsExtractor struct {
	edition        string
	loggerProvider log.LoggerProvider
	cfg            *config.MetaConfig
	schemaStore    SchemaStore
}

func NewSettingsExtractor(cfg *config.MetaConfig, schemaStore SchemaStore, edition string, loggerProvider log.LoggerProvider) *SettingsExtractor {
	return &SettingsExtractor{
		cfg:            cfg,
		edition:        edition,
		loggerProvider: loggerProvider,
		schemaStore:    schemaStore,
	}
}

// SignatureMode
// if not cse returns NoSignatureMode
func (e *SettingsExtractor) SignatureMode() (string, error) {
	if govalue.IsNil(e.cfg) {
		return "", fmt.Errorf("Internal error: meta config did not pass to control-plane settings extractor")
	}

	logger := e.loggerProvider()

	if !config.IsEEEdition(e.edition) {
		logger.DebugF("Got not ee edition '%s'. Returns no signature mode", e.edition)
		return NoSignatureMode, nil
	}

	schema, err := e.schemaStore.GetModuleConfigSchema(moduleName)
	if err != nil {
		return "", fmt.Errorf("Cannot get signature mode schema for module %s: %w", moduleName, err)
	}

	defaultMode := e.findDefaultSignatureMode(schema)

	logger.DebugF("Got ee edition try to extract signature mode")

	mc := e.cfg.FindModuleConfig(moduleName)

	logAndReturnDefaultMode := func(msg string, args ...any) (string, error) {
		msg = fmt.Sprintf(msg, args...)
		logger.DebugF("%s. Returns mode '%s'", msg, defaultMode)

		return defaultMode, nil
	}

	if govalue.IsNil(mc) {
		return logAndReturnDefaultMode("Module config not found")
	}

	apiServerRaw, ok := mc.Spec.Settings["apiserver"]
	if !ok {
		return logAndReturnDefaultMode("apiserver settings key not found")
	}

	apiServer, ok := apiServerRaw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("Cannot convert apiserver key to map. It is %T", apiServerRaw)
	}

	signatureRaw, ok := apiServer["signature"]
	if !ok {
		return logAndReturnDefaultMode("apiserver.signature settings key not found")

	}

	signature, ok := signatureRaw.(string)
	if !ok {
		return "", fmt.Errorf("Cannot convert apiserver.signature key to string. It is %T", signatureRaw)
	}

	return signature, nil
}

func (e *SettingsExtractor) findDefaultSignatureMode(schema *spec.Schema) string {
	logger := e.loggerProvider()

	returnDefault := func(msg string) string {
		logger.DebugF("%s return %s", msg, defaultSignatureMode)
		return defaultSignatureMode
	}

	apiServer, ok := schema.Properties["apiserver"]
	if !ok {
		return returnDefault("property apiserver not found")
	}

	signature, ok := apiServer.SchemaProps.Properties["signature"]
	if !ok {
		return returnDefault("property apiserver.signature not found")
	}

	signatureProps := signature.SchemaProps

	if !signatureProps.Type.Contains("string") {
		return returnDefault("property apiserver.signature is not string")
	}

	res, ok := signatureProps.Default.(string)
	if !ok {
		return returnDefault("property apiserver.signature default is not string")
	}

	return res
}
