/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Exporter struct {
	lwIngressGatewayServices           *cache.ListWatch
	lwIngressGatewayPods               *cache.ListWatch
	lwIngressGatewayAdvertiseConfigMap *cache.ListWatch
	lwAmbientGatewayServices           *cache.ListWatch
	lwAmbientGatewayPods               *cache.ListWatch
	lwAmbientGatewayAdvertiseConfigMap *cache.ListWatch
	lwNodes                            *cache.ListWatch
	lwCertCAConfigMap                  *cache.ListWatch
	lwPublicServices                   *cache.ListWatch // for federation
	lwRemoteClustersPublicMetadata     *cache.ListWatch
	lwRemoteAuthnKeypair               *cache.ListWatch

	ingressGatewayServiceInformer            cache.SharedInformer
	ingressGatewayPodInformer                cache.SharedInformer
	ingressGatewayAdvertiseConfigMapInformer cache.SharedInformer
	ambientGatewayServiceInformer            cache.SharedInformer
	ambientGatewayPodInformer                cache.SharedInformer
	ambientGatewayAdvertiseConfigMapInformer cache.SharedInformer
	nodeInformer                             cache.SharedInformer
	certCAConfigMapInformer                  cache.SharedInformer
	publicServiceInformer                    cache.SharedInformer // for federation
	remoteClustersPublicMetadataInformer     cache.SharedInformer
	remoteAuthnKeypairInformer               cache.SharedInformer

	ingressGatewayInlet     string
	ambientGatewayInlet     string
	clusterDomain           string
	clusterUUID             string
	multiclusterClusterID   string
	multiclusterNetworkName string
	multiclusterAPIHost     string
	federationEnabled       string
}

const (
	ingressGatewayPortName = "tls"
	ambientGatewayPortName = "hbone"

	ingressGatewayAdvertiseConfigMapName = "metadata-exporter-ingressgateway-advertise"
	ambientGatewayAdvertiseConfigMapName = "metadata-exporter-ambientgateway-advertise"

	advertisedGatewaysKey           = "gateways.json"
	deprecatedAdvertisedGatewaysKey = "ingressgateways-array.json" // drop in 1.79
)

