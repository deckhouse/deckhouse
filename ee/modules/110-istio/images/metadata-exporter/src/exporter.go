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
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Exporter struct {
	namespace                            string
	labelSelector                        string
	clientSet                            *kubernetes.Clientset
	lwPods                               *cache.ListWatch
	lwNodes                              *cache.ListWatch
	lwService                            *cache.ListWatch
	lwConfigMaps                         *cache.ListWatch
	lwCertCAConfigMap                    *cache.ListWatch
	lwPublicService                      *cache.ListWatch // for federation
	lwdRemoteClustersPublicMetadata      *cache.ListWatch
	lwRemoteAuthnKeypair                 *cache.ListWatch
	serviceInformer                      cache.SharedInformer
	podInformer                          cache.SharedInformer
	nodeInformer                         cache.SharedInformer
	configMapInformer                    cache.SharedInformer
	certCAConfigMapInformer              cache.SharedInformer
	publicServiceInformer                cache.SharedInformer // for federation
	remoteClustersPublicMetadataInformer cache.SharedInformer
	remoteAuthnKeypair                   cache.SharedInformer
	inlet                                string
	clusterDomain                        string
	clusterUUID                          string
	multicluserNetworkName               string
	multiclusterAPIHost                  string
	federationEnabled                    string
}

func New(namespace string, labelSelector string) (*Exporter, error) {

	// Get enviroments

	inlet := os.Getenv("INLET")
	clusterDomain := os.Getenv("CLUSTER_DOMAIN")
	clusterUUID := os.Getenv("CLUSTER_UUID")
	multicluserNetworkName := os.Getenv("MULTICLUSTER_NETWORK_NAME")
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

	lwService := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"services",
		namespace,
		func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("app=%s", labelSelector)
		},
	)

	lwPods := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"pods",
		namespace,
		func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("app=%s", labelSelector)
		},
	)

	lwNodes := cache.NewListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"nodes",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	lwConfigMap := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=metadata-exporter-ingressgateway-advertise"
		},
	)

	lwPublicServices := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"services",
		metav1.NamespaceAll,
		func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("federation.istio.deckhouse.io/public-service=")
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
			namespace:                       namespace,
			labelSelector:                   labelSelector,
			clientSet:                       clientSet,
			lwPods:                          lwPods,
			lwNodes:                         lwNodes,
			lwService:                       lwService,
			lwConfigMaps:                    lwConfigMap,
			lwPublicService:                 lwPublicServices,
			lwCertCAConfigMap:               lwCertCAConfigMap,
			lwdRemoteClustersPublicMetadata: lwRemoteClustersPublicMetadata,
			lwRemoteAuthnKeypair:            lwRemoteAuthnKeypair,
			inlet:                           inlet,
			clusterDomain:                   clusterDomain,
			clusterUUID:                     clusterUUID,
			multicluserNetworkName:          multicluserNetworkName,
			multiclusterAPIHost:             multiclusterAPIHost,
			federationEnabled:               federationEnabled,
		},
		nil
}

func (exp *Exporter) watchIngressGateways(ctx context.Context) {

	exp.serviceInformer = cache.NewSharedInformer(
		exp.lwService,
		&v1.Service{},
		0,
	)
	exp.podInformer = cache.NewSharedInformer(
		exp.lwPods,
		&v1.Pod{},
		0,
	)
	exp.nodeInformer = cache.NewSharedInformer(
		exp.lwNodes,
		&v1.Node{},
		0,
	)

	exp.configMapInformer = cache.NewSharedInformer(
		exp.lwConfigMaps,
		&v1.ConfigMap{},
		0,
	)

	exp.publicServiceInformer = cache.NewSharedInformer(
		exp.lwPublicService,
		&v1.Service{},
		0,
	)

	exp.certCAConfigMapInformer = cache.NewSharedInformer(
		exp.lwCertCAConfigMap,
		&v1.ConfigMap{},
		0,
	)

	exp.remoteClustersPublicMetadataInformer = cache.NewSharedInformer(
		exp.lwdRemoteClustersPublicMetadata,
		&v1.Secret{},
		0,
	)

	exp.remoteAuthnKeypair = cache.NewSharedInformer(
		exp.lwRemoteAuthnKeypair,
		&v1.Secret{},
		0,
	)

	go exp.serviceInformer.Run(ctx.Done())
	go exp.podInformer.Run(ctx.Done())
	go exp.nodeInformer.Run(ctx.Done())
	go exp.configMapInformer.Run(ctx.Done())
	go exp.publicServiceInformer.Run(ctx.Done())
	go exp.certCAConfigMapInformer.Run(ctx.Done())
	go exp.remoteClustersPublicMetadataInformer.Run(ctx.Done())
	go exp.remoteAuthnKeypair.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(),
		exp.serviceInformer.HasSynced,
		exp.podInformer.HasSynced,
		exp.nodeInformer.HasSynced,
		exp.configMapInformer.HasSynced,
		exp.publicServiceInformer.HasSynced,
		exp.certCAConfigMapInformer.HasSynced,
		exp.remoteClustersPublicMetadataInformer.HasSynced,
		exp.remoteAuthnKeypair.HasSynced) {
		fmt.Println("[ERROR] Failed to sync caches")
		return
	}
}

