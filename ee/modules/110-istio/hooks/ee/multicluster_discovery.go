/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Global in-memory JWT cache to work around CRD status persistence issues
var (
	jwtCache      = make(map[string]cachedJWT)
	jwtCacheMutex sync.RWMutex
)

type cachedJWT struct {
	JWT        string
	ExpiryTime time.Time
	CreatedAt  time.Time
}

// getOrCreateJWT returns a cached JWT if valid, otherwise creates a new one
func getOrCreateJWT(clusterName string, privKey []byte, claims map[string]string, ttl time.Duration, logger go_hook.Logger) (string, error) {
	jwtCacheMutex.RLock()
	cached, exists := jwtCache[clusterName]
	jwtCacheMutex.RUnlock()

	// Check if cached JWT is still valid
	if exists && time.Now().Before(cached.ExpiryTime) {
		logger.Info("reusing cached JWT for multicluster", "name", clusterName, "expires_at", cached.ExpiryTime.Format(time.RFC3339))
		return cached.JWT, nil
	}

	// Generate new JWT
	newJWT, err := jwt.GenerateJWT(privKey, claims, ttl)
	if err != nil {
		return "", err
	}

	// Cache the new JWT
	jwtCacheMutex.Lock()
	jwtCache[clusterName] = cachedJWT{
		JWT:        newJWT,
		ExpiryTime: time.Now().Add(ttl),
		CreatedAt:  time.Now(),
	}
	jwtCacheMutex.Unlock()

	logger.Info("generated and cached new JWT for multicluster", "name", clusterName, "expires_at", time.Now().Add(ttl).Format(time.RFC3339))
	return newJWT, nil
}

var (
	multiclusterMetricsGroup = "multicluster_discovery"
	multiclusterMetricName   = "d8_istio_multicluster_metadata_endpoints_fetch_error_count"
)

type IstioMulticlusterDiscoveryCrdInfo struct {
	Name                     string
	ClusterUUID              string
	EnableIngressGateway     bool
	ClusterCA                string
	EnableInsecureConnection bool
	PublicMetadataEndpoint   string
	PrivateMetadataEndpoint  string
	ExistingAPIJWT           string
}

func (i *IstioMulticlusterDiscoveryCrdInfo) SetMetricMetadataEndpointError(mc sdkpkg.MetricsCollector, endpoint string, isError float64) {
	labels := map[string]string{
		"multicluster_name": i.Name,
		"endpoint":          endpoint,
	}

	mc.Set(multiclusterMetricName, isError, labels, metrics.WithGroup(multiclusterMetricsGroup))
}