func New(namespace string, ingressGatewayLabelSelector string, ambientGatewayLabelSelector string) (*Exporter, error) {
	// Get environments

	ingressGatewayInlet := os.Getenv("INLET")
	ambientGatewayInlet := os.Getenv("AMBIENT_INLET")
	clusterDomain := os.Getenv("CLUSTER_DOMAIN")
	clusterUUID := os.Getenv("CLUSTER_UUID")
	multiclusterClusterID := os.Getenv("MULTICLUSTER_CLUSTER_ID")
	multiclusterNetworkName := os.Getenv("MULTICLUSTER_NETWORK_NAME")
	multiclusterAPIHost := os.Getenv("MULTICLUSTER_API_HOST")
	federationEnabled := os.Getenv("FEDERATION_ENABLED")

	// Create config for Kubernetes-client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("[metadata-exporter] Error : %s", err)
	}

	// Create client Kubernetes
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("[metadata-exporter] Error : %s", err)
	}

	lwIngressGatewayServices := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"services",
		namespace,
		func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("app=%s", ingressGatewayLabelSelector)
		},
	)

	lwAmbientGatewayServices := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"services",
		namespace,
		func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("app=%s", ambientGatewayLabelSelector)
		},
	)

	lwIngressGatewayPods := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"pods",
		namespace,
		func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("app=%s", ingressGatewayLabelSelector)
		},
	)

	lwAmbientGatewayPods := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"pods",
		namespace,
		func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("app=%s", ambientGatewayLabelSelector)
		},
	)

	lwNodes := cache.NewListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"nodes",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	lwIngressGatewayAdvertiseConfigMap := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + ingressGatewayAdvertiseConfigMapName
		},
	)

	lwAmbientGatewayAdvertiseConfigMap := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + ambientGatewayAdvertiseConfigMapName
		},
	)

	lwPublicServices := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"services",
		metav1.NamespaceAll,
		func(options *metav1.ListOptions) {
			options.LabelSelector = "federation.istio.deckhouse.io/public-service="
		},
	)

	lwCertCAConfigMap := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=istio-ca-root-cert"
		},
	)

	lwRemoteClustersPublicMetadata := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"secrets",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=d8-remote-clusters-public-metadata"
		},
	)

	lwRemoteAuthnKeypair := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"secrets",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=d8-remote-authn-keypair"
		},
	)

	return &Exporter{
			lwIngressGatewayServices:           lwIngressGatewayServices,
			lwIngressGatewayPods:               lwIngressGatewayPods,
			lwIngressGatewayAdvertiseConfigMap: lwIngressGatewayAdvertiseConfigMap,
			lwAmbientGatewayServices:           lwAmbientGatewayServices,
			lwAmbientGatewayPods:               lwAmbientGatewayPods,
			lwAmbientGatewayAdvertiseConfigMap: lwAmbientGatewayAdvertiseConfigMap,
			lwNodes:                            lwNodes,
			lwCertCAConfigMap:                  lwCertCAConfigMap,
			lwPublicServices:                   lwPublicServices,
			lwRemoteClustersPublicMetadata:     lwRemoteClustersPublicMetadata,
			lwRemoteAuthnKeypair:               lwRemoteAuthnKeypair,
			ingressGatewayInlet:                ingressGatewayInlet,
			ambientGatewayInlet:                ambientGatewayInlet,
			clusterDomain:                      clusterDomain,
			clusterUUID:                        clusterUUID,
			multiclusterClusterID:              multiclusterClusterID,
			multiclusterNetworkName:            multiclusterNetworkName,
			multiclusterAPIHost:                multiclusterAPIHost,
			federationEnabled:                  federationEnabled,
		},
		nil
}

func (exp *Exporter) startInformers(ctx context.Context) {
	exp.ingressGatewayServiceInformer = cache.NewSharedInformer(
		exp.lwIngressGatewayServices,
		&v1.Service{},
		0,
	)

	exp.ambientGatewayServiceInformer = cache.NewSharedInformer(
		exp.lwAmbientGatewayServices,
		&v1.Service{},
		0,
	)

	exp.ingressGatewayPodInformer = cache.NewSharedInformer(
		exp.lwIngressGatewayPods,
		&v1.Pod{},
		0,
	)

	exp.ambientGatewayPodInformer = cache.NewSharedInformer(
		exp.lwAmbientGatewayPods,
		&v1.Pod{},
		0,
	)

	exp.nodeInformer = cache.NewSharedInformer(
		exp.lwNodes,
		&v1.Node{},
		0,
	)

	exp.ingressGatewayAdvertiseConfigMapInformer = cache.NewSharedInformer(
		exp.lwIngressGatewayAdvertiseConfigMap,
		&v1.ConfigMap{},
		0,
	)

	exp.ambientGatewayAdvertiseConfigMapInformer = cache.NewSharedInformer(
		exp.lwAmbientGatewayAdvertiseConfigMap,
		&v1.ConfigMap{},
		0,
	)

	exp.publicServiceInformer = cache.NewSharedInformer(
		exp.lwPublicServices,
		&v1.Service{},
		0,
	)

	exp.certCAConfigMapInformer = cache.NewSharedInformer(
		exp.lwCertCAConfigMap,
		&v1.ConfigMap{},
		0,
	)

	exp.remoteClustersPublicMetadataInformer = cache.NewSharedInformer(
		exp.lwRemoteClustersPublicMetadata,
		&v1.Secret{},
		0,
	)

	exp.remoteAuthnKeypairInformer = cache.NewSharedInformer(
		exp.lwRemoteAuthnKeypair,
		&v1.Secret{},
		0,
	)

	go exp.ingressGatewayServiceInformer.Run(ctx.Done())
	go exp.ambientGatewayServiceInformer.Run(ctx.Done())
	go exp.ingressGatewayPodInformer.Run(ctx.Done())
	go exp.ambientGatewayPodInformer.Run(ctx.Done())
	go exp.nodeInformer.Run(ctx.Done())
	go exp.ingressGatewayAdvertiseConfigMapInformer.Run(ctx.Done())
	go exp.ambientGatewayAdvertiseConfigMapInformer.Run(ctx.Done())
	go exp.publicServiceInformer.Run(ctx.Done())
	go exp.certCAConfigMapInformer.Run(ctx.Done())
	go exp.remoteClustersPublicMetadataInformer.Run(ctx.Done())
	go exp.remoteAuthnKeypairInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(),
		exp.ingressGatewayServiceInformer.HasSynced,
		exp.ambientGatewayServiceInformer.HasSynced,
		exp.ingressGatewayPodInformer.HasSynced,
		exp.ambientGatewayPodInformer.HasSynced,
		exp.nodeInformer.HasSynced,
		exp.ingressGatewayAdvertiseConfigMapInformer.HasSynced,
		exp.ambientGatewayAdvertiseConfigMapInformer.HasSynced,
		exp.publicServiceInformer.HasSynced,
		exp.certCAConfigMapInformer.HasSynced,
		exp.remoteClustersPublicMetadataInformer.HasSynced,
		exp.remoteAuthnKeypairInformer.HasSynced) {
		fmt.Println("[ERROR] Failed to sync caches")
		return
	}

	go exp.trackAmbientGatewayState(ctx)
}

