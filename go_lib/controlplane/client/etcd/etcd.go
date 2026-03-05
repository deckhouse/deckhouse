package etcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	kubeadmapp "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapp"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"

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

func addMembersToPodManifest(podManifest []byte, initialCluster []*etcdserverpb.Member) []byte {
	// podManifest == static pod manifest
	// change --initial-cluster=... into
	// --initial-cluster=borovets-multi-master-master-2=https://10.241.44.16:2380,borovets-multi-master-master-0=https://10.241.32.26:2380,borovets-multi-master-master-1=https://10.241.36.19:2380

	podManifestString := string(podManifest)
	var endpoints []string
	for _, member := range initialCluster {
		endpoints = append(endpoints, fmt.Sprintf("%s=%s", member.Name, member.PeerURLs[0]))
	}
	initialClusterString := strings.Join(endpoints, ",")

	re := regexp.MustCompile(`--initial-cluster=[^\s\n\r]*`)
	podManifestString = re.ReplaceAllString(podManifestString, "--initial-cluster="+initialClusterString)

	return []byte(podManifestString)
}

func prepareAndWriteEtcdStaticPod(podManifest []byte, config *EtcdConfig, nodeName string, initialCluster []*etcdserverpb.Member) error {
	if len(initialCluster) > 0 {
		podManifest = addMembersToPodManifest(podManifest, initialCluster)
	}

	if err := WriteStaticPodToDisk(podManifest, constants.Etcd, config.ManifestDir); err != nil {
		return err
	}
	logger.Info("[etcd] podManifest is written to disk")

	return nil
}

func NewEtcdClient(client clientset.Interface, certificatesDir string, endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, error) {
	var etcdClient *clientv3.Client
	etcdClient, err := NewFromCluster(client, certificatesDir)
	if err != nil {
		return nil, err
	}
	return etcdClient, nil
}

func InitCluster(podManifest []byte, config *EtcdConfig, endpoint *kubeadmapi.APIEndpoint, nodeName string) error {

	logger.Info("[etcd] Creating static Pod manifest during init cluster", slog.String("component", constants.Etcd))

	if err := prepareAndWriteEtcdStaticPod(podManifest, config, nodeName, []*etcdserverpb.Member{}); err != nil {
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
	logger.Info("TEST-ETCD KUBECLIENT: kubeClient", slog.Any("kubeClient", kubeClient))
	pods, err := kubeClient.CoreV1().Pods("d8-chrony").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	logger.Info("TEST-ETCD KUBECLIENT: pods:", slog.String("pods", pods.Items[0].Name))
	////////////////////////////////

	////UNCOMMENT THIS BLOCK//////////////// test etcdPeerAddress ///////////////////////
	etcdPeerAddress := GetPeerURL(endpoint)
	/////////////////////////////////////////////////////////////////////

	var cluster []*clientv3.Member
	var etcdClient *clientv3.Client

	etcdClient, err = NewFromCluster(kubeClient, config.CertificatesDir)
	if err != nil {
		return err
	}

	////DELETE THIS BLOCK//////////////// test etcdClient ///////////////////////
	logger.Info("TEST-ETCD client: etcdClient", slog.Any("etcdClient", etcdClient))
	clusterAuthStatus, err := etcdClient.AuthStatus(context.Background())
	if err != nil {
		return err
	}
	logger.Info("TEST-ETCD client: clusterAuthStatus", slog.Any("clusterAuthStatus", clusterAuthStatus))
	clusterMembers, err := etcdClient.MemberList(context.Background())
	if err != nil {
		return err
	}
	logger.Info("TEST-ETCD client: clusterMembers", slog.Any("clusterMembers", clusterMembers))
	/////////////////////////////////////////////////////////////////////

	////UNCOMMENT THIS BLOCK//////////////// test etcdPeerAddress ///////////////////////
	logger.Info("[etcd] Adding etcd member", slog.String("etcdPeerAddress", etcdPeerAddress))
	// cluster, err = etcdClient.AddMemberAsLearner(nodeName, etcdPeerAddress)
	clusterResponse, err := etcdClient.MemberAddAsLearner(context.Background(), []string{etcdPeerAddress})
	if err != nil {
		return err
	}
	/////////////////////////////////////////////////////////////////////

	////DELETE THIS BLOCK//////////////// test cluster ///////////////////////
	cluster = []*clientv3.Member{
		{Name: "borovets-multi-master-master-0", PeerURLs: []string{"https://10.241.32.26:2380"}},
		{Name: "borovets-multi-master-master-1", PeerURLs: []string{"https://10.241.36.19:2380"}},
		{Name: "borovets-multi-master-master-2", PeerURLs: []string{"https://10.241.44.16:2380"}},
	}
	logger.Info("TEST-ETCD client: [etcd] Announced new etcd member joining to the existing etcd cluster")
	logger.Info("TEST-ETCD client: Updated etcd member list", slog.Any("cluster", cluster))
	/////////////////////////////////////////////////////////////////////

	logger.Info("[etcd] Creating static Pod manifest during join cluster", slog.String("component", constants.Etcd))

	// if err := prepareAndWriteEtcdStaticPod(podManifest, config, nodeName, cluster); err != nil {
	// 	return err
	// }
	if err := prepareAndWriteEtcdStaticPod(podManifest, config, nodeName, clusterResponse.Members); err != nil {
		return err
	}

	/////UNCOMMENT THIS BLOCK ///////// test etcdPeerAddress ///////////////////////
	//learnerID, err := etcdClient.GetMemberID(etcdPeerAddress)
	// if err != nil {
	// 	return err
	// }
	_, err = etcdClient.MemberPromote(context.Background(), clusterResponse.Member.ID)
	if err != nil {
		return err
	}

	logger.Info("[etcd] Waiting for the new etcd member to join the cluster", slog.Duration("timeout", etcdHealthyCheckInterval*etcdHealthyCheckRetries))
	if _, err := WaitForClusterAvailable(etcdClient, etcdHealthyCheckRetries, etcdHealthyCheckInterval); err != nil {
		return err
	}
	/////////////////////////////////////////////////////////////////////

	return nil
}
