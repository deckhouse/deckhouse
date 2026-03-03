package etcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	kubeadmapp "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	kubeadmapi "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

type EtcdConfig struct {
	ManifestDir     string
	CertificatesDir string
}

var logger = log.Default().Named("etcd")

const etcdTimeout = 2 * time.Second

const (
	etcdHealthyCheckInterval = 5 * time.Second
	etcdHealthyCheckRetries  = 8
)

var ErrNoMemberIDForPeerURL = errors.New("no member id found for peer URL")

type etcdClient interface {
	// Close shuts down the client's etcd connections.
	Close() error

	// Endpoints lists the registered endpoints for the client.
	Endpoints() []string

	// MemberList lists the current cluster membership.
	MemberList(ctx context.Context) (*clientv3.MemberListResponse, error)

	// MemberAdd adds a new member into the cluster.
	MemberAdd(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error)

	// MemberAddAsLearner adds a new learner member into the cluster.
	MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error)

	// MemberRemove removes an existing member from the cluster.
	MemberRemove(ctx context.Context, id uint64) (*clientv3.MemberRemoveResponse, error)

	// MemberPromote promotes a member from raft learner (non-voting) to raft voting member.
	MemberPromote(ctx context.Context, id uint64) (*clientv3.MemberPromoteResponse, error)

	// Status gets the status of the endpoint.
	Status(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error)

	// Sync synchronizes client's endpoints with the known endpoints from the etcd membership.
	Sync(ctx context.Context) error
}

type Client struct {
	Endpoints []string

	newEtcdClient func(endpoints []string) (etcdClient, error)

	listMembersFunc func(timeout time.Duration) (*clientv3.MemberListResponse, error)
}

// New creates a new EtcdCluster client
func New(endpoints []string, ca, cert, key string) (*Client, error) {
	client := Client{Endpoints: endpoints}

	var err error
	var tlsConfig *tls.Config
	if ca != "" || cert != "" || key != "" {
		tlsInfo := transport.TLSInfo{
			CertFile:      cert,
			KeyFile:       key,
			TrustedCAFile: ca,
		}
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
	}

	client.newEtcdClient = func(endpoints []string) (etcdClient, error) {
		return clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: etcdTimeout,
			DialOptions: []grpc.DialOption{
				grpc.WithBlock(), // block until the underlying connection is up
			},
			TLS: tlsConfig,
		})
	}

	client.listMembersFunc = client.listMembers

	return &client, nil
}

// NewFromCluster creates an etcd client for the etcd endpoints present in etcd member list. In order to compose this information,
// it will first discover at least one etcd endpoint to connect to. Once created, the client synchronizes client's endpoints with
// the known endpoints from the etcd membership API, since it is the authoritative source of truth for the list of available members.
func NewFromCluster(client clientset.Interface, certificatesDir string) (*Client, error) {
	// Discover at least one etcd endpoint to connect to by inspecting the existing etcd pods

	// Get the list of etcd endpoints
	endpoints, err := getEtcdEndpoints(client)
	if err != nil {
		return nil, err
	}
	klog.V(1).Infof("etcd endpoints read from pods: %s", strings.Join(endpoints, ","))

	// Creates an etcd client
	etcdClient, err := New(
		endpoints,
		filepath.Join(certificatesDir, constants.EtcdCACertName),
		filepath.Join(certificatesDir, constants.EtcdHealthcheckClientCertName),
		filepath.Join(certificatesDir, constants.EtcdHealthcheckClientKeyName),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating etcd client for %v endpoints", endpoints)
	}

	// synchronizes client's endpoints with the known endpoints from the etcd membership.
	err = etcdClient.Sync()
	if err != nil {
		return nil, errors.Wrap(err, "error syncing endpoints with etcd")
	}
	klog.V(1).Infof("update etcd endpoints: %s", strings.Join(etcdClient.Endpoints, ","))

	return etcdClient, nil
}

func (c *Client) Sync() error {
	// Syncs the list of endpoints
	var cli etcdClient
	var lastError error
	err := wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, kubeadmapi.GetActiveTimeouts().EtcdAPICall.Duration,
		true, func(_ context.Context) (bool, error) {
			var err error
			cli, err = c.newEtcdClient(c.Endpoints)
			if err != nil {
				lastError = err
				return false, nil
			}
			defer func() { _ = cli.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
			err = cli.Sync(ctx)
			cancel()
			if err == nil {
				return true, nil
			}
			klog.V(5).Infof("Failed to sync etcd endpoints: %v", err)
			lastError = err
			return false, nil
		})
	if err != nil {
		return lastError
	}
	klog.V(1).Infof("etcd endpoints read from etcd: %s", strings.Join(cli.Endpoints(), ","))

	c.Endpoints = cli.Endpoints()
	return nil
}