func loadBalancerAddress(ingresses []v1.LoadBalancerIngress) string {
	for _, ingress := range ingresses {
		if ingress.IP != "" {
			return ingress.IP
		}

		if ingress.Hostname != "" {
			return ingress.Hostname
		}
	}

	return ""
}

func extractLoadBalancerInfo(services *v1.ServiceList, portName string) []Gateway {
	var gateways = make([]Gateway, 0, len(services.Items))

	for _, svc := range services.Items {
		address := loadBalancerAddress(svc.Status.LoadBalancer.Ingress)

		var port int32
		for _, p := range svc.Spec.Ports {
			if p.Name == portName {
				port = p.Port
				break
			}
		}

		if address != "" && port != 0 {
			gateways = append(gateways, Gateway{Address: address, Port: port})
		}
	}

	return gateways
}

func extractNodePortInfo(service *v1.Service, pods *v1.PodList, nodes *v1.NodeList, portName string) ([]Gateway, error) {
	var port int32
	for _, p := range service.Spec.Ports {
		if p.Name == portName {
			port = p.NodePort
			break
		}
	}

	if port == 0 {
		return nil, fmt.Errorf("no %s port found", portName)
	}

	nodesWithPods := map[string]struct{}{}
	for _, pod := range pods.Items {
		nodesWithPods[pod.Spec.NodeName] = struct{}{}
	}

	gateways := make([]Gateway, 0)
	for _, node := range nodes.Items {
		if _, exists := nodesWithPods[node.Name]; !exists {
			continue
		}

		var address string
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeExternalIP {
				address = addr.Address
				break
			}
		}
		if address == "" {
			for _, addr := range node.Status.Addresses {
				if addr.Type == v1.NodeInternalIP {
					address = addr.Address
					break
				}
			}
		}

		if isNodeActive(&node) && address != "" {
			gateways = append(gateways, Gateway{Address: address, Port: port})
		}
	}

	return gateways, nil
}

func isNodeActive(node *v1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node.kubernetes.io/unschedulable" {
			return false
		}
	}

	for _, cond := range node.Status.Conditions {
		if cond.Type == v1.NodeReady && cond.Status != v1.ConditionTrue {
			return false
		}
	}

	return true
}