func extractLoadBalancerInfo(services *v1.ServiceList) ([]IngressGateway, error) {
	var gateways = make([]IngressGateway, 0, len(services.Items))

	for _, svc := range services.Items {
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			continue
		}

		var address string
		ingress := svc.Status.LoadBalancer.Ingress[0]
		if ingress.IP != "" {
			address = ingress.IP
		} else if ingress.Hostname != "" {
			address = ingress.Hostname
		}

		var port int32
		for _, p := range svc.Spec.Ports {
			if p.Name == "tls" {
				port = p.Port
				break
			}
		}

		if address != "" && port != 0 {
			gateways = append(gateways, IngressGateway{Address: address, Port: port})
		}
	}

	return gateways, nil
}

func extractNodePortInfo(service *v1.Service, pods *v1.PodList, nodes *v1.NodeList) ([]IngressGateway, error) {
	var port int32
	for _, p := range service.Spec.Ports {
		if p.Name == "tls" {
			port = p.NodePort
			break
		}
	}

	if port == 0 {
		return nil, fmt.Errorf("no tls port found")
	}

	nodesWithPods := map[string]struct{}{}
	for _, pod := range pods.Items {
		nodesWithPods[pod.Spec.NodeName] = struct{}{}
	}

	gateways := make([]IngressGateway, 0)
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
			gateways = append(gateways, IngressGateway{Address: address, Port: port})
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

func extractIngressGatewaysFromCM(cm *v1.ConfigMap) ([]IngressGateway, error) {

	data, exists := cm.Data["ingressgateways-array.json"]
	if !exists {
		return nil, fmt.Errorf("ConfigMap does not contain ingressgateways-array.json")
	}

	gateways := make([]IngressGateway, 0)
	if err := json.Unmarshal([]byte(data), &gateways); err != nil {
		return nil, fmt.Errorf("failed to parse ingressGatewaysArray: %w", err)
	}

	return gateways, nil
}

// GetIngressGateways Main function to get all ingress gateways
func (exp *Exporter) GetIngressGateways() ([]IngressGateway, error) {
	inlet := exp.inlet
	//debug
	fmt.Printf("INLET=%s\n", inlet)

	items := exp.serviceInformer.GetStore().List()
	var ingressGateways = make([]IngressGateway, 0, len(items))
	serviceList := &v1.ServiceList{}
	for _, item := range items {
		service, ok := item.(*v1.Service)
		if !ok {
			continue
		}
		serviceList.Items = append(serviceList.Items, *service)
	}

	// Try get ConfigMap
	// Prioritizes ConfigMap if it is present in the inlet Gateways is ignored
	ingressGwFromCm := exp.configMapInformer.GetStore().List()
	if len(ingressGwFromCm) > 0 {
		cm := ingressGwFromCm[0].(*v1.ConfigMap)
		ingressGatewaysConfigmap, err := extractIngressGatewaysFromCM(cm)
		if err != nil {
			return nil, fmt.Errorf("failed to extract gateways from cm: %w", err)
		}
		logger.Printf("Found ingresGateways advertisements overriding config in ConfigMap: %s, %v", cm.Name, ingressGatewaysConfigmap)
		ingressGateways = append(ingressGateways, ingressGatewaysConfigmap...)
		return ingressGateways, nil
	}

	switch inlet {

	case "LoadBalancer":
		ingressGatewaysLoadBalancer, err := extractLoadBalancerInfo(serviceList)
		if err != nil {
			return nil, fmt.Errorf("failed to extract load balancer info: %w", err)
		}
		fmt.Printf("ingressGatewaysLoadBalancer=%+v\n", ingressGatewaysLoadBalancer)
		ingressGateways = append(ingressGateways, ingressGatewaysLoadBalancer...)

	case "NodePort":
		podsItems := exp.podInformer.GetStore().List()
		podsList := &v1.PodList{}
		for _, item := range podsItems {
			pod, ok := item.(*v1.Pod)
			if !ok {
				continue
			}
			podsList.Items = append(podsList.Items, *pod)
		}
		nodesItems := exp.nodeInformer.GetStore().List()
		nodesList := &v1.NodeList{}
		for _, item := range nodesItems {
			node, ok := item.(*v1.Node)
			if !ok {
				continue
			}
			nodesList.Items = append(nodesList.Items, *node)
		}

		if len(serviceList.Items) == 0 {
			return nil, fmt.Errorf("no services found in ingressgateways")
		}
		ingressGatewaysNodePort, err := extractNodePortInfo(&serviceList.Items[0], podsList, nodesList)
		if err != nil {
			return nil, fmt.Errorf("failed to extract node port info: %w", err)
		}
		//debug
		fmt.Printf("ingressGatewaysNodePort=%+v\n", ingressGatewaysNodePort)
		ingressGateways = append(ingressGateways, ingressGatewaysNodePort...)
	default:
		return nil, fmt.Errorf("unknown inlet type: %s", inlet)
	}

	//debug
	fmt.Printf("ingressGateways=%v\n", ingressGateways)

	return ingressGateways, nil
}

// GetPublicServices main function for federation to get public services
func (exp *Exporter) GetPublicServices() ([]PublicServices, error) {
	services := exp.publicServiceInformer.GetStore().List()
	clusterDomain := exp.clusterDomain
	result := make([]PublicServices, 0, len(services))

	for _, svc := range services {
		svc := svc.(*v1.Service)
		serviceInfo := PublicServices{
			Hostname: fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, clusterDomain),
			Ports:    []Port{},
		}

		for _, p := range svc.Spec.Ports {
			serviceInfo.Ports = append(serviceInfo.Ports, Port{Name: p.Name, Port: p.Port})
		}

		result = append(result, serviceInfo)
	}

	return result, nil
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

	items := exp.remoteAuthnKeypair.GetStore().List()
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

	pubKey := string(rootCAPem)

	return pubKey, nil
}

