/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package metadataExporter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/tidwall/gjson"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	AllianceKindMulticluster AllianceKind = "IstioMulticluster"
	AllianceKindFederation   AllianceKind = "IstioFederation"
)

const (
	TypeMulticluster DiscoveryType = "multicluster"
	TypeFederation   DiscoveryType = "federation"
)

const (
	scopePublic  discoveryScope = "public"
	scopePrivate discoveryScope = "private"
)

const (
	MulticlusterMetricsGroup metricsGroup = "multicluster_discovery"
	MulticlusterMetricName   metricsName  = "d8_istio_multicluster_metadata_endpoints_fetch_error_count"
	FederationMetricsGroup   metricsGroup = "federation_discovery"
	FederationMetricName     metricsName  = "d8_istio_federation_metadata_endpoints_fetch_error_count"
)

type AllianceKind string
type DiscoveryType string
type discoveryScope string
type metricsGroup string
type metricsName string

type CommonInfo struct {
	AllianceKind             string
	Name                     string
	ClusterUUID              string
	ClusterCA                string
	EnableInsecureConnection bool
	PublicMetadataEndpoint   string
	PrivateMetadataEndpoint  string
	publicMetadata           *eeCrd.AlliancePublicMetadata
}

type MulticlusterCrdInfo struct {
	*CommonInfo
	EnableIngressGateway bool
}

type FederationCrdInfo struct {
	*CommonInfo
	TrustDomain string
}

type Discovery struct {
	Type  DiscoveryType
	input *go_hook.HookInput
	metricsGroup
	metricsName
	MyTrustDomain string
	dc            dependency.Container
}

type AllianceCRDInfo interface {
	validateWithoutSkipping(d *Discovery)
	validateWithSkipping(d *Discovery) error
	fetchPublicMetadata(d *Discovery) (bool, error)
	fetchPrivateMetadata(d *Discovery) (bool, error)
}

func New(input *go_hook.HookInput, dt DiscoveryType, dc dependency.Container) (*Discovery, error) {
	discovery := Discovery{
		Type:  dt,
		input: input,
		dc:    dc,
	}
	if !discovery.hasValidType() {
		return &discovery, fmt.Errorf("DiscoveryType '%s' is invalid", discovery.Type)
	}
	switch discovery.Type {
	case TypeMulticluster:
		discovery.metricsGroup = MulticlusterMetricsGroup
		discovery.metricsName = MulticlusterMetricName
	case TypeFederation:
		discovery.metricsGroup = FederationMetricsGroup
		discovery.metricsName = FederationMetricName
	}

	input.MetricsCollector.Expire(string(discovery.metricsGroup))

	if !discovery.isEnabled() {
		return &discovery, fmt.Errorf("Discovery for %s is not enabled", discovery.Type)
	}
	if !discovery.isReady() {
		return &discovery, fmt.Errorf("Discovery for %s is not ready", discovery.Type)
	}
	if discovery.isTypedAs(TypeFederation) {
		discovery.MyTrustDomain = input.Values.Get("global.discovery.clusterDomain").String()
	}
	return &discovery, nil
}

func (discovery *Discovery) RunDiscoveryOf(a AllianceCRDInfo) (bool, error) {
	if !discovery.shouldRunDiscoveryOf(a) {
		return true, nil // skipping
	}
	if skip, err := a.fetchPublicMetadata(discovery); skip {
		return true, nil // skipping
	} else if err != nil {
		return false, err
	} else {
		if skip, err := a.fetchPrivateMetadata(discovery); skip {
			return true, nil // skipping
		} else if err != nil {
			return false, err
		}
	}
	return false, nil // ok
}

func (d *Discovery) shouldRunDiscoveryOf(a AllianceCRDInfo) bool {
	a.validateWithoutSkipping(d)
	if err := a.validateWithSkipping(d); err != nil {
		return false
	}
	return true
}

func (c *CommonInfo) validateWithoutSkipping(d *Discovery) {
	c.ifHasDeprecatedSubdomainAlert(d)
}

func (c *FederationCrdInfo) validateWithSkipping(d *Discovery) error {
	if c.TrustDomain == d.MyTrustDomain {
		// alert probably?
		return fmt.Errorf("Federation '%s' has trust domain equal to local trust domain", c.Name)
	}
	return nil
}

func (c *MulticlusterCrdInfo) validateWithSkipping(d *Discovery) error {
	return nil
}

func (c *CommonInfo) fetchPublicMetadata(d *Discovery) (bool, error) {
	var meta eeCrd.AlliancePublicMetadata
	var scope = scopePublic

	if bodyBytes, ok := c.fetchMetadata(d, scope); !ok {
		return true, nil
	} else {
		// have got metadata
		if c.unmarshalMetadata(d, bodyBytes, &meta, scope) != nil {
			return true, nil
		}
		if meta.ClusterUUID == "" || meta.AuthnKeyPub == "" || meta.RootCA == "" {
			c.warnBadMetadataFormat(d, scope)
			return true, nil
		}
		c.setMetricMetadataEndpointError(d, scope, 0)
		c.patchMetadataCache(d, scope, meta)

		c.ClusterUUID = meta.ClusterUUID
		c.publicMetadata = &meta
		return false, nil
	}
}