func extractAdvertisedGatewaysFromCM(cm *v1.ConfigMap) ([]Gateway, error) {
	key := advertisedGatewaysKey
	data, exists := cm.Data[key]
	if !exists {
		key = deprecatedAdvertisedGatewaysKey
		data, exists = cm.Data[key]
	}
	if !exists {
		return nil, fmt.Errorf("ConfigMap contains neither %s nor %s", advertisedGatewaysKey, deprecatedAdvertisedGatewaysKey)
	}

	gateways := make([]Gateway, 0)
	if err := json.Unmarshal([]byte(data), &gateways); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", key, err)
	}

	return gateways, nil
}

// ingressGateways collects the addresses of this cluster's ingress gateway. It serves both
// private metadata documents: for multicluster it is the east-west gateway the sidecar data
// plane uses, whose ambient counterpart is ambientGateways; for federation it is the gateway
// peers reach the published services through.
func (exp *Exporter) ingressGateways() ([]Gateway, error) {
	// debug
	fmt.Printf("INLET=%s\n", exp.ingressGatewayInlet)

	serviceList := &v1.ServiceList{Items: listFromStore[v1.Service](exp.ingressGatewayServiceInformer)}
	var ingressGateways = make([]Gateway, 0, len(serviceList.Items))

	advertised, err := exp.advertisedGateways(exp.ingressGatewayAdvertiseConfigMapInformer)
	if err != nil {
		return nil, err
	}
	if len(advertised) > 0 {
		logger.Printf("Found ingressGateways advertisements overriding config in ConfigMap: %s, %v", ingressGatewayAdvertiseConfigMapName, advertised)
		sortGateways(advertised)

		return advertised, nil
	}

	switch exp.ingressGatewayInlet {
	case "LoadBalancer":
		ingressGatewaysLoadBalancer := extractLoadBalancerInfo(serviceList, ingressGatewayPortName)
		fmt.Printf("ingressGatewaysLoadBalancer=%+v\n", ingressGatewaysLoadBalancer)
		ingressGateways = append(ingressGateways, ingressGatewaysLoadBalancer...)

	case "NodePort":
		podsList := &v1.PodList{Items: listFromStore[v1.Pod](exp.ingressGatewayPodInformer)}
		nodesList := &v1.NodeList{Items: listFromStore[v1.Node](exp.nodeInformer)}

		if len(serviceList.Items) == 0 {
			return nil, fmt.Errorf("no services found in ingressgateways")
		}
		ingressGatewaysNodePort, err := extractNodePortInfo(&serviceList.Items[0], podsList, nodesList, ingressGatewayPortName)
		if err != nil {
			return nil, fmt.Errorf("failed to extract node port info: %w", err)
		}
		// debug
		fmt.Printf("ingressGatewaysNodePort=%+v\n", ingressGatewaysNodePort)
		ingressGateways = append(ingressGateways, ingressGatewaysNodePort...)
	default:
		return nil, fmt.Errorf("unknown ingress gateway inlet type: %q", exp.ingressGatewayInlet)
	}

	// debug
	fmt.Printf("ingressGateways=%v\n", ingressGateways)

	sortGateways(ingressGateways)

	return ingressGateways, nil
}