func (c *Client) listMembers(timeout time.Duration) (*clientv3.MemberListResponse, error) {
	// Gets the member list
	var lastError error
	var resp *clientv3.MemberListResponse
	if timeout == 0 {
		timeout = kubeadmapi.GetActiveTimeouts().EtcdAPICall.Duration
	}
	err := wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, timeout,
		true, func(_ context.Context) (bool, error) {
			cli, err := c.newEtcdClient(c.Endpoints)
			if err != nil {
				lastError = err
				return false, nil
			}
			defer func() { _ = cli.Close() }()

			ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
			resp, err = cli.MemberList(ctx)
			cancel()
			if err == nil {
				return true, nil
			}
			klog.V(5).Infof("Failed to get etcd member list: %v", err)
			lastError = err
			return false, nil
		})
	if err != nil {
		return nil, lastError
	}
	return resp, nil
}

// addMember notifies an existing etcd cluster that a new member is joining, and
// return the updated list of members. If the member has already been added to the
// cluster, this will return the existing list of etcd members.
func (c *Client) addMember(name string, peerAddrs string, isLearner bool) ([]Member, error) {
	// Parse the peer address, required to add the client URL later to the list
	// of endpoints for this client. Parsing as a first operation to make sure that
	// if this fails no member addition is performed on the etcd cluster.
	parsedPeerAddrs, err := url.Parse(peerAddrs)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing peer address %s", peerAddrs)
	}

	cli, err := c.newEtcdClient(c.Endpoints)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cli.Close() }()

	// Adds a new member to the cluster
	var (
		lastError   error
		respMembers []*etcdserverpb.Member
		resp        *clientv3.MemberAddResponse
	)
	err = wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, kubeadmapi.GetActiveTimeouts().EtcdAPICall.Duration,
		true, func(_ context.Context) (bool, error) {
			ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
			defer cancel()

			// List members and quickly return if the member already exists.
			listResp, err := cli.MemberList(ctx)
			if err != nil {
				klog.V(5).Infof("Failed to check whether the member %q exists: %v", peerAddrs, err)
				lastError = err
				return false, nil
			}
			found := false
			for _, member := range listResp.Members {
				if member.GetPeerURLs()[0] == peerAddrs {
					found = true
					break
				}
			}
			if found {
				klog.V(5).Infof("The peer URL %q for the added etcd member already exists. Skipping etcd member addition", peerAddrs)
				respMembers = listResp.Members
				return true, nil
			}

			if isLearner {
				klog.V(1).Infof("[etcd] Adding etcd member %q as learner", peerAddrs)
				resp, err = cli.MemberAddAsLearner(ctx, []string{peerAddrs})
			} else {
				klog.V(1).Infof("[etcd] Adding etcd member %q", peerAddrs)
				resp, err = cli.MemberAdd(ctx, []string{peerAddrs})
			}
			if err == nil {
				respMembers = resp.Members
				return true, nil
			}

			// If the error indicates that the peer already exists, exit early. In this situation, resp is nil, so
			// call out to MemberList to fetch all the members before returning.
			if errors.Is(err, rpctypes.ErrPeerURLExist) {
				klog.V(5).Info("The peer URL for the added etcd member already exists. Fetching the existing etcd members")
				listResp, err = cli.MemberList(ctx)
				if err == nil {
					respMembers = listResp.Members
					return true, nil
				}
			}

			klog.V(5).Infof("Failed to add etcd member: %v", err)
			lastError = err
			return false, nil
		})
	if err != nil {
		return nil, lastError
	}

	// Returns the updated list of etcd members
	ret := []Member{}
	for _, m := range respMembers {
		// If the peer address matches, this is the member we are adding.
		// Use the name we passed to the function.
		if peerAddrs == m.PeerURLs[0] {
			ret = append(ret, Member{Name: name, PeerURL: peerAddrs})
			continue
		}
		// Otherwise, we are processing other existing etcd members returned by AddMembers.
		memberName := m.Name
		// In some cases during concurrent join, some members can end up without a name.
		// Use the member ID as name for those.
		if len(memberName) == 0 {
			memberName = strconv.FormatUint(m.ID, 16)
		}
		ret = append(ret, Member{Name: memberName, PeerURL: m.PeerURLs[0]})
	}

	// Add the new member client address to the list of endpoints
	c.Endpoints = append(c.Endpoints, GetClientURLByIP(parsedPeerAddrs.Hostname()))

	return ret, nil
}

// AddMember adds a new member into the etcd cluster
func (c *Client) AddMember(name string, peerAddrs string) ([]Member, error) {
	return c.addMember(name, peerAddrs, false)
}

// AddMemberAsLearner adds a new learner member into the etcd cluster.
func (c *Client) AddMemberAsLearner(name string, peerAddrs string) ([]Member, error) {
	return c.addMember(name, peerAddrs, true)
}

// GetMemberID returns the member ID of the given peer URL
func (c *Client) GetMemberID(peerURL string) (uint64, error) {
	resp, err := c.listMembersFunc(0)
	if err != nil {
		return 0, err
	}

	for _, member := range resp.Members {
		if member.GetPeerURLs()[0] == peerURL {
			return member.GetID(), nil
		}
	}
	return 0, ErrNoMemberIDForPeerURL
}