func (c *MulticlusterCrdInfo) fetchPrivateMetadata(d *Discovery) (bool, error) {
	var meta eeCrd.MulticlusterPrivateMetadata
	var scope = scopePrivate

	if bodyBytes, ok := c.fetchMetadata(d, scope); !ok {
		return true, nil
	} else {
		// have got metadata
		if c.unmarshalMetadata(d, bodyBytes, &meta, scope) != nil {
			return true, nil
		}
		if meta.NetworkName == "" || meta.APIHost == "" || meta.IngressGateways == nil {
			c.warnBadMetadataFormat(d, scope)
			return true, nil
		}
		c.setMetricMetadataEndpointError(d, scope, 0)
		c.patchMetadataCache(d, scope, meta)

		return false, nil
	}
}

func (c *FederationCrdInfo) fetchPrivateMetadata(d *Discovery) (bool, error) {
	var meta eeCrd.FederationPrivateMetadata
	var scope = scopePrivate

	protocolMap := map[string]string{
		"https":    "TLS",
		"tls":      "TLS",
		"http":     "HTTP",
		"http2":    "HTTP2",
		"grpc":     "HTTP2",
		"grpc-web": "HTTP2",
	}

	defaultProtocol := "TCP"

	if bodyBytes, ok := c.fetchMetadata(d, scope); !ok {
		return true, nil
	} else {
		// have got metadata
		if c.unmarshalMetadata(d, bodyBytes, &meta, scope) != nil {
			return true, nil
		}
		if meta.IngressGateways == nil || meta.PublicServices == nil {
			c.warnBadMetadataFormat(d, scope)
			return true, nil
		}

		c.setMetricMetadataEndpointError(d, scope, 0)
		c.updatePortProtocols(meta.PublicServices, defaultProtocol, protocolMap)
		c.patchMetadataCache(d, scope, meta)

		var countServices = 0
		if meta.PublicServices != nil {
			countServices = len(*meta.PublicServices)
		}
		d.input.Logger.Info(fmt.Sprintf("Cluster name: %s connected successfully, published services: %s", d.MyTrustDomain, strconv.Itoa(countServices)))
		return false, nil
	}
}

