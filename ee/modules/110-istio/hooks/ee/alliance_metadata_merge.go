/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/square/go-jose/v3"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type IstioFederationMergeCrdInfo struct {
	ClusterUUID              string                            `json:"clusterUUID"`
	EnableInsecureConnection bool                              `json:"insecureSkipVerify"`
	IngressGateways          *[]eeCrd.FederationIngressGateway `json:"ingressGateways"`
	Name                     string                            `json:"name"`
	Public                   *eeCrd.AlliancePublicMetadata     `json:"public,omitempty"`
	PublicServices           *[]eeCrd.FederationPublicService  `json:"publicServices"`
	RootCA                   string                            `json:"rootCA"`
	SpiffeEndpoint           string                            `json:"spiffeEndpoint"`
	TrustDomain              string                            `json:"trustDomain"`
}

type IstioMulticlusterMergeCrdInfo struct {
	APIHost                  string                               `json:"apiHost"`
	APIJWT                   string                               `json:"apiJWT"`
	ClusterUUID              string                               `json:"clusterUUID"`
	EnableIngressGateway     bool                                 `json:"enableIngressGateway"`
	EnableInsecureConnection bool                                 `json:"insecureSkipVerify"`
	IngressGateways          *[]eeCrd.MulticlusterIngressGateways `json:"ingressGateways"`
	MetadataExporterCA       string                               `json:"metadataExporterCA"`
	Name                     string                               `json:"name"`
	NetworkName              string                               `json:"networkName"`
	Public                   *eeCrd.AlliancePublicMetadata        `json:"public,omitempty"`
	RootCA                   string                               `json:"rootCA"`
	SpiffeEndpoint           string                               `json:"spiffeEndpoint"`
}

type ServiceEntry struct {
	Name       string                              `json:"name"`
	Hostname   string                              `json:"hostname"`
	Resolution string                              `json:"resolution"`
	Ports      []eeCrd.FederationPublicServicePort `json:"ports"`
	Endpoints  []eeCrd.FederationIngressGateway    `json:"endpoints"`
}

func federationServiceEntryResolution(endpoints []eeCrd.FederationIngressGateway) string {
	for _, ep := range endpoints {
		if strings.TrimSpace(ep.Address) == "" {
			return "DNS"
		}
		if net.ParseIP(ep.Address) == nil {
			return "DNS"
		}
	}
	return "STATIC"
}

func sortedEndpointsKey(endpoints []eeCrd.FederationIngressGateway) string {
	sorted := make([]eeCrd.FederationIngressGateway, len(endpoints))
	copy(sorted, endpoints)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Address != sorted[j].Address {
			return sorted[i].Address < sorted[j].Address
		}
		return sorted[i].Port < sorted[j].Port
	})
	parts := make([]string, len(sorted))
	for i, ep := range sorted {
		parts[i] = fmt.Sprintf("%s:%d", ep.Address, ep.Port)
	}
	return strings.Join(parts, ",")
}

const safeChars = "bcdfghjklmnpqrstvwxz2456789"

func safeEncodeString(s string) string {
	r := make([]byte, len(s))
	for i, b := range []byte(s) {
		r[i] = safeChars[int(b)%len(safeChars)]
	}
	return string(r)
}

func serviceEntryName(hostname string, endpoints []eeCrd.FederationIngressGateway) string {
	h := fnv.New32a()
	h.Write([]byte(sortedEndpointsKey(endpoints)))
	return strings.ReplaceAll(hostname, ".", "-") + "-" + safeEncodeString(fmt.Sprint(h.Sum32()))
}

func applyFederationMergeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var federation eeCrd.IstioFederation
	err := sdk.FromUnstructured(obj, &federation)
	if err != nil {
		return nil, err
	}

	me := federation.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	var (
		igs    *[]eeCrd.FederationIngressGateway
		pss    *[]eeCrd.FederationPublicService
		p      *eeCrd.AlliancePublicMetadata
		uuid   string
		rootCA string
	)

	if federation.Status.MetadataCache.Private != nil {
		if federation.Status.MetadataCache.Private.IngressGateways != nil {
			igs = federation.Status.MetadataCache.Private.IngressGateways
		}
		if federation.Status.MetadataCache.Private.PublicServices != nil {
			pss = federation.Status.MetadataCache.Private.PublicServices
		}
	}
	if federation.Status.MetadataCache.Public != nil {
		p = federation.Status.MetadataCache.Public
		uuid = federation.Status.MetadataCache.Public.ClusterUUID
		rootCA = federation.Status.MetadataCache.Public.RootCA
	}

	return IstioFederationMergeCrdInfo{
		ClusterUUID:              uuid,
		EnableInsecureConnection: federation.Spec.Metadata.EnableInsecureConnection,
		IngressGateways:          igs,
		Name:                     federation.GetName(),
		Public:                   p,
		PublicServices:           pss,
		RootCA:                   rootCA,
		SpiffeEndpoint:           me + "/public/spiffe-bundle-endpoint",
		TrustDomain:              federation.Spec.TrustDomain,
	}, nil
}

func applyMulticlusterMergeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var multicluster eeCrd.IstioMulticluster

	err := sdk.FromUnstructured(obj, &multicluster)
	if err != nil {
		return nil, err
	}

	me := multicluster.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	var (
		igs         *[]eeCrd.MulticlusterIngressGateways
		apiHost     string
		networkName string
		p           *eeCrd.AlliancePublicMetadata
		uuid        string
		rootCA      string
	)

	if multicluster.Status.MetadataCache.Private != nil {
		if multicluster.Status.MetadataCache.Private.IngressGateways != nil {
			igs = multicluster.Status.MetadataCache.Private.IngressGateways
		}
		apiHost = multicluster.Status.MetadataCache.Private.APIHost
		networkName = multicluster.Status.MetadataCache.Private.NetworkName
	}
	if multicluster.Status.MetadataCache.Public != nil {
		p = multicluster.Status.MetadataCache.Public
		uuid = multicluster.Status.MetadataCache.Public.ClusterUUID
		rootCA = multicluster.Status.MetadataCache.Public.RootCA
	}

	return IstioMulticlusterMergeCrdInfo{
		APIHost:                  apiHost,
		ClusterUUID:              uuid,
		EnableIngressGateway:     multicluster.Spec.EnableIngressGateway,
		EnableInsecureConnection: multicluster.Spec.Metadata.EnableInsecureConnection,
		IngressGateways:          igs,
		MetadataExporterCA:       multicluster.Spec.Metadata.CA,
		Name:                     multicluster.GetName(),
		NetworkName:              networkName,
		Public:                   p,
		RootCA:                   rootCA,
		SpiffeEndpoint:           me + "/public/spiffe-bundle-endpoint",
	}, nil
}

// Simplified struct for storing only essential token data
type IstioRemoteSecretToken struct {
	MultiClusterName string `json:"multiClusterName"`
	Token            string `json:"token"`
}

// Kubeconfig represents the structure of a kubeconfig file
type Kubeconfig struct {
	Users []struct {
		User struct {
			Token string `yaml:"token"`
		} `yaml:"user"`
	} `yaml:"users"`
}