// MemberPromote promotes a member as a voting member. If the given member ID is already a voting member this method
// will return early and do nothing.
func (c *Client) MemberPromote(learnerID uint64) error {
	isLearner, err := c.isLearner(learnerID)
	if err != nil {
		return err
	}
	if !isLearner {
		klog.V(1).Infof("[etcd] Member %s already promoted.", strconv.FormatUint(learnerID, 16))
		return nil
	}

	klog.V(1).Infof("[etcd] Promoting a learner as a voting member: %s", strconv.FormatUint(learnerID, 16))
	cli, err := c.newEtcdClient(c.Endpoints)
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	// TODO: warning logs from etcd client should be removed.
	// The warning logs are printed by etcd client code for several reasons, including
	// 1. can not promote yet(no synced)
	// 2. context deadline exceeded
	// 3. peer URLs already exists
	// Once the client provides a way to check if the etcd learner is ready to promote, the retry logic can be revisited.
	var (
		lastError error
	)
	err = wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, kubeadmapi.GetActiveTimeouts().EtcdAPICall.Duration,
		true, func(_ context.Context) (bool, error) {
			ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
			defer cancel()

			isLearner, err := c.isLearner(learnerID)
			if err != nil {
				return false, err
			}
			if !isLearner {
				klog.V(1).Infof("[etcd] Member %s was already promoted.", strconv.FormatUint(learnerID, 16))
				return true, nil
			}

			_, err = cli.MemberPromote(ctx, learnerID)
			if err == nil {
				klog.V(1).Infof("[etcd] The learner was promoted as a voting member: %s", strconv.FormatUint(learnerID, 16))
				return true, nil
			}
			klog.V(5).Infof("[etcd] Promoting the learner %s failed: %v", strconv.FormatUint(learnerID, 16), err)
			lastError = err
			return false, nil
		})
	if err != nil {
		return lastError
	}
	return nil
}

// isLearner returns true if the given member ID is a learner.
func (c *Client) isLearner(memberID uint64) (bool, error) {
	resp, err := c.listMembersFunc(0)
	if err != nil {
		return false, err
	}

	for _, member := range resp.Members {
		if member.ID == memberID && member.IsLearner {
			return true, nil
		}
	}
	return false, nil
}

// WaitForClusterAvailable returns true if all endpoints in the cluster are available after retry attempts, an error is returned otherwise
func (c *Client) WaitForClusterAvailable(retries int, retryInterval time.Duration) (bool, error) {
	for i := 0; i < retries; i++ {
		if i > 0 {
			klog.V(1).Infof("[etcd] Waiting %v until next retry\n", retryInterval)
			time.Sleep(retryInterval)
		}
		klog.V(2).Infof("[etcd] attempting to see if all cluster endpoints (%s) are available %d/%d", c.Endpoints, i+1, retries)
		_, err := c.getClusterStatus()
		if err != nil {
			switch err {
			case context.DeadlineExceeded:
				klog.V(1).Infof("[etcd] Attempt timed out")
			default:
				klog.V(1).Infof("[etcd] Attempt failed with error: %v\n", err)
			}
			continue
		}
		return true, nil
	}
	return false, errors.New("timeout waiting for etcd cluster to be available")
}

// getClusterStatus returns nil for status Up (along with endpoint status response map) or error for status Down
func (c *Client) getClusterStatus() (map[string]*clientv3.StatusResponse, error) {
	clusterStatus := make(map[string]*clientv3.StatusResponse)
	for _, ep := range c.Endpoints {
		// Gets the member status
		var lastError error
		var resp *clientv3.StatusResponse
		err := wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, kubeadmapi.GetActiveTimeouts().EtcdAPICall.Duration,
			true, func(_ context.Context) (bool, error) {
				cli, err := c.newEtcdClient(c.Endpoints)
				if err != nil {
					lastError = err
					return false, nil
				}
				defer func() { _ = cli.Close() }()

				ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
				resp, err = cli.Status(ctx, ep)
				cancel()
				if err == nil {
					return true, nil
				}
				klog.V(5).Infof("Failed to get etcd status for %s: %v", ep, err)
				lastError = err
				return false, nil
			})
		if err != nil {
			return nil, lastError
		}

		clusterStatus[ep] = resp
	}
	return clusterStatus, nil
}

// Для реализации используем [go.etcd.io/etcd/client/v3](http://go.etcd.io/etcd/client/v3)

/*
Нужно из файла /Users/avborovets/Desktop/kubeadm_cpm/kubernetes/cmd/kubeadm/app/phases/etcd/local.go
придумать структуру EtcdConfig, которая будет получаться из render https://flant.kaiten.ru/space/667673/boards/card/60937405
и используя ее и клиент etcd реализовать эти методы.

Обратить внимание на сертификаты

*/

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

	config = &EtcdConfig{
		ManifestDir:     "/etc/kubernetes/manifests_mytest",
		CertificatesDir: "/etc/kubernetes/pki",
	}

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

	config = &EtcdConfig{
		ManifestDir:     "/etc/kubernetes/manifests_mytest_join",
		CertificatesDir: "/etc/kubernetes/pki",
	}

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