func (c *CommonInfo) patchMetadataCache(d *Discovery, scope discoveryScope, meta interface{}) {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"metadataCache": map[string]interface{}{
				string(scope):                        meta,
				string(scope) + "LastFetchTimestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
	d.input.PatchCollector.PatchWithMerge(patch, "deckhouse.io/v1alpha1", c.AllianceKind, "", c.Name, object_patch.WithSubresource("/status"))
}

func (c *CommonInfo) fetchMetadata(d *Discovery, scope discoveryScope) ([]byte, bool) {
	bearerToken, err := d.generateBearerToken(c, scope)
	if err != nil {
		d.input.Logger.Warn(
			"can't generate auth token for metadata endpoint",
			slog.String("scope", string(scope)),
			slog.String("alliance_kind", c.AllianceKind),
			slog.String("endpoint", c.getEndpoint(scope)),
			slog.String("name", c.Name),
			log.Err(err),
		)
		c.setMetricMetadataEndpointError(d, scope, 1)
		return nil, false
	}

	var httpOption = c.prepareHTTPOptions()

	body, code, err := lib.HTTPGet(d.dc.GetHTTPClient(httpOption...), c.getEndpoint(scope), bearerToken)
	if err != nil {
		d.input.Logger.Warn("cannot fetch metadata endpoint",
			slog.String("scope", string(scope)),
			slog.String("alliance_kind", c.AllianceKind),
			slog.String("endpoint", c.getEndpoint(scope)),
			slog.String("name", c.Name),
			log.Err(err),
		)
		c.setMetricMetadataEndpointError(d, scope, 1)
		return nil, false
	}
	if code != 200 {
		d.input.Logger.Warn("cannot fetch metadata endpoint",
			slog.String("scope", string(scope)),
			slog.String("alliance_kind", c.AllianceKind),
			slog.String("endpoint", c.getEndpoint(scope)),
			slog.String("name", c.Name),
			slog.Int("http_code", code))
		c.setMetricMetadataEndpointError(d, scope, 1)
		return nil, false
	}
	return body, true
}

func (c *CommonInfo) warnBadMetadataFormat(d *Discovery, scope discoveryScope) {
	d.input.Logger.Warn(
		"bad metadata format in endpoint",
		slog.String("scope", string(scope)),
		slog.String("alliance_kind", c.AllianceKind),
		slog.String("endpoint", c.getEndpoint(scope)),
		slog.String("name", c.Name),
	)
	c.setMetricMetadataEndpointError(d, scope, 1)
}

func (c *CommonInfo) unmarshalMetadata(d *Discovery, bodyBytes []byte, meta interface{}, scope discoveryScope) error {
	err := json.Unmarshal(bodyBytes, meta)
	if err != nil {
		d.input.Logger.Warn(
			"cannot unmarshal metadata from endpoint",
			slog.String("scope", string(scope)),
			slog.String("alliance_kind", c.AllianceKind),
			slog.String("endpoint", c.getEndpoint(scope)),
			slog.String("name", c.Name),
			log.Err(err),
		)
		c.setMetricMetadataEndpointError(d, scope, 1)
		return err
	}
	return nil
}

func (c *FederationCrdInfo) updatePortProtocols(services *[]eeCrd.FederationPublicServices, defaultProtocol string, protocolMap map[string]string) {
	keys := make([]string, 0, len(protocolMap))
	for key := range protocolMap {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int { return len(b) - len(a) })
	for serviceIndex := range *services {
		service := &(*services)[serviceIndex]
		for portIndex, port := range service.Ports {
			port.Protocol = defaultProtocol
			portNameParts := strings.SplitN(port.Name, "-", 2)
			basePortName := portNameParts[0]
			for _, keyword := range keys {
				protocol := protocolMap[keyword]
				if strings.Contains(basePortName, keyword) {
					port.Protocol = protocol
					break
				}
			}
			service.Ports[portIndex] = port
		}
	}
}

func (c *CommonInfo) getEndpoint(scope discoveryScope) string {
	switch scope {
	case scopePublic:
		return c.PublicMetadataEndpoint
	case scopePrivate:
		return c.PrivateMetadataEndpoint
	}
	return ""
}
func (c *CommonInfo) setMetricMetadataEndpointError(d *Discovery, scope discoveryScope, isError float64) {
	labels := map[string]string{
		"scope":         string(scope),
		"alliance_kind": c.AllianceKind,
		"name":          c.Name,
		"endpoint":      c.getEndpoint(scope),
	}

	d.input.MetricsCollector.Set(string(d.metricsName), isError, labels, metrics.WithGroup(string(d.metricsGroup)))
}

func (c CommonInfo) prepareHTTPOptions() []http.Option {
	var httpOption []http.Option
	if c.ClusterCA != "" {
		caCerts := [][]byte{[]byte(c.ClusterCA)}
		httpOption = append(httpOption, http.WithAdditionalCACerts(caCerts))
	} else if c.EnableInsecureConnection {
		httpOption = append(httpOption, http.WithInsecureSkipVerify())
	}
	return httpOption
}

func (d *Discovery) getJWTClaims(c *CommonInfo) (map[string]string, error) {
	if c.publicMetadata == nil {
		return nil, fmt.Errorf("public metadata is not set")
	}
	if c.publicMetadata.ClusterUUID == "" {
		return nil, fmt.Errorf("cluster UUID of remote cluster is not set in metadata")
	}
	return map[string]string{
		"iss":   "d8-istio",
		"aud":   c.publicMetadata.ClusterUUID,
		"sub":   d.input.Values.Get("global.discovery.clusterUUID").String(),
		"scope": "private-" + string(d.Type), // private-multicluster|federation
	}, nil
}

func (d *Discovery) generateBearerToken(c *CommonInfo, scope discoveryScope) (string, error) {
	if scope == scopePublic {
		return "", nil
	} else {
		claims, err := d.getJWTClaims(c)
		if err != nil {
			return "", err
		}
		bearerToken, err := jwt.GenerateJWT(d.getAuthKeypairBytes(), claims, time.Minute)
		if err != nil {
			return "", err
		}
		return bearerToken, nil
	}
}

func (d *Discovery) getAuthKeypair() gjson.Result {
	return d.input.Values.Get("istio.internal.remoteAuthnKeypair.priv")
}

func (d *Discovery) getAuthKeypairBytes() []byte {
	return []byte(d.getAuthKeypair().String())
}

func (d *Discovery) hasValidType() bool {
	switch d.Type {
	case TypeMulticluster, TypeFederation:
		return true
	default:
		d.input.Logger.Warn(
			"Discovery value of DiscoveryType is invalid",
			slog.String("discovery_type", string(d.Type)),
		)
		return false
	}
}

func (d *Discovery) isReady() bool {
	if !d.getAuthKeypair().Exists() {
		d.input.Logger.Warn("authn keypair for signing requests to remote metadata endpoints isn't generated yet, retry in 1min")
		return false
	}
	return true
}

func (d *Discovery) isEnabled() bool {
	switch d.Type {
	case TypeMulticluster:
		if d.input.Values.Get("istio.multicluster.enabled").Bool() {
			return true
		}
	case TypeFederation:
		if d.input.Values.Get("istio.federation.enabled").Bool() {
			return true
		}
	}

	return false
}

func (d *Discovery) isTypedAs(dt DiscoveryType) bool {
	return d.Type == dt
}