// ambientGateways collects the addresses of this cluster's ambient gateway, the east-west
// gateway the ambient data plane uses. Its sidecar counterpart is ingressGateways.
func (exp *Exporter) ambientGateways() ([]Gateway, error) {
	serviceList := listFromStore[v1.Service](exp.ambientGatewayServiceInformer)
	if len(serviceList) == 0 {
		return nil, nil
	}

	service := &serviceList[0]

	advertised, err := exp.advertisedGateways(exp.ambientGatewayAdvertiseConfigMapInformer)
	if err != nil {
		return nil, err
	}

	var gateways []Gateway

	switch {
	case len(advertised) > 0:
		gateways = advertised
	case exp.ambientGatewayInlet == "LoadBalancer":
		gateways = extractLoadBalancerInfo(&v1.ServiceList{Items: serviceList}, ambientGatewayPortName)
	case exp.ambientGatewayInlet == "NodePort":
		podsList := &v1.PodList{Items: listFromStore[v1.Pod](exp.ambientGatewayPodInformer)}
		nodesList := &v1.NodeList{Items: listFromStore[v1.Node](exp.nodeInformer)}

		if gateways, err = extractNodePortInfo(service, podsList, nodesList, ambientGatewayPortName); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown ambient gateway inlet type: %q", exp.ambientGatewayInlet)
	}

	gateways, dropped := keepDialableAddresses(gateways)
	if len(gateways) == 0 {
		if len(dropped) == 0 {
			return nil, fmt.Errorf("service %s has no external address and no node is running a gateway pod; check that the gateway pods are scheduled (alliance.ambientGateway.nodeSelector and .tolerations) and that the inlet can assign an address", service.Name)
		}

		return nil, fmt.Errorf("no address of service %s is an IP address or DNS name peers can dial (%s); set alliance.ambientGateway.advertise to one", service.Name, addressList(dropped))
	}

	sortGateways(gateways)

	return gateways, nil
}

func sortGateways(gateways []Gateway) {
	sort.Slice(gateways, func(i, j int) bool {
		if gateways[i].Address != gateways[j].Address {
			return gateways[i].Address < gateways[j].Address
		}

		return gateways[i].Port < gateways[j].Port
	})
}

func listFromStore[T any](informer cache.SharedInformer) []T {
	items := informer.GetStore().List()
	out := make([]T, 0, len(items))
	for _, item := range items {
		object, ok := item.(*T)
		if !ok {
			continue
		}
		out = append(out, *object)
	}

	return out
}

func (exp *Exporter) advertisedGateways(informer cache.SharedInformer) ([]Gateway, error) {
	items := informer.GetStore().List()
	if len(items) == 0 {
		return nil, nil
	}

	cm, ok := items[0].(*v1.ConfigMap)
	if !ok {
		return nil, nil
	}

	advertised, err := extractAdvertisedGatewaysFromCM(cm)
	if err != nil {
		return nil, fmt.Errorf("failed to extract gateways from cm %s: %w", cm.Name, err)
	}

	return advertised, nil
}

func keepDialableAddresses(gateways []Gateway) ([]Gateway, []Gateway) {
	kept := make([]Gateway, 0, len(gateways))
	dropped := make([]Gateway, 0, len(gateways))
	seen := make(map[Gateway]struct{}, len(gateways))

	for _, gw := range gateways {
		address, ok := canonicalAmbientGatewayAddress(gw.Address)
		if !ok {
			dropped = append(dropped, gw)
			continue
		}

		gw.Address = address
		if _, duplicate := seen[gw]; duplicate {
			continue
		}

		seen[gw] = struct{}{}
		kept = append(kept, gw)
	}

	return kept, dropped
}

// Keep in sync with the function of the same name in ee/modules/110-istio/hooks/ee/multicluster_discovery.go.
func canonicalAmbientGatewayAddress(address string) (string, bool) {
	address = strings.TrimSpace(address)

	if ip := net.ParseIP(address); ip != nil {
		return ip.String(), true
	}

	host := strings.ToLower(strings.TrimSuffix(address, "."))
	if host == "" || len(validation.IsDNS1123Subdomain(host)) > 0 {
		return "", false
	}

	return host, true
}

func addressList(gateways []Gateway) string {
	addresses := make([]string, 0, len(gateways))
	for _, gw := range gateways {
		addresses = append(addresses, strconv.Quote(gw.Address))
	}

	return strings.Join(addresses, ", ")
}

func (exp *Exporter) reportAmbientGatewayState() error {
	if _, err := exp.ambientGateways(); err != nil {
		ambientGatewayAddressUnusable.Set(1)

		return err
	}

	ambientGatewayAddressUnusable.Set(0)

	return nil
}

