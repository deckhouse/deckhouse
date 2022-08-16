package server

import (
	"fmt"

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
		cfg = &discardConfig{}
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
			fmt.Println("DISCARDED", ns)
			return true
		}
	}

	if len(dp.Ingresses) > 0 {
		if _, ok := dp.Ingresses[namespacedIngress]; ok {
			fmt.Println("DISCARDED ING", namespacedIngress)
			return true
		}
	}

	fmt.Println("OK", namespacedIngress)

	return false
}
