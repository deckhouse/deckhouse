/*
Copyright 2026 Flant JSC

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

package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/internal/config"
	"kubernetes-api-proxy/internal/upstream"
	kutils "kubernetes-api-proxy/pkg/kubernetes"
	"kubernetes-api-proxy/pkg/utils"
)

const (
	ns                    = "default"
	es                    = "kubernetes"
	fallbackReconcileIter = 61
)

// StartDiscovery runs a loop that periodically fetches a Kubernetes Endpoint Slice
// object and sends the derived list of upstream addresses to the provided
// list. It performs a simple diff to avoid spamming identical updates and
// applies backpressure handling to always keep the most recent update.
func StartDiscovery(
	ctx context.Context,
	cfg config.Config,
	logger *log.Logger,
	mainList *upstream.List,
	fallbackList *upstream.FallbackList,
) {
	ticker := time.NewTicker(cfg.DiscoverPeriod)
	defer ticker.Stop()

	if err := discovery(ctx, logger, kutils.BuildGetter(cfg, fallbackList), mainList); err != nil {
		logger.Error("initial discovery failed", slog.String("error", err.Error()))
	}

	fallbackListReconcileCounter := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := discovery(ctx, logger, kutils.BuildGetter(cfg, fallbackList), mainList); err != nil {
				logger.Error("discovery failed", slog.String("error", err.Error()))
			} else {
				fallbackListReconcileCounter++
			}

			if fallbackListReconcileCounter >= fallbackReconcileIter {
				fallbackListReconcileCounter = 0

				logger.Debug(
					"update fallback list",
					slog.Any("list", mainList.ListFullAddresses()),
				)

				fallbackList.UpdateFromList(mainList)
			}
		}
	}
}

func discovery(
	ctx context.Context,
	logger *log.Logger,
	clusterConfigGetter func() (*rest.Config, error),
	mainList *upstream.List,
) error {
	clusterConfig, err := clusterConfigGetter()
	if err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	cs, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to init k8s client for discovery: %w", err)
	}

	logger.Debug("discovery: run")

	slice, err := cs.DiscoveryV1().EndpointSlices(ns).Get(ctx, es, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to list endpoint slices: %w", err)
	}

	addrs := endpointSliceToUpstreams(slice)
	logger.Debug("discovery: endpointslices fetched",
		slog.Int("derived_upstreams", len(addrs)),
	)

	if len(addrs) == 0 {
		if hp := apiServerAddrFromRestHost(clusterConfig.Host); hp != "" {
			addrs = []string{hp}

			logger.Debug("discovery: fallback to clusterConfig.Host",
				slog.String("host", clusterConfig.Host),
				slog.String("upstream", hp),
			)
		}
	}

	mainList.Reconcile(
		utils.Map(
			addrs,
			upstream.NewUpstream,
		),
	)

	return nil
}

// endpointSliceToUpstreams converts EndpointSlices for the default/kubernetes
// service into a list of host:port addresses. Prefer port named "https",
// then the first defined, defaulting to 6443 if none.
func endpointSliceToUpstreams(es *discoveryv1.EndpointSlice) []string {
	if es == nil {
		return nil
	}
	result := make([]string, 0)

	var port int32
	for _, p := range es.Ports {
		if p.Name != nil && *p.Name == "https" && p.Port != nil && *p.Port > 0 {
			port = *p.Port
			break
		}
	}

	if port == 0 {
		for _, p := range es.Ports {
			if p.Port != nil && *p.Port > 0 {
				port = *p.Port
				break
			}
		}
	}

	if port == 0 {
		port = 6443
	}

	for _, ep := range es.Endpoints {
		ready := true
		if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
			ready = false
		}
		if !ready {
			continue
		}
		for _, addr := range ep.Addresses {
			if addr != "" {
				result = append(result, net.JoinHostPort(addr, strconv.Itoa(int(port))))
			}
		}
	}

	return result
}

// apiServerAddrFromRestHost converts rest.Config.Host to host:port, defaulting
// to 6443 if port is not specified. Accepts values with or without scheme.
func apiServerAddrFromRestHost(host string) string {
	if host == "" {
		return ""
	}
	// Ensure we have a scheme for url parsing
	raw := host
	if !hasSchemePrefix(host) {
		raw = "https://" + host
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	h := u.Hostname()
	p := u.Port()
	if h == "" {
		return ""
	}
	if p == "" {
		p = "6443"
	}
	return net.JoinHostPort(h, p)
}

func hasSchemePrefix(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
