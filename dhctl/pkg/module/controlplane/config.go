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
	"context"
	"fmt"
	"log/slog"

	"github.com/go-openapi/spec"
	"github.com/name212/govalue"
	"k8s.io/apimachinery/pkg/api/resource"

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
	edition     string
	logger      *slog.Logger
	cfg         *config.MetaConfig
	schemaStore SchemaStore
}

func NewSettingsExtractor(cfg *config.MetaConfig, schemaStore SchemaStore, edition string, logger *slog.Logger) *SettingsExtractor {
	return &SettingsExtractor{
		cfg:         cfg,
		edition:     edition,
		logger:      logger,
		schemaStore: schemaStore,
	}
}

// SignatureMode
// if not cse returns NoSignatureMode
func (e *SettingsExtractor) SignatureMode() (string, error) {
	if govalue.IsNil(e.cfg) {
		return "", fmt.Errorf("Internal error: meta config was not passed to control-plane settings extractor")
	}

	logger := e.logger

	// TODO after enable signature for ee and fe after full ready sig-migrate
	// change to !config.IsEEEdition
	// and after change fix config_test.go (see TODO comments)
	if !config.IsCSEdition(e.edition) {
		// TODO fix cse to ee after enable in ee and fe
		logger.DebugContext(context.Background(), fmt.Sprintf("Got non-cse edition '%s'. Returning no signature mode", e.edition))
		return NoSignatureMode, nil
	}

	schema, err := e.schemaStore.GetModuleConfigSchema(moduleName)
	if err != nil {
		return "", fmt.Errorf("Cannot get signature mode schema for module %s: %w", moduleName, err)
	}

	defaultMode := e.findDefaultSignatureMode(schema)

	logger.DebugContext(context.Background(), fmt.Sprintf("Got ee edition, trying to extract signature mode"))

	mc := e.cfg.FindModuleConfig(moduleName)

	logAndReturnDefaultMode := func(msg string, args ...any) (string, error) {
		msg = fmt.Sprintf(msg, args...)
		logger.DebugContext(context.Background(), fmt.Sprintf("%s. Returning mode '%s'", msg, defaultMode))

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

func (e *SettingsExtractor) TemplateConfigForBootstrap(nodeIP string) (*TemplateConfig, error) {
	metaCfg := e.cfg
	clusterConfiguration, err := metaCfg.ClusterConfigMap()
	if err != nil {
		return nil, err
	}

	cfg := &TemplateConfig{
		RunType:              "ClusterBootstrap",
		NodeIP:               "$MY_IP", // bashible placeholder, replaced by envsubst
		NodeName:             "$MY_NODENAME",
		Registry:             metaCfg.Registry.Manifest().KubeadmContext().ToMap(),
		Images:               metaCfg.Images.ConvertToMap(),
		VersionMap:           metaCfg.VersionMap,
		ClusterConfiguration: clusterConfiguration,
	}

	if nodeIP != "" {
		cfg.NodeIP = nodeIP
	}

	mcSettings, err := e.extractSettings()
	if err != nil {
		return nil, fmt.Errorf("read control-plane-manager moduleConfig: %w", err)
	}

	if mcSettings == nil {
		mcSettings = make(map[string]any)
	}

	cfg.Settings = mcSettings

	apiServer, err := e.extractAPIServerSettings()
	if err != nil {
		return nil, fmt.Errorf("cannot extract apiserver settings: %w", err)
	}

	cfg.APIServer = apiServer

	return cfg, nil
}

// extractSettings returns the control-plane-manager ModuleConfig settings
// ready for template rendering. Returns nil when the ModuleConfig is absent.
// resourcesRequests.{cpu,memory} are replaced with milliCPU and memoryBytes (int64)
// so templates can use them directly in arithmetic.
func (e *SettingsExtractor) extractSettings() (map[string]any, error) {
	mc := e.cfg.FindModuleConfig(moduleName)
	if mc == nil {
		return nil, nil
	}

	out := make(map[string]interface{}, len(mc.Spec.Settings))
	for k, v := range mc.Spec.Settings {
		out[k] = v
	}

	if rr, ok := mc.Spec.Settings["resourcesRequests"].(map[string]interface{}); ok {
		milliCPU, memoryBytes, err := parseResourceRequests(rr)
		if err != nil {
			return nil, fmt.Errorf("parse resourcesRequests: %w", err)
		}
		parsed := map[string]interface{}{}
		if milliCPU != 0 {
			parsed["milliCPU"] = milliCPU
		}
		if memoryBytes != 0 {
			parsed["memoryBytes"] = memoryBytes
		}
		out["resourcesRequests"] = parsed
	}

	return out, nil
}

func (e *SettingsExtractor) extractAPIServerSettings() (map[string]any, error) {
	signMode, err := e.SignatureMode()
	if err != nil {
		return nil, err
	}

	if signMode == NoSignatureMode {
		return nil, nil
	}

	return map[string]any{
		"signature": signMode,
	}, nil
}

func (e *SettingsExtractor) findDefaultSignatureMode(schema *spec.Schema) string {
	logger := e.logger

	returnDefault := func(msg string) string {
		logger.DebugContext(context.Background(), fmt.Sprintf("%s, returning %s", msg, defaultSignatureMode))
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
		return returnDefault("property apiserver.signature is not a string")
	}

	res, ok := signatureProps.Default.(string)
	if !ok {
		return returnDefault("property apiserver.signature default is not a string")
	}

	return res
}

func parseResourceRequests(rr map[string]interface{}) (int64, int64, error) {
	var milliCPU int64
	var memoryBytes int64

	if cpu, _ := rr["cpu"].(string); cpu != "" {
		q, e := resource.ParseQuantity(cpu)
		if e != nil {
			return 0, 0, fmt.Errorf("cpu %q: %w", cpu, e)
		}
		milliCPU = q.MilliValue()
	}
	if mem, _ := rr["memory"].(string); mem != "" {
		q, e := resource.ParseQuantity(mem)
		if e != nil {
			return 0, 0, fmt.Errorf("memory %q: %w", mem, e)
		}
		memoryBytes = q.Value()
	}
	return milliCPU, memoryBytes, nil
}
