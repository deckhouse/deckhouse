package etcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	kubeadmapp "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	kubeadmapi "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/pkg/errors"
	clientset "k8s.io/client-go/kubernetes"
)

type EtcdConfig struct {
	ManifestDir     string
	CertificatesDir string
}

var logger = log.Default().Named("etcd")

const (
	etcdHealthyCheckInterval = 5 * time.Second
	etcdHealthyCheckRetries  = 8
)

func WriteStaticPodToDisk(podManifest []byte, componentName, manifestDir string) error {

	if err := os.MkdirAll(manifestDir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", manifestDir)
	}

	filename := constants.GetStaticPodFilepath(componentName, manifestDir)

	if err := os.WriteFile(filename, podManifest, 0600); err != nil {
		return errors.Wrapf(err, "failed to write static pod manifest file for %q (%q)", componentName, filename)
	}

	return nil
}

func addMembersToPodManifest(podManifest []byte, initialCluster []Member) []byte {
	// podManifest == static pod manifest
	// change --initial-cluster=... into
	// --initial-cluster=borovets-multi-master-master-2=https://10.241.44.16:2380,borovets-multi-master-master-0=https://10.241.32.26:2380,borovets-multi-master-master-1=https://10.241.36.19:2380

	podManifestString := string(podManifest)
	var endpoints []string
	for _, member := range initialCluster {
		endpoints = append(endpoints, fmt.Sprintf("%s=%s", member.Name, member.PeerURL))
	}
	initialClusterString := strings.Join(endpoints, ",")

	re := regexp.MustCompile(`--initial-cluster=[^\s\n\r]*`)
	podManifestString = re.ReplaceAllString(podManifestString, "--initial-cluster="+initialClusterString)

	return []byte(podManifestString)
}

func prepareAndWriteEtcdStaticPod(podManifest []byte, config *EtcdConfig, nodeName string, initialCluster []Member) error {
	if len(initialCluster) > 0 {
		podManifest = addMembersToPodManifest(podManifest, initialCluster)
	}

	if err := WriteStaticPodToDisk(podManifest, constants.Etcd, config.ManifestDir); err != nil {
		return err
	}
	logger.Info("[etcd] podManifest is written to disk")

	return nil
}

func NewEtcdClient(client clientset.Interface, certificatesDir string, endpoints []string, tlsConfig *tls.Config) (*Client, error) {
	var etcdClient *Client
	etcdClient, err := NewFromCluster(client, certificatesDir)
	if err != nil {
		return nil, err
	}
	return etcdClient, nil
}

func InitCluster(podManifest []byte, config *EtcdConfig, endpoint *kubeadmapi.APIEndpoint, nodeName string) error {

	logger.Info(fmt.Sprintf("[etcd] Creating static Pod manifest for %q during init cluster", constants.Etcd))

	if err := prepareAndWriteEtcdStaticPod(podManifest, config, nodeName, []Member{}); err != nil {
		return err
	}
	return nil
}

func JoinCluster(podManifest []byte, config *EtcdConfig, endpoint *kubeadmapi.APIEndpoint, nodeName string) error {

	kubeClient, err := kubeadmapp.MyNewKubernetesClient()
	if err != nil {
		return err
	}

	///////////////////////////// test kubeClient
	logger.Info(fmt.Sprintf("TEST-ETCD KUBECLIENT: kubeClient: %v", kubeClient))
	pods, err := kubeClient.CoreV1().Pods("d8-chrony").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("TEST-ETCD KUBECLIENT: pods: %v", pods))
	////////////////////////////////

	etcdPeerAddress := GetPeerURL(endpoint)

	var cluster []Member
	var etcdClient *Client
	////////////////////////////////////////////
	// Creates an etcd client that connects to all the local/stacked etcd members.
	logger.Info("TEST-ETCD client: creating etcd client that connects to etcd pods")
	etcdClient, err = NewFromCluster(kubeClient, config.CertificatesDir)
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("TEST-ETCD client: etcdClient: %v", etcdClient))
	clusterStatus, err := etcdClient.getClusterStatus()
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("TEST-ETCD client: clusterStatus: %v", clusterStatus))

	logger.Info(fmt.Sprintf("[etcd] Adding etcd member: %s", etcdPeerAddress))
	cluster, err = etcdClient.AddMemberAsLearner(nodeName, etcdPeerAddress)
	if err != nil {
		return err
	}
	cluster = []Member{
		{Name: "borovets-multi-master-master-0", PeerURL: "https://10.241.32.26:2380"},
		{Name: "borovets-multi-master-master-1", PeerURL: "https://10.241.36.19:2380"},
		{Name: "borovets-multi-master-master-2", PeerURL: "https://10.241.44.16:2380"},
	}
	logger.Info(fmt.Sprintf("TEST-ETCD client: [etcd] Announced new etcd member joining to the existing etcd cluster"))
	logger.Info(fmt.Sprintf("TEST-ETCD client: Updated etcd member list: %v", cluster))
	////////////////////////////////////////////

	logger.Info(fmt.Sprintf("[etcd] Creating static Pod manifest for %q during join cluster", constants.Etcd))

	if err := prepareAndWriteEtcdStaticPod(podManifest, config, nodeName, cluster); err != nil {
		return err
	}

	learnerID, err := etcdClient.GetMemberID(etcdPeerAddress)
	if err != nil {
		return err
	}
	err = etcdClient.MemberPromote(learnerID)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("[etcd] Waiting for the new etcd member to join the cluster. This can take up to %v\n", etcdHealthyCheckInterval*etcdHealthyCheckRetries))
	if _, err := etcdClient.WaitForClusterAvailable(etcdHealthyCheckRetries, etcdHealthyCheckInterval); err != nil {
		return err
	}

	return nil
}