func (exp *Exporter) trackAmbientGatewayState(ctx context.Context) {
	const interval = time.Minute

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	reported := false

	for {
		err := exp.reportAmbientGatewayState()
		switch {
		case err != nil && !reported:
			logger.Printf("No usable address for the ambient east-west gateway, publishing none: %v", err)
			reported = true
		case err == nil && reported:
			logger.Print("The ambient east-west gateway has a usable address again")
			reported = false
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// GetPublicServices main function for federation to get public services
func (exp *Exporter) GetPublicServices() []PublicService {
	services := exp.publicServiceInformer.GetStore().List()
	clusterDomain := exp.clusterDomain
	result := make([]PublicService, 0, len(services))

	for _, svc := range services {
		svc := svc.(*v1.Service)
		serviceInfo := PublicService{
			Hostname: fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, clusterDomain),
			Ports:    []Port{},
		}

		for _, p := range svc.Spec.Ports {
			serviceInfo.Ports = append(serviceInfo.Ports, Port{Name: p.Name, Port: p.Port})
		}

		result = append(result, serviceInfo)
	}

	return result
}

// SpiffeBundleJSON create JSON Spiffe Bundle
func (exp *Exporter) SpiffeBundleJSON() (string, error) {
	// extract root-cert.pem
	pubPem, err := exp.ExtractRootCaCert()
	if err != nil {
		return "", fmt.Errorf("failed to extract root ca cert: %v", err)
	}

	// Decode PEM
	pubPemBlock, _ := pem.Decode([]byte(pubPem))
	if pubPemBlock == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	// Parse x509
	cert, err := x509.ParseCertificate(pubPemBlock.Bytes)
	if err != nil {
		return "", fmt.Errorf("x509 parse error: %v", err)
	}

	// Convert pub key in  RSA
	rsaPublicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("certificate public key is not RSA")
	}

	// Encode N in base64 URL encoding
	n := base64.RawURLEncoding.EncodeToString(rsaPublicKey.N.Bytes())

	//  SpiffeKey
	sk := SpiffeKey{
		Kty: "RSA",
		Use: "x509-svid",
		E:   "AQAB",
		N:   n,
		X5c: [][]byte{pubPemBlock.Bytes},
	}

	// Create Spiffe Bundle
	se := SpiffeEndpoint{
		SpiffeSequence:    1,
		SpiffeRefreshHint: 2419200,
		Keys:              []SpiffeKey{sk},
	}

	// Encode in  JSON
	jsonbuf, err := json.MarshalIndent(se, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal Spiffe JSON: %v", err)
	}

	return string(jsonbuf), nil
}

// ExtractRemotePublicMetadata extract remote-public-metadata.json fom Secret d8-remote-clusters-public-metadata
func (exp *Exporter) ExtractRemotePublicMetadata() (RemotePublicMetadata, error) {
	items := exp.remoteClustersPublicMetadataInformer.GetStore().List()
	if len(items) == 0 {
		return nil, fmt.Errorf("no secrets found in d8-remote-clusters-public-metadata")
	}

	secret, ok := items[0].(*v1.Secret)
	if !ok {
		return nil, fmt.Errorf("failed to cast item to *v1.Secret")
	}

	data, exists := secret.Data["remote-public-metadata.json"]
	if !exists {
		return nil, fmt.Errorf("secret d8-remote-clusters-public-metadata does not contain remote-public-metadata.json")
	}

	var metadata RemotePublicMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse remote-public-metadata.json: %w", err)
	}

	return metadata, nil
}

