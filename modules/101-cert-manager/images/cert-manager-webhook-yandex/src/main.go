package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	capi "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/cert-manager/cert-manager/pkg/issuer/acme/dns/util"
	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/dns/v1"
	dnssdk "github.com/yandex-cloud/go-sdk/services/dns/v1"
	ycsdk "github.com/yandex-cloud/go-sdk/v2"
	"github.com/yandex-cloud/go-sdk/v2/credentials"
	"github.com/yandex-cloud/go-sdk/v2/pkg/iamkey"
	"github.com/yandex-cloud/go-sdk/v2/pkg/options"
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
	apiEndpoint   string
	client        *kubernetes.Clientset
	dnsZoneClient dnssdk.DnsZoneClient
	folder        string
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
	err := c.setConfig(ch)
	if err != nil {
		return err
	}

	zone, err := c.getDNSZone(ch.ResolvedZone)
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

	op, err := c.dnsZoneClient.UpsertRecordSets(context.Background(), &reqUpd)
	if err != nil {
		return err
	}

	for !op.Done() {
		time.Sleep(time.Second)
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
	zone, err := c.getDNSZone(ch.ResolvedZone)
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

	op, err := c.dnsZoneClient.UpsertRecordSets(context.Background(), &reqDel)
	if err != nil {
		return err
	}

	for !op.Done() {
		time.Sleep(time.Second)
	}

	return nil
}

func (c *yandexCloudDNSSolver) getDNSZone(zone string) (*dns.DnsZone, error) {
	resolvedZone, err := util.FindZoneByFqdn(context.Background(), zone, util.RecursiveNameservers)
	if err != nil {
		return nil, err
	}

	req := dns.ListDnsZonesRequest{
		FolderId: c.folder,
		Filter:   `zone = "` + resolvedZone + `"`,
	}
	resp, err := c.dnsZoneClient.List(context.Background(), &req)
	if err != nil {
		return nil, err
	}

	for _, dnsZone := range resp.DnsZones {
		if isPublicDnsZone(dnsZone) {
			return dnsZone, nil
		}
	}

	return nil, errors.Errorf("no public zone %s found", zone)
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

func (c *yandexCloudDNSSolver) setConfig(ch *v1alpha1.ChallengeRequest) error {
	apiEndpoint := "api.cloud.yandex.net:443"
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	saBytes, err := c.loadSecretData(cfg.ServiceAccountKey, ch.ResourceNamespace)
	if err != nil {
		return err
	}

	key := &iamkey.Key{}
	err = key.UnmarshalJSON(saBytes)
	if err != nil {
		return err
	}

	saKey, err := credentials.ServiceAccountKey(key)
	if err != nil {
		return err
	}

	if c.apiEndpoint != "" {
		apiEndpoint = c.apiEndpoint
	}

	sdk, err := ycsdk.Build(context.Background(), options.WithCredentials(saKey), options.WithDiscoveryEndpoint(apiEndpoint))
	if err != nil {
		return err
	}

	c.dnsZoneClient = dnssdk.NewDnsZoneClient(sdk)
	c.folder = cfg.Folder

	return nil
}

func (c *yandexCloudDNSSolver) loadSecretData(selector capi.SecretKeySelector, ns string) ([]byte, error) {
	secret, err := c.client.CoreV1().Secrets(ns).Get(context.TODO(), selector.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load secret %q", ns+"/"+selector.Name)
	}

	data, ok := secret.Data[selector.Key]
	if !ok {
		return nil, errors.Errorf("no key %q in secret %q", selector.Key, ns+"/"+selector.Name)
	}

	return data, nil
}

func isPublicDnsZone(dnsZone *dns.DnsZone) bool {
	return dnsZone.PublicVisibility != nil
}