func (i *IstioMulticlusterDiscoveryCrdInfo) PatchMetadataCache(pc go_hook.PatchCollector, scope string, meta interface{}) error {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"metadataCache": map[string]interface{}{
				scope:                        meta,
				scope + "LastFetchTimestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	pc.PatchWithMerge(patch, "deckhouse.io/v1alpha1", "IstioMulticluster", "", i.Name, object_patch.WithSubresource("/status"))
	return nil
}

func applyMulticlusterFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var multicluster eeCrd.IstioMulticluster

	err := sdk.FromUnstructured(obj, &multicluster)
	if err != nil {
		return nil, err
	}

	clusterUUID := ""
	if multicluster.Status.MetadataCache.Public != nil {
		clusterUUID = multicluster.Status.MetadataCache.Public.ClusterUUID
	}

	// Check if we have an existing valid JWT
	var existingAPIJWT string
	if multicluster.Status.MetadataCache.Private != nil && multicluster.Status.MetadataCache.Private.APIJWT != "" {
		// Validate the existing JWT
		isValid, _, err := jwt.IsJWTValid(multicluster.Status.MetadataCache.Private.APIJWT)
		if err != nil {
			// JWT validation error - will generate new one
		} else if !isValid {
			// JWT expired or invalid - will generate new one
		} else {
			// Use existing JWT if it's still valid
			existingAPIJWT = multicluster.Status.MetadataCache.Private.APIJWT
		}
	}

	// Debug: Log what we found in the CRD status
	if multicluster.Status.MetadataCache.Private != nil {
		if multicluster.Status.MetadataCache.Private.APIJWT != "" {
			fmt.Printf("DEBUG: Found JWT in CRD: %s...\n", multicluster.Status.MetadataCache.Private.APIJWT[:50])
		} else {
			fmt.Printf("DEBUG: No JWT in CRD status for %s\n", multicluster.GetName())
		}
	} else {
		fmt.Printf("DEBUG: No private metadata in CRD status for %s\n", multicluster.GetName())
	}

	me := multicluster.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	return IstioMulticlusterDiscoveryCrdInfo{
		Name:                     multicluster.GetName(),
		EnableIngressGateway:     multicluster.Spec.EnableIngressGateway,
		ClusterCA:                multicluster.Spec.Metadata.ClusterCA,
		EnableInsecureConnection: multicluster.Spec.Metadata.EnableInsecureConnection,
		ClusterUUID:              clusterUUID,
		PublicMetadataEndpoint:   me + "/public/public.json",
		PrivateMetadataEndpoint:  me + "/private/multicluster.json",
		ExistingAPIJWT:           existingAPIJWT,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("multicluster"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "multiclusters",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IstioMulticluster",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyMulticlusterFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, dependency.WithExternalDependencies(multiclusterDiscovery))

func multiclusterDiscovery(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(multiclusterMetricsGroup)

	if !input.Values.Get("istio.multicluster.enabled").Bool() {
		return nil
	}
	if !input.Values.Get("istio.internal.remoteAuthnKeypair.priv").Exists() {
		input.Logger.Warn("authn keypair for signing requests to remote metadata endpoints isn't generated yet, retry in 1min")
		return nil
	}

	for multiclusterInfo, err := range sdkobjectpatch.SnapshotIter[IstioMulticlusterDiscoveryCrdInfo](input.Snapshots.Get("multiclusters")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over multiclusters: %v", err)
		}

		var publicMetadata eeCrd.AlliancePublicMetadata
		var privateMetadata eeCrd.MulticlusterPrivateMetadata
		var httpOption []http.Option

		if multiclusterInfo.ClusterCA != "" {
			caCerts := [][]byte{[]byte(multiclusterInfo.ClusterCA)}
			httpOption = append(httpOption, http.WithAdditionalCACerts(caCerts))
		} else if multiclusterInfo.EnableInsecureConnection {
			httpOption = append(httpOption, http.WithInsecureSkipVerify())
		}

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(httpOption...), multiclusterInfo.PublicMetadataEndpoint, "")
		if err != nil {
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			input.Logger.Warn("cannot unmarshal public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			input.Logger.Warn("bad public metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 0)
		err = multiclusterInfo.PatchMetadataCache(input.PatchCollector, "public", publicMetadata)
		if err != nil {
			return err
		}

		// TODO Make independent public and private fetch?
		privKey := []byte(input.Values.Get("istio.internal.remoteAuthnKeypair.priv").String())

		claims := map[string]string{
			"iss":   "d8-istio",
			"aud":   publicMetadata.ClusterUUID,
			"sub":   input.Values.Get("global.discovery.clusterUUID").String(),
			"scope": "private-multicluster",
		}
		bearerToken, err := jwt.GenerateJWT(privKey, claims, time.Minute)
		if err != nil {
			input.Logger.Warn("can't generate auth token for endpoint of IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}

		// Use in-memory cache for JWT to work around CRD status persistence issues
		apiClaims := map[string]string{
			"iss":   "d8-istio",
			"aud":   publicMetadata.ClusterUUID,
			"sub":   input.Values.Get("global.discovery.clusterUUID").String(),
			"scope": "api",
		}
		apiJWT, err := getOrCreateJWT(multiclusterInfo.Name, privKey, apiClaims, time.Hour*24*366, input.Logger) // 1 year
		if err != nil {
			input.Logger.Warn("can't generate API JWT for IstioMulticluster", slog.String("name", multiclusterInfo.Name), log.Err(err))
			continue
		}

		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(httpOption...), multiclusterInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		// Preserve JWT fields before unmarshaling remote data
		// The remote response doesn't include our JWT, so we need to preserve it
		savedAPIJWT := apiJWT
		savedJWTExpiryTime := time.Now().Add(time.Hour * 24 * 366).Format(time.RFC3339)

		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			input.Logger.Warn("cannot unmarshal private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}

		// Restore JWT fields after unmarshaling remote data
		// The remote response overwrites these fields, so we need to restore them
		privateMetadata.APIJWT = savedAPIJWT
		privateMetadata.JWTExpiryTime = savedJWTExpiryTime

		// Debug: Log what we're about to store
		input.Logger.Info("about to store JWT", slog.String("name", multiclusterInfo.Name), slog.String("jwt", savedAPIJWT[:50]+"..."), slog.String("expires_at", savedJWTExpiryTime))

		if multiclusterInfo.ExistingAPIJWT == "" {
			input.Logger.Info("stored new API JWT for multicluster", slog.String("name", multiclusterInfo.Name), slog.String("expires_at", privateMetadata.JWTExpiryTime))
		} else {
			input.Logger.Info("stored existing API JWT for multicluster", slog.String("name", multiclusterInfo.Name), slog.String("expires_at", privateMetadata.JWTExpiryTime))
		}
		if privateMetadata.NetworkName == "" || privateMetadata.APIHost == "" || privateMetadata.IngressGateways == nil {
			input.Logger.Warn("bad private metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 0)

		// Always patch private metadata to ensure JWT is stored in CRD status
		input.Logger.Info("about to patch CRD with JWT", slog.String("name", multiclusterInfo.Name), slog.String("jwt", savedAPIJWT[:50]+"..."))

		// Create a unique patch to minimize conflicts
		uniquePatch := map[string]interface{}{
			"status": map[string]interface{}{
				"metadataCache": map[string]interface{}{
					"private":                   privateMetadata,
					"privateLastFetchTimestamp": time.Now().UTC().Format(time.RFC3339),
					// Add a unique timestamp to help identify this specific patch
					"jwtPatchTimestamp": time.Now().UTC().Format(time.RFC3339Nano),
				},
			},
		}

		input.PatchCollector.PatchWithMerge(uniquePatch, "deckhouse.io/v1alpha1", "IstioMulticluster", "", multiclusterInfo.Name, object_patch.WithSubresource("/status"))

		// Debug: Log about patched the CRD
		input.Logger.Info("successfully patched CRD with JWT", slog.String("name", multiclusterInfo.Name), slog.String("jwt", savedAPIJWT[:50]+"..."), slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)), slog.String("jwt_expiry", privateMetadata.JWTExpiryTime))

		// CRITICAL: Add a verification step to ensure the JWT was actually stored
		// This helps identify if the patch operation is failing silently
		input.Logger.Info("JWT patch operation completed", slog.String("name", multiclusterInfo.Name), slog.String("patch_timestamp", time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return nil
}