// TokenValidationResult represents the result of token validation
type TokenValidationResult struct {
	NeedReissue bool      `json:"needReissue"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Error       string    `json:"error,omitempty"`
}

// expiresSoonThreshold defines the minimum time until token expiration to consider it valid.
// If token expires sooner, it will be proactively reissued (hook runs once a month).
const expiresSoonThreshold = 30 * 24 * time.Hour

// validateJWTToken validates a JWT token and checks if it's expired or expires soon
func validateJWTToken(tokenString string) TokenValidationResult {
	if tokenString == "" {
		return TokenValidationResult{
			NeedReissue: true,
			Error:       "token is empty",
		}
	}

	// Parse the JWT token
	token, err := jose.ParseSigned(tokenString)
	if err != nil {
		return TokenValidationResult{
			NeedReissue: true,
			Error:       fmt.Sprintf("failed to parse token: %v", err),
		}
	}

	// Check expiration time
	payload := token.UnsafePayloadWithoutVerification()

	// Parse the payload to get claims
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return TokenValidationResult{
			NeedReissue: true,
			Error:       fmt.Sprintf("failed to unmarshal claims: %v", err),
		}
	}

	expTime := int64(claims["exp"].(float64))
	expiresAt := time.Unix(expTime, 0)

	// Reissue if expired OR expires in less than expiresSoonThreshold.
	// Hook runs once a month, so we need at least ~30 days buffer to avoid gaps.
	now := time.Now().UTC().Unix()
	needReissue := expTime < now || time.Until(expiresAt) < expiresSoonThreshold

	var errMsg string
	if expTime < now {
		errMsg = "JWT token expired"
	}

	return TokenValidationResult{
		NeedReissue: needReissue,
		ExpiresAt:   expiresAt,
		Error:       errMsg,
	}
}

func applyIstioRemoteSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert k8s secret to struct: %v", err)
	}

	secretName := secret.GetName()
	if !strings.HasPrefix(secretName, "istio-remote-secret-") {
		return nil, fmt.Errorf("secret %s is not an istio remote secret", secretName)
	}

	// Extract cluster name from annotation instead of parsing from secret name
	annotations := secret.GetAnnotations()
	clusterName, exists := annotations["networking.istio.io/cluster"]
	if !exists {
		return nil, fmt.Errorf("secret %s does not have required annotation 'networking.istio.io/cluster'", secretName)
	}

	// Get the base64-encoded kubeconfig from the field named after the cluster
	secData, exists := secret.Data[clusterName]
	if !exists {
		return nil, fmt.Errorf("secret %s does not contain '%s' field", secretName, clusterName)
	}

	var kubeconfigBytes []byte

	// parse the data directly as YAML
	var testKubeconfig Kubeconfig
	if yaml.Unmarshal(secData, &testKubeconfig) == nil {
		kubeconfigBytes = secData
	} else {
		// If direct YAML parsing failed, try base64 decoding
		// Remove all whitespace characters first
		cleanBase64 := strings.Map(func(r rune) rune {
			if strings.ContainsRune(" \t\n\r\v\f", r) {
				return -1
			}
			return r
		}, string(secData))

		var decodeErr error
		kubeconfigBytes, decodeErr = base64.StdEncoding.DecodeString(cleanBase64)
		if decodeErr != nil {
			return nil, fmt.Errorf("cannot decode base64 kubeconfig from secret %s: %v (also tried direct YAML parsing)", secretName, decodeErr)
		}
	}

	// Extract token from kubeconfig directly
	var kubeconfig Kubeconfig
	err = yaml.Unmarshal(kubeconfigBytes, &kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig from secret %s: %v", secretName, err)
	}

	// Look for the first user with a token
	var token string
	for _, user := range kubeconfig.Users {
		if user.User.Token != "" {
			token = user.User.Token
			break
		}
	}

	if token == "" {
		return nil, fmt.Errorf("token not found in kubeconfig from secret %s", secretName)
	}

	return IstioRemoteSecretToken{
		MultiClusterName: clusterName,
		Token:            token,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("alliance"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "federations",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IstioFederation",
			FilterFunc: applyFederationMergeFilter,
		},
		{
			Name:       "multiclusters",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IstioMulticlusters",
			FilterFunc: applyMulticlusterMergeFilter,
		},
		{
			Name:       "istioRemoteSecrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyIstioRemoteSecretFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"istio/multiCluster": "true",
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		// until the bug won't be solved https://github.com/istio/istio/issues/37925
		// {Name: "cron", Crontab: "0 3 * * *"}, // once a day to refresh apiJWT
		{Name: "cron", Crontab: "0 3 1 * *"}, // once a month to refresh apiJWT
	},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, metadataMerge)

func metadataMerge(_ context.Context, input *go_hook.HookInput) error {
	var properFederations = make([]IstioFederationMergeCrdInfo, 0)
	var properMulticlusters = make([]IstioMulticlusterMergeCrdInfo, 0)
	var multiclustersNeedIngressGateway = false
	//                              map[clusterUUID]public
	var remotePublicMetadata = make(map[string]eeCrd.AlliancePublicMetadata)

	var myTrustDomain = input.Values.Get("global.discovery.clusterDomain").String()

	// Create a map of cluster names to tokens from remote secrets for quick lookup
	secretTokens := make(map[string]string)
	for secretInfo := range sdkobjectpatch.SnapshotIter[IstioRemoteSecretToken](input.Snapshots.Get("istioRemoteSecrets")) {
		secretTokens[secretInfo.MultiClusterName] = secretInfo.Token
	}

federationsLoop:
	for federationInfo, err := range sdkobjectpatch.SnapshotIter[IstioFederationMergeCrdInfo](input.Snapshots.Get("federations")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over federations: %w", err)
		}

		if federationInfo.TrustDomain == myTrustDomain {
			input.Logger.Warn("skipping IstioFederation with trustDomain equals to ours", slog.String("name", federationInfo.Name), slog.String("trust_domain", federationInfo.TrustDomain))
			continue federationsLoop
		}
		if federationInfo.Public == nil {
			input.Logger.Warn("public metadata for IstioFederation wasn't fetched yet", slog.String("name", federationInfo.Name))
			continue federationsLoop
		}
		// federationInfo.Public.AllianceRef = &eeCrd.PublicMetadataAllianceRef{
		// 	Kind: "IstioFederation",
		// 	Name: federationInfo.Name,
		// }
		remotePublicMetadata[federationInfo.Public.ClusterUUID] = *federationInfo.Public

		if federationInfo.PublicServices == nil {
			input.Logger.Warn("private metadata for IstioFederation wasn't fetched yet", slog.String("name", federationInfo.Name))
			continue
		}

		if federationInfo.IngressGateways == nil || len(*federationInfo.IngressGateways) == 0 {
			input.Logger.Warn("private metadata for IstioFederation wasn't fetched yet", slog.String("name", federationInfo.Name))
			continue federationsLoop
		}

		federationInfo.Public = nil
		properFederations = append(properFederations, federationInfo)
	}

	// Build ServiceEntries by merging endpoints across federations for each
	// (hostname, port) pair, then grouping ports that share the same hostname
	// and endpoint set into a single ServiceEntry.
	// Port collisions (same number, different name/protocol) are first-wins with a warning.

	portDefs := make(map[string]map[uint]eeCrd.FederationPublicServicePort)
	portEndpoints := make(map[string]map[uint]map[eeCrd.FederationIngressGateway]struct{})

	for _, fed := range properFederations {
		if fed.PublicServices == nil || fed.IngressGateways == nil {
			continue
		}

		seenInFed := make(map[string]map[uint]struct{}) // hostname -> set of port numbers
		for _, ps := range *fed.PublicServices {
			if seenInFed[ps.Hostname] == nil {
				seenInFed[ps.Hostname] = make(map[uint]struct{})
			}
			if portDefs[ps.Hostname] == nil {
				portDefs[ps.Hostname] = make(map[uint]eeCrd.FederationPublicServicePort)
				portEndpoints[ps.Hostname] = make(map[uint]map[eeCrd.FederationIngressGateway]struct{})
			}

			for _, port := range ps.Ports {
				if _, dup := seenInFed[ps.Hostname][port.Port]; dup {
					continue
				}
				seenInFed[ps.Hostname][port.Port] = struct{}{}

				if existing, ok := portDefs[ps.Hostname][port.Port]; !ok {
					portDefs[ps.Hostname][port.Port] = port
					eps := make(map[eeCrd.FederationIngressGateway]struct{}, len(*fed.IngressGateways))
					for _, ig := range *fed.IngressGateways {
						eps[ig] = struct{}{}
					}
					portEndpoints[ps.Hostname][port.Port] = eps
				} else {
					if existing.Name != port.Name || existing.Protocol != port.Protocol {
						input.Logger.Warn("port collision: same hostname and port number with different name/protocol across federations, keeping first definition",
							slog.String("federation", fed.Name),
							slog.String("hostname", ps.Hostname),
							slog.Uint64("port", uint64(port.Port)),
							slog.String("existing_name", existing.Name),
							slog.String("existing_protocol", existing.Protocol),
							slog.String("conflicting_name", port.Name),
							slog.String("conflicting_protocol", port.Protocol),
						)
					}
					for _, ig := range *fed.IngressGateways {
						portEndpoints[ps.Hostname][port.Port][ig] = struct{}{}
					}
				}
			}
		}
	}

	// Group ports sharing the same hostname and endpoint set into ServiceEntries.
	// hostname -> endpointSetKey -> *ServiceEntry

	serviceEntriesByHost := make(map[string]map[string]*ServiceEntry)
	for hostname, ports := range portDefs {
		for portNum, port := range ports {
			endpoints := make([]eeCrd.FederationIngressGateway, 0, len(portEndpoints[hostname][portNum]))
			for ep := range portEndpoints[hostname][portNum] {
				endpoints = append(endpoints, ep)
			}
			epKey := sortedEndpointsKey(endpoints)

			byEpKey := serviceEntriesByHost[hostname]
			if byEpKey == nil {
				byEpKey = make(map[string]*ServiceEntry)
				serviceEntriesByHost[hostname] = byEpKey
			}

			if se, ok := byEpKey[epKey]; ok {
				se.Ports = append(se.Ports, port)
			} else {
				byEpKey[epKey] = &ServiceEntry{
					Hostname:  hostname,
					Ports:     []eeCrd.FederationPublicServicePort{port},
					Endpoints: endpoints,
				}
			}
		}
	}

	serviceEntries := make([]ServiceEntry, 0)
	for _, byEpKey := range serviceEntriesByHost {
		for _, se := range byEpKey {
			sort.Slice(se.Ports, func(i, j int) bool {
				return se.Ports[i].Port < se.Ports[j].Port
			})
			sort.Slice(se.Endpoints, func(i, j int) bool {
				if se.Endpoints[i].Address != se.Endpoints[j].Address {
					return se.Endpoints[i].Address < se.Endpoints[j].Address
				}
				return se.Endpoints[i].Port < se.Endpoints[j].Port
			})
			se.Name = serviceEntryName(se.Hostname, se.Endpoints)
			se.Resolution = federationServiceEntryResolution(se.Endpoints)
			serviceEntries = append(serviceEntries, *se)
		}
	}
	sort.Slice(serviceEntries, func(i, j int) bool {
		if serviceEntries[i].Hostname != serviceEntries[j].Hostname {
			return serviceEntries[i].Hostname < serviceEntries[j].Hostname
		}
		return serviceEntries[i].Ports[0].Port < serviceEntries[j].Ports[0].Port
	})

multiclustersLoop:
	for multiclusterInfo, err := range sdkobjectpatch.SnapshotIter[IstioMulticlusterMergeCrdInfo](input.Snapshots.Get("multiclusters")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over multiclusters: %w", err)
		}

		if multiclusterInfo.EnableIngressGateway {
			multiclustersNeedIngressGateway = true
		}

		if multiclusterInfo.Public == nil {
			input.Logger.Warn("public metadata for IstioMulticluster wasn't fetched yet", slog.String("name", multiclusterInfo.Name))
			continue multiclustersLoop
		}
		// multiclusterInfo.Public.AllianceRef = &eeCrd.PublicMetadataAllianceRef{
		// 	Kind: "IstioMulticluster",
		// 	Name: multiclusterInfo.Name,
		// }
		remotePublicMetadata[multiclusterInfo.Public.ClusterUUID] = *multiclusterInfo.Public

		if multiclusterInfo.APIHost == "" || multiclusterInfo.NetworkName == "" {
			input.Logger.Warn("private metadata for IstioMulticluster wasn't fetched yet", slog.String("name", multiclusterInfo.Name))
			continue multiclustersLoop
		}
		if multiclusterInfo.EnableIngressGateway &&
			(multiclusterInfo.IngressGateways == nil || len(*multiclusterInfo.IngressGateways) == 0) {
			input.Logger.Warn("ingressGateways for IstioMulticluster weren't fetched yet", slog.String("name", multiclusterInfo.Name))
			continue multiclustersLoop
		}

		// Check existing token from remote secrets and validate it
		existingToken := secretTokens[multiclusterInfo.Name]

		input.Logger.Info("validating existing token",
			slog.String("name", multiclusterInfo.Name))

		validationResult := validateJWTToken(existingToken)
		input.Logger.Info("token validation result",
			slog.String("name", multiclusterInfo.Name),
			slog.Bool("needReissue", validationResult.NeedReissue),
			slog.String("error", validationResult.Error),
			slog.String("expiresAt", validationResult.ExpiresAt.Format(time.RFC3339)))

		if !validationResult.NeedReissue {
			multiclusterInfo.APIJWT = existingToken
			input.Logger.Info("reusing existing valid token for multicluster",
				slog.String("name", multiclusterInfo.Name),
				slog.String("expiresAt", validationResult.ExpiresAt.Format(time.RFC3339)))
		} else {
			reason := validationResult.Error
			if reason == "" {
				reason = "expires in less than 30 days (proactive refresh)"
			}
			input.Logger.Info("regenerating token for multicluster",
				slog.String("name", multiclusterInfo.Name),
				slog.String("reason", reason))

			privKey := []byte(input.Values.Get("istio.internal.remoteAuthnKeypair.priv").String())
			claims := map[string]string{
				"iss":   "d8-istio",
				"aud":   multiclusterInfo.ClusterUUID,
				"sub":   input.Values.Get("global.discovery.clusterUUID").String(),
				"scope": "api",
			}
			// until the bug won't be solved https://github.com/istio/istio/issues/37925
			// multiclusterInfo.APIJWT, err = jwt.GenerateJWT(privKey, claims, time.Hour*25)
			multiclusterInfo.APIJWT, err = jwt.GenerateJWT(privKey, claims, time.Hour*24*366)
			if err != nil {
				input.Logger.Warn("can't generate auth token for remote api of IstioMulticluster", slog.String("name", multiclusterInfo.Name), log.Err(err))
				continue multiclustersLoop
			}
		}

		multiclusterInfo.Public = nil
		properMulticlusters = append(properMulticlusters, multiclusterInfo)
	}

	input.Values.Set("istio.internal.federations", properFederations)
	input.Values.Set("istio.internal.federationServiceEntries", serviceEntries)
	input.Values.Set("istio.internal.multiclusters", properMulticlusters)
	input.Values.Set("istio.internal.multiclustersNeedIngressGateway", multiclustersNeedIngressGateway)
	input.Values.Set("istio.internal.remotePublicMetadata", remotePublicMetadata)

	return nil
}
