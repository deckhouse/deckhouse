package api

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type SWHCCapabilities struct {
	svc *Service

	mu        sync.RWMutex
	lastCheck time.Time
	ttl       time.Duration
	lastErr   error

	crdPresent    bool
	moduleEnabled bool
	edition       string
}

func NewSWHCCapabilities(svc *Service) *SWHCCapabilities {
	return &SWHCCapabilities{
		svc:     svc,
		ttl:     60 * time.Second,
		edition: detectDeckhouseEdition(),
	}
}

func (c *SWHCCapabilities) CanUseSWHC(ctx context.Context) (bool, error) {
	if err := c.refreshIfStale(ctx); err != nil {
		return false, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.crdPresent && c.moduleEnabled, nil
}

func (c *SWHCCapabilities) Snapshot() (crdPresent, moduleEnabled bool, edition string, lastErr error, lastCheck time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.crdPresent, c.moduleEnabled, c.edition, c.lastErr, c.lastCheck
}

func (c *SWHCCapabilities) refreshIfStale(ctx context.Context) error {
	c.mu.RLock()
	stale := time.Since(c.lastCheck) > c.ttl
	c.mu.RUnlock()
	if !stale {
		return nil
	}
	return c.refresh(ctx)
}

func (c *SWHCCapabilities) refresh(ctx context.Context) error {
	crdPresent, crdErr := c.detectSWHCCRD(ctx)
	moduleEnabled, modErr := c.detectSWHCModuleEnabled(ctx)

	var finalErr error
	if crdErr != nil {
		finalErr = crdErr
	} else if modErr != nil {
		finalErr = modErr
	}

	c.mu.Lock()
	c.crdPresent = crdPresent
	c.moduleEnabled = moduleEnabled
	c.lastErr = finalErr
	c.lastCheck = time.Now()
	c.mu.Unlock()

	if finalErr != nil {
		klog.V(4).InfoS("SWHC capabilities refresh completed with error",
			"crdPresent", crdPresent, "moduleEnabled", moduleEnabled, "err", finalErr)
	}
	return finalErr
}

func (c *SWHCCapabilities) detectSWHCCRD(ctx context.Context) (bool, error) {
	discovery := c.svc.clientset.Discovery()
	res, err := discovery.ServerResourcesForGroupVersion("network.deckhouse.io/v1alpha1")
	if err != nil {
		return false, err
	}
	for _, r := range res.APIResources {
		if r.Name == "servicewithhealthchecks" {
			return true, nil
		}
	}
	return false, nil
}

// detectSWHCModuleEnabled reads ModuleConfig/service-with-healthchecks (cluster-scoped).
func (c *SWHCCapabilities) detectSWHCModuleEnabled(ctx context.Context) (bool, error) {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("deckhouse.io/v1alpha1")
	u.SetKind("ModuleConfig")

	// ModuleConfig is cluster-scoped => no Namespace here.
	key := types.NamespacedName{Name: "service-with-healthchecks"}
	if err := c.svc.client.Get(ctx, key, u); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	enabled, found, err := unstructured.NestedBool(u.Object, "spec", "enabled")
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	return enabled, nil
}

func detectDeckhouseEdition() string {
	candidates := []string{
		"DECKHOUSE_EDITION",
		"D8_EDITION",
		"DVP_EDITION",
	}
	for _, env := range candidates {
		v := strings.TrimSpace(os.Getenv(env))
		if v == "" {
			continue
		}
		v = strings.ToUpper(v)
		switch {
		case strings.Contains(v, "EE"):
			return "EE"
		case strings.Contains(v, "CE"):
			return "CE"
		default:
			return v
		}
	}
	return "Unknown"
}
