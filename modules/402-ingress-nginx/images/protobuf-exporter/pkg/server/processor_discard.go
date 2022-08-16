/*
Copyright 2022 Flant JSC

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

package server

import (
	"github.com/prometheus/common/log"
)

type discardConfig struct {
	Namespaces []string `yaml:"namespaces"`
	Ingresses  []string `yaml:"ingresses"`
}

type discardProcessor struct {
	Namespaces map[string]struct{}
	Ingresses  map[string]struct{}
}

// newDiscardProcessor message processor decides which messages could be dropped prometheus exporter
func newDiscardProcessor(cfg *discardConfig) *discardProcessor {
	if cfg == nil {
		return &discardProcessor{
			Namespaces: make(map[string]struct{}),
			Ingresses:  make(map[string]struct{}),
		}
	}
	log.Infof("Exclude metrics from namespaces: %v", cfg.Namespaces)
	log.Infof("Exclude metrics from ingresses: %v", cfg.Ingresses)

	nss, ings := make(map[string]struct{}, len(cfg.Namespaces)), make(map[string]struct{}, len(cfg.Ingresses))

	for _, ns := range cfg.Namespaces {
		nss[ns] = struct{}{}
	}

	for _, ing := range cfg.Ingresses {
		ings[ing] = struct{}{}
	}

	return &discardProcessor{
		Namespaces: nss,
		Ingresses:  ings,
	}
}

// IsDiscarded discard message based on annotations
func (dp *discardProcessor) IsDiscarded(msgAnnotations map[string]string) bool {
	ns, ok := msgAnnotations["namespace"]
	if !ok {
		return false
	}
	ingress, ok := msgAnnotations["ingress"]
	if !ok {
		return false
	}

	namespacedIngress := ns + ":" + ingress

	if len(dp.Namespaces) > 0 {
		if _, ok := dp.Namespaces[ns]; ok {
			return true
		}
	}

	if len(dp.Ingresses) > 0 {
		if _, ok := dp.Ingresses[namespacedIngress]; ok {
			return true
		}
	}

	return false
}