// CheckAuthn check JWT token authentication
func (exp *Exporter) CheckAuthn(header http.Header, scope string) error {
	reqTokenString := header.Get("Authorization")
	if !strings.HasPrefix(reqTokenString, "Bearer ") {
		return fmt.Errorf("Bearer authorization required")
	}
	reqTokenString = strings.TrimPrefix(reqTokenString, "Bearer ")

	reqToken, err := jose.ParseSigned(reqTokenString)
	if err != nil {
		return err
	}
	payloadBytes := reqToken.UnsafePayloadWithoutVerification()

	var payload JwtPayload
	err = json.Unmarshal(payloadBytes, &payload)
	if err != nil {
		return err
	}

	// Load remote-public-metadata.json
	remotePublicMetadataMap, err := exp.ExtractRemotePublicMetadata()
	if err != nil {
		return err
	}

	// Check JWT
	expectedUUID := exp.clusterUUID
	if payload.Aud != expectedUUID {
		return fmt.Errorf("JWT is signed for wrong destination cluster. Expected: %s, Got: %s", expectedUUID, payload.Aud)
	}

	if payload.Scope != scope {
		return fmt.Errorf("JWT is signed for wrong scope")
	}

	if payload.Exp < time.Now().UTC().Unix() {
		return fmt.Errorf("JWT token expired")
	}

	// Checking if the source cluster is known
	_, ok := remotePublicMetadataMap[payload.Sub]
	if !ok {
		return fmt.Errorf("JWT is signed for unknown source cluster")
	}

	// check sign JWT
	remoteAuthnKeyPubBlock, _ := pem.Decode([]byte(remotePublicMetadataMap[payload.Sub].AuthnKeyPub))
	if remoteAuthnKeyPubBlock == nil {
		return fmt.Errorf("failed to decode public key PEM")
	}

	remoteAuthnKeyPub, err := x509.ParsePKIXPublicKey(remoteAuthnKeyPubBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	if _, err := reqToken.Verify(remoteAuthnKeyPub); err != nil {
		return fmt.Errorf("cannot verify JWT token with known public key")
	}

	return nil
}

func (exp *Exporter) RenderMulticlusterPrivateMetadataJSON() string {
	var pm MulticlusterPrivateMetadata

	ingressGateways, err := exp.GetIngressGateways()
	if err != nil {
		fmt.Printf("failed to get ingress gateways: %v", err)
	}

	pm.IngressGateways = &ingressGateways

	pm.NetworkName = exp.multicluserNetworkName
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

	ingressGateways, err := exp.GetIngressGateways()
	if err != nil {
		fmt.Printf("failed to get ingress gateways: %v", err)
	}

	pm.IngressGateways = &ingressGateways

	if exp.federationEnabled == "true" {
		services, err := exp.GetPublicServices()
		if err != nil {
			fmt.Printf("failed to get public services: %v", err)
		}
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
		fmt.Printf("failed to extract authn key pub pem: %v", err)
	}

	rootCAPem, err := exp.ExtractRootCaCert()
	if err != nil {
		fmt.Printf("failed to extract root ca cert: %v", err)
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
