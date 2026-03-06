package etcd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	kubeclient "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var logger = log.Default().Named("etcd")

const (
	etcdHealthyCheckInterval = 5 * time.Second
	etcdHealthyCheckRetries  = 8
	KubernetesAPICallTimeout = 1 * time.Minute
)

func InitCluster(podManifest []byte, nodeName string, options ...option) error {
	opt := prepareOptions(options...)

	logger.Info("Creating static Pod manifest during init cluster", slog.String("component", constants.Etcd))

	if err := prepareAndWriteEtcdStaticPod(podManifest, opt, nodeName, []*etcdserverpb.Member{}); err != nil {
		return err
	}
	return nil
}

func JoinCluster(podManifest []byte, ip string, nodeName string, options ...option) error {
	opt := prepareOptions(options...)

	kubeClient, err := kubeclient.MyNewKubernetesClient()
	if err != nil {
		return err
	}

	//////DELETE THIS BLOCK/////////// test kubeClient ///////////////////////////////////////
	logger.Info("TEST-ETCD KUBECLIENT: kubeClient", slog.Any("kubeClient", kubeClient))
	pods, err := kubeClient.CoreV1().Pods("d8-chrony").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	logger.Info("TEST-ETCD KUBECLIENT: pods:", slog.String("pods", pods.Items[0].Name))
	/////////////////////////////////////////////////////////////////////

	////UNCOMMENT THIS BLOCK//////////////// test etcdPeerAddress ///////////////////////
	// etcdPeerAddress := GetPeerURL(ip)
	/////////////////////////////////////////////////////////////////////

	var cluster []*etcdserverpb.Member
	var etcdClient *clientv3.Client

	etcdClient, err = client.NewFromCluster(kubeClient, opt.CertificatesDir)
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
	// logger.Info("Adding etcd member", slog.String("etcdPeerAddress", etcdPeerAddress))
	// // cluster, err = etcdClient.AddMemberAsLearner(nodeName, etcdPeerAddress)
	// clusterResponse, err := etcdClient.MemberAddAsLearner(context.Background(), []string{etcdPeerAddress})
	// if err != nil {
	// 	return err
	// }
	/////////////////////////////////////////////////////////////////////

	////DELETE THIS BLOCK//////////////// test cluster ///////////////////////
	cluster = []*etcdserverpb.Member{
		{Name: "borovets-multi-master-master-0", PeerURLs: []string{"https://10.241.32.26:2380"}},
		{Name: "borovets-multi-master-master-1", PeerURLs: []string{"https://10.241.36.19:2380"}},
		{Name: "borovets-multi-master-master-2", PeerURLs: []string{"https://10.241.44.16:2380"}},
	}
	logger.Info("TEST-ETCD client: [etcd] Announced new etcd member joining to the existing etcd cluster")
	logger.Info("TEST-ETCD client: Updated etcd member list", slog.Any("cluster", cluster))
	/////////////////////////////////////////////////////////////////////

	logger.Info("Creating static Pod manifest during join cluster", slog.String("component", constants.Etcd))

	if err := prepareAndWriteEtcdStaticPod(podManifest, opt, nodeName /*clusterResponse.Members*/, cluster); err != nil {
		return err
	}

	/////UNCOMMENT THIS BLOCK ///////// test etcdPeerAddress ///////////////////////
	//learnerID, err := etcdClient.GetMemberID(etcdPeerAddress)
	// if err != nil {
	// 	return err
	// }
	// _, err = etcdClient.MemberPromote(context.Background(), clusterResponse.Member.ID)
	// if err != nil {
	// 	return err
	// }

	// logger.Info("Waiting for the new etcd member to join the cluster", slog.Duration("timeout", etcdHealthyCheckInterval*etcdHealthyCheckRetries))
	// if _, err := WaitForClusterAvailable(etcdClient, etcdHealthyCheckRetries, etcdHealthyCheckInterval); err != nil {
	// 	return err
	// }
	/////////////////////////////////////////////////////////////////////

	return nil
}

func prepareAndWriteEtcdStaticPod(podManifest []byte, options *options, nodeName string, initialCluster []*etcdserverpb.Member) error {
	if len(initialCluster) > 0 {
		podManifest = addMembersToPodManifest(podManifest, initialCluster)
	}

	if err := writeStaticPodToDisk(podManifest, constants.Etcd, options.ManifestDir); err != nil {
		return err
	}
	logger.Info("podManifest is written to disk")

	return nil
}

func addMembersToPodManifest(podManifest []byte, initialCluster []*etcdserverpb.Member) []byte {
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

func writeStaticPodToDisk(podManifest []byte, componentName, manifestDir string) error {
	if err := os.MkdirAll(manifestDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", manifestDir, err)
	}

	filename := GetStaticPodFilepath(componentName, manifestDir)

	if err := os.WriteFile(filename, podManifest, 0600); err != nil {
		return fmt.Errorf("failed to write static pod manifest file for %q (%q): %w", componentName, filename, err)
	}

	return nil
}
