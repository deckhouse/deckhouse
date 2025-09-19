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
	"log/slog"
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
	Name             string                             `json:"name"`
	TrustDomain      string                             `json:"trustDomain"`
	SpiffeEndpoint   string                             `json:"spiffeEndpoint"`
	IngressGateways  *[]eeCrd.FederationIngressGateways `json:"ingressGateways"`
	MetadataCA       string                             `json:"ca"`
	MetadataInsecure bool                               `json:"insecureSkipVerify"`
	PublicServices   *[]eeCrd.FederationPublicServices  `json:"publicServices"`
	Public           *eeCrd.AlliancePublicMetadata      `json:"public,omitempty"`
}

type IstioMulticlusterMergeCrdInfo struct {
	Name                 string                               `json:"name"`
	SpiffeEndpoint       string                               `json:"spiffeEndpoint"`
	EnableIngressGateway bool                                 `json:"enableIngressGateway"`
	MetadataCA           string                               `json:"ca"`
	MetadataInsecure     bool                                 `json:"insecureSkipVerify"`
	APIHost              string                               `json:"apiHost"`
	NetworkName          string                               `json:"networkName"`
	APIJWT               string                               `json:"apiJWT"`
	IngressGateways      *[]eeCrd.MulticlusterIngressGateways `json:"ingressGateways"`
	Public               *eeCrd.AlliancePublicMetadata        `json:"public,omitempty"`
}

func applyFederationMergeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var federation eeCrd.IstioFederation
	err := sdk.FromUnstructured(obj, &federation)
	if err != nil {
		return nil, err
	}

	me := federation.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	var igs *[]eeCrd.FederationIngressGateways
	var pss *[]eeCrd.FederationPublicServices
	var p *eeCrd.AlliancePublicMetadata

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
	}

	return IstioFederationMergeCrdInfo{
		Name:             federation.GetName(),
		TrustDomain:      federation.Spec.TrustDomain,
		SpiffeEndpoint:   me + "/public/spiffe-bundle-endpoint",
		IngressGateways:  igs,
		MetadataCA:       federation.Spec.Metadata.ClusterCA,
		MetadataInsecure: federation.Spec.Metadata.EnableInsecureConnection,
		PublicServices:   pss,
		Public:           p,
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

	var igs *[]eeCrd.MulticlusterIngressGateways
	var apiHost string
	var networkName string
	var p *eeCrd.AlliancePublicMetadata

	if multicluster.Status.MetadataCache.Private != nil {
		if multicluster.Status.MetadataCache.Private.IngressGateways != nil {
			igs = multicluster.Status.MetadataCache.Private.IngressGateways
		}
		apiHost = multicluster.Status.MetadataCache.Private.APIHost
		networkName = multicluster.Status.MetadataCache.Private.NetworkName
	}
	if multicluster.Status.MetadataCache.Public != nil {
		p = multicluster.Status.MetadataCache.Public
	}

	return IstioMulticlusterMergeCrdInfo{
		Name:                 multicluster.GetName(),
		SpiffeEndpoint:       me + "/public/spiffe-bundle-endpoint",
		MetadataCA:           multicluster.Spec.Metadata.ClusterCA,
		MetadataInsecure:     multicluster.Spec.Metadata.EnableInsecureConnection,
		EnableIngressGateway: multicluster.Spec.EnableIngressGateway,
		APIHost:              apiHost,
		NetworkName:          networkName,
		IngressGateways:      igs,
		Public:               p,
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
	IsExpired bool      `json:"isExpired"`
	ExpiresAt time.Time `json:"expiresAt"`
	Error     string    `json:"error,omitempty"`
}

// validateJWTToken validates a JWT token and checks if it's expired
func validateJWTToken(tokenString string) TokenValidationResult {
	if tokenString == "" {
		return TokenValidationResult{
			IsExpired: true,
			Error:     "token is empty",
		}
	}

	// Parse the JWT token
	token, err := jose.ParseSigned(tokenString)
	if err != nil {
		return TokenValidationResult{
			IsExpired: true,
			Error:     fmt.Sprintf("failed to parse token: %v", err),
		}
	}

	// Check expiration time
	payload := token.UnsafePayloadWithoutVerification()

	// Parse the payload to get claims
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return TokenValidationResult{
			IsExpired: true,
			Error:     fmt.Sprintf("failed to unmarshal claims: %v", err),
		}
	}

	expTime := int64(claims["exp"].(float64))

	if expTime < time.Now().UTC().Unix() {
		return TokenValidationResult{
			IsExpired: true,
			Error:     "JWT token expired",
			ExpiresAt: time.Unix(expTime, 0),
		}
	}

	return TokenValidationResult{
		IsExpired: false,
		ExpiresAt: time.Unix(expTime, 0),
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
			slog.Bool("isExpired", validationResult.IsExpired),
			slog.String("error", validationResult.Error),
			slog.String("expiresAt", validationResult.ExpiresAt.Format(time.RFC3339)))

		if !validationResult.IsExpired {
			// Token is still valid, reuse it
			multiclusterInfo.APIJWT = existingToken
			input.Logger.Info("reusing existing valid token for multicluster",
				slog.String("name", multiclusterInfo.Name),
				slog.String("expiresAt", validationResult.ExpiresAt.Format(time.RFC3339)))
		} else {
			input.Logger.Info("existing token is invalid or expired, generating new token",
				slog.String("name", multiclusterInfo.Name),
				slog.String("error", validationResult.Error),
				slog.Bool("isExpired", validationResult.IsExpired))

			privKey := []byte(input.Values.Get("istio.internal.remoteAuthnKeypair.priv").String())
			claims := map[string]string{
				"iss":   "d8-istio",
				"aud":   multiclusterInfo.Public.ClusterUUID,
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
	input.Values.Set("istio.internal.multiclusters", properMulticlusters)
	input.Values.Set("istio.internal.multiclustersNeedIngressGateway", multiclustersNeedIngressGateway)
	input.Values.Set("istio.internal.remotePublicMetadata", remotePublicMetadata)

	return nil
}