// ExtractAuthnKeyPub extract pub.pem from secret d8-remote-authn-keypair
func (exp *Exporter) ExtractAuthnKeyPub() (string, error) {
	items := exp.remoteAuthnKeypairInformer.GetStore().List()
	if len(items) == 0 {
		return "", fmt.Errorf("no secrets found in d8-remote-authn-keypair")
	}

	secret, ok := items[0].(*v1.Secret)
	if !ok {
		return "", fmt.Errorf("failed to cast item to *v1.Secret")
	}

	pubKeyBase64, exists := secret.Data["pub.pem"]
	if !exists {
		return "", fmt.Errorf("secret  d8-remote-authn-keypair does not contain pub.pem")
	}

	pubKey := string(pubKeyBase64)

	return pubKey, nil
}

// ExtractRootCaCert  extract pub.pem from cm istio-ca-root-cert
func (exp *Exporter) ExtractRootCaCert() (string, error) {
	items := exp.certCAConfigMapInformer.GetStore().List()
	if len(items) == 0 {
		return "", fmt.Errorf("no configmaps found in istio-ca-root-cert")
	}

	cm, ok := items[0].(*v1.ConfigMap)
	if !ok {
		return "", fmt.Errorf("failed to cast item to *v1.ConfigMap")
	}

	rootCAPem, exists := cm.Data["root-cert.pem"]
	if !exists {
		return "", fmt.Errorf("ConfigMap does not contain root-cert.pem")
	}

	pubKey := rootCAPem

	return pubKey, nil
}

// CheckAuthn check JWT token authentication
func (exp *Exporter) CheckAuthn(header http.Header, scope string) (string, error) {
	reqTokenString := header.Get("Authorization")
	if !strings.HasPrefix(reqTokenString, "Bearer ") {
		return "", fmt.Errorf("Bearer authorization required")
	}
	reqTokenString = strings.TrimPrefix(reqTokenString, "Bearer ")

	reqToken, err := jose.ParseSigned(reqTokenString)
	if err != nil {
		return "", err
	}
	payloadBytes := reqToken.UnsafePayloadWithoutVerification()

	var payload JwtPayload
	err = json.Unmarshal(payloadBytes, &payload)
	if err != nil {
		return "", err
	}

	// Load remote-public-metadata.json
	remotePublicMetadataMap, err := exp.ExtractRemotePublicMetadata()
	if err != nil {
		return "", err
	}

	// Check JWT
	expectedUUID := exp.clusterUUID
	if payload.Aud != expectedUUID {
		return "", fmt.Errorf("JWT is signed for wrong destination cluster. Expected: %s, Got: %s", expectedUUID, payload.Aud)
	}

	if payload.Scope != scope {
		return "", fmt.Errorf("JWT is signed for wrong scope")
	}

	if payload.Exp < time.Now().UTC().Unix() {
		return "", fmt.Errorf("JWT token expired")
	}

	// Checking if the source cluster is known
	_, ok := remotePublicMetadataMap[payload.Sub]
	if !ok {
		return "", fmt.Errorf("JWT is signed for unknown source cluster")
	}
	uuid := payload.Sub

	// check sign JWT
	remoteAuthnKeyPubBlock, _ := pem.Decode([]byte(remotePublicMetadataMap[payload.Sub].AuthnKeyPub))
	if remoteAuthnKeyPubBlock == nil {
		return "", fmt.Errorf("failed to decode public key PEM")
	}

	remoteAuthnKeyPub, err := x509.ParsePKIXPublicKey(remoteAuthnKeyPubBlock.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse public key: %w", err)
	}

	if _, err := reqToken.Verify(remoteAuthnKeyPub); err != nil {
		return "", fmt.Errorf("cannot verify JWT token with known public key")
	}

	return uuid, nil
}

