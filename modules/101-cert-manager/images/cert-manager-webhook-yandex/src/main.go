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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	capi "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/dns/v1"
	dnssdk "github.com/yandex-cloud/go-sdk/services/dns/v1"
	ycsdk "github.com/yandex-cloud/go-sdk/v2"
	"github.com/yandex-cloud/go-sdk/v2/credentials"
	"github.com/yandex-cloud/go-sdk/v2/pkg/iamkey"
	"github.com/yandex-cloud/go-sdk/v2/pkg/options"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultAPIEndpoint = "api.cloud.yandex.net:443"
	apiRequestTimeout  = 2 * time.Minute
)

func main() {
	if GroupName := os.Getenv("GROUP_NAME"); GroupName == "" {
		log.Fatal("GROUP_NAME env must be specified")
	} else {
		// This will register our yandex DNS provider with the webhook serving
		// library, making it available as an API under the provided GroupName.
		// You can register multiple DNS provider implementations with a single
		// webhook, where the Name() method will be used to disambiguate between
		// the different implementations.
		cmd.RunWebhookServer(GroupName,
			&yandexCloudDNSSolver{
				apiEndpoint: os.Getenv("API_ENDPOINT"),
			},
		)
	}
}

// yandexCloudDNSSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type yandexCloudDNSSolver struct {
	apiEndpoint string
	client      *kubernetes.Clientset
}

// yandexCloudDNSConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type yandexCloudDNSConfig struct {
	Folder            string                 `json:"folder"`
	ServiceAccountKey capi.SecretKeySelector `json:"serviceAccountSecretRef"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *yandexCloudDNSSolver) Name() string {
	return "yandex-cloud-dns"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *yandexCloudDNSSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), apiRequestTimeout)
	defer cancel()

	dnsClient, folder, err := c.buildDNSClient(ctx, ch)
	if err != nil {
		return err
	}

	zone, err := getDNSZone(ctx, dnsClient, folder, ch.ResolvedZone)
	if err != nil {
		return err
	}

	record := dns.RecordSet{
		Name: ch.ResolvedFQDN,
		Data: []string{ch.Key},
		Ttl:  int64(60),
		Type: "TXT",
	}

	reqUpd := dns.UpsertRecordSetsRequest{
		DnsZoneId: zone.Id,
		Merges:    []*dns.RecordSet{&record},
	}

	op, err := dnsClient.UpsertRecordSets(ctx, &reqUpd)
	if err != nil {
		return err
	}

	if _, err := op.Wait(ctx); err != nil {
		return errors.Wrap(err, "waiting for UpsertRecordSets (present)")
	}

	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *yandexCloudDNSSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), apiRequestTimeout)
	defer cancel()

	// Always rebuild the client: CleanUp may run on another replica or after a
	// restart, so it must not rely on Present having initialized shared state.
	dnsClient, folder, err := c.buildDNSClient(ctx, ch)
	if err != nil {
		return err
	}

	zone, err := getDNSZone(ctx, dnsClient, folder, ch.ResolvedZone)
	if err != nil {
		return err
	}

	record := dns.RecordSet{
		Name: ch.ResolvedFQDN,
		Data: []string{ch.Key},
		Ttl:  int64(60),
		Type: "TXT",
	}

	reqDel := dns.UpsertRecordSetsRequest{
		DnsZoneId: zone.Id,
		Deletions: []*dns.RecordSet{&record},
	}

	op, err := dnsClient.UpsertRecordSets(ctx, &reqDel)
	if err != nil {
		return err
	}

	if _, err := op.Wait(ctx); err != nil {
		return errors.Wrap(err, "waiting for UpsertRecordSets (cleanup)")
	}

	return nil
}

func getDNSZone(ctx context.Context, dnsClient dnssdk.DnsZoneClient, folder, resolvedZone string) (*dns.DnsZone, error) {
	// cert-manager already resolved the authoritative zone (SOA recursion) into
	// ChallengeRequest.ResolvedZone. Do not re-query public NS from the webhook.
	zoneName := normalizeZone(resolvedZone)
	if zoneName == "" {
		return nil, errors.New("resolved zone is empty")
	}

	pageToken := ""
	for {
		req := dns.ListDnsZonesRequest{
			FolderId:  folder,
			Filter:    `zone = "` + zoneName + `"`,
			PageSize:  1000,
			PageToken: pageToken,
		}
		resp, err := dnsClient.List(ctx, &req)
		if err != nil {
			return nil, err
		}

		for _, dnsZone := range resp.DnsZones {
			if isPublicDNSZone(dnsZone) && normalizeZone(dnsZone.Zone) == zoneName {
				return dnsZone, nil
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return nil, errors.Errorf("no public zone %s found", zoneName)
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *yandexCloudDNSSolver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl

	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (yandexCloudDNSConfig, error) {
	cfg := yandexCloudDNSConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func (c *yandexCloudDNSSolver) buildDNSClient(ctx context.Context, ch *v1alpha1.ChallengeRequest) (dnssdk.DnsZoneClient, string, error) {
	apiEndpoint := defaultAPIEndpoint
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return nil, "", err
	}
	if cfg.Folder == "" {
		return nil, "", errors.New("folder must be specified in solver config")
	}

	saBytes, err := c.loadSecretData(ctx, cfg.ServiceAccountKey, ch.ResourceNamespace)
	if err != nil {
		return nil, "", err
	}

	key := &iamkey.Key{}
	err = key.UnmarshalJSON(saBytes)
	if err != nil {
		return nil, "", err
	}

	saKey, err := credentials.ServiceAccountKey(key)
	if err != nil {
		return nil, "", err
	}

	if c.apiEndpoint != "" {
		apiEndpoint = c.apiEndpoint
	}

	sdk, err := ycsdk.Build(ctx, options.WithCredentials(saKey), options.WithDiscoveryEndpoint(apiEndpoint))
	if err != nil {
		return nil, "", err
	}

	return dnssdk.NewDnsZoneClient(sdk), cfg.Folder, nil
}

func (c *yandexCloudDNSSolver) loadSecretData(ctx context.Context, selector capi.SecretKeySelector, ns string) ([]byte, error) {
	secret, err := c.client.CoreV1().Secrets(ns).Get(ctx, selector.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load secret %q", ns+"/"+selector.Name)
	}

	data, ok := secret.Data[selector.Key]
	if !ok {
		return nil, errors.Errorf("no key %q in secret %q", selector.Key, ns+"/"+selector.Name)
	}

	return data, nil
}

func normalizeZone(zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return ""
	}
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}
	return zone
}

func isPublicDNSZone(dnsZone *dns.DnsZone) bool {
	return dnsZone.PublicVisibility != nil
}