func (exp *Exporter) RenderMulticlusterPrivateMetadataJSON() string {
	var pm MulticlusterPrivateMetadata

	ingressGateways, err := exp.ingressGateways()
	if err != nil {
		logger.Printf("failed to get ingress gateways, publishing none: %v", err)
	} else {
		pm.IngressGateways = &ingressGateways
	}

	ambientGateways, err := exp.ambientGateways()
	if err != nil {
		logger.Printf("failed to get ambient gateways, publishing none: %v", err)
	} else if len(ambientGateways) > 0 {
		pm.AmbientGateways = &ambientGateways
	}

	pm.ClusterID = exp.multiclusterClusterID
	if len(pm.ClusterID) == 0 {
		panic("Error reading MULTICLUSTER_CLUSTER_ID from env")
	}

	pm.NetworkName = exp.multiclusterNetworkName
	if len(pm.NetworkName) == 0 {
		panic("Error reading MULTICLUSTER_NETWORK_NAME from env")
	}

	pm.APIHost = exp.multiclusterAPIHost
	if len(pm.APIHost) == 0 {
		panic("Error reading MULTICLUSTER_API_HOST from env")
	}

	jsonbuf, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		panic("Error marshalling cluster private metadata to json: " + err.Error())
	}
	return string(jsonbuf)
}

func (exp *Exporter) RenderFederationPrivateMetadataJSON() string {
	var pm FederationPrivateMetadata

	ingressGateways, err := exp.ingressGateways()
	if err != nil {
		logger.Printf("failed to get ingress gateways, publishing none: %v", err)
	} else {
		pm.IngressGateways = &ingressGateways
	}

	if exp.federationEnabled == "true" {
		services := exp.GetPublicServices()
		pm.PublicServices = &services
	}

	jsonbuf, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		panic("Error marshalling cluster private metadata to json: " + err.Error())
	}
	return string(jsonbuf)
}

func (exp *Exporter) RenderPublicMetadataJSON() string {
	clusterUUID := exp.clusterUUID
	if len(clusterUUID) == 0 {
		panic("Error reading cluster UUID")
	}

	authnKeyPubPem, err := exp.ExtractAuthnKeyPub()
	if err != nil {
		logger.Printf("failed to extract authn key pub pem: %v", err)
	}

	rootCAPem, err := exp.ExtractRootCaCert()
	if err != nil {
		logger.Printf("failed to extract root ca cert: %v", err)
	}

	pm := AlliancePublicMetadata{
		ClusterUUID: clusterUUID,
		AuthnKeyPub: authnKeyPubPem,
		RootCA:      rootCAPem,
	}

	jsonbuf, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		panic("Error marshalling cluster public metadata to json: " + err.Error())
	}

	return string(jsonbuf)
}

func (exp *Exporter) checkIfAccessedViaDeprecatedSubdomain(r *http.Request, accessedWithUUID string) {
	dontWant := strings.TrimSpace(os.Getenv("ISTIO_METADATA_OLD_PUBLIC_HOST"))

	if dontWant == "" {
		return
	}
	hostname, err := prepareEndpointString(r.Host)
	if err != nil {
		return
	}
	if hostname == "" || hostname != dontWant {
		return
	}
	// Load remote-public-metadata.json
	remotePublicMetadataMap, err := exp.ExtractRemotePublicMetadata()
	if err != nil {
		return
	}
	// Checking if the source cluster is known
	PublicMetadata, ok := remotePublicMetadataMap[accessedWithUUID]
	if !ok {
		logger.Printf("Request with JWT is signed for unknown source cluster with uuid %s", accessedWithUUID)
		return
	}

	privateAllianceDeprecatedSubdomainRequests.With(prometheus.Labels{
		"remote_uuid":           PublicMetadata.ClusterUUID,
		"alliance_kind":         PublicMetadata.AllianceRef.Kind,
		"name":                  PublicMetadata.AllianceRef.Name,
		"requested_to_hostname": hostname,
	}).Inc()

	logger.Printf("Request via deprecated subdomain: kind=%s, name=%s, cluster_uuid=%s dontWant=%s haveGot=%s", PublicMetadata.AllianceRef.Kind, PublicMetadata.AllianceRef.Name, PublicMetadata.ClusterUUID, dontWant, hostname)
}

func prepareEndpointString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty metadataEndpoint")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("no host in metadataEndpoint")
	}
	return strings.ToLower(host), nil
}
