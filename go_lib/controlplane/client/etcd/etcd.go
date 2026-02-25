package etcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	dryrunutil "github.com/deckhouse/deckhouse/go_lib/controlplane/client/dryrun"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/etcdconfig"
	images "github.com/deckhouse/deckhouse/go_lib/controlplane/client/image"
	kubeadmapi "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
	kubeadmutil "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmutil"
	staticpodutil "github.com/deckhouse/deckhouse/go_lib/controlplane/client/staticpod"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	utilsnet "k8s.io/utils/net"
)

const etcdTimeout = 2 * time.Second

const (
	etcdVolumeName           = "etcd-data"
	certsVolumeName          = "etcd-certs"
	etcdHealthyCheckInterval = 5 * time.Second
	etcdHealthyCheckRetries  = 8
)

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
	err := wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, GetActiveTimeouts().EtcdAPICall.Duration,
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
		timeout = GetActiveTimeouts().EtcdAPICall.Duration
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

// Для реализации используем [go.etcd.io/etcd/client/v3](http://go.etcd.io/etcd/client/v3)

/*
Нужно из файла /Users/avborovets/Desktop/kubeadm_cpm/kubernetes/cmd/kubeadm/app/phases/etcd/local.go
придумать структуру EtcdConfig, которая будет получаться из render https://flant.kaiten.ru/space/667673/boards/card/60937405
и используя ее и клиент etcd реализовать эти методы.

Обратить внимание на сертификаты

*/

// EnvVar represents an environment variable present in a Container.
type EnvVar struct {
	v1.EnvVar
}

// GetEtcdPodSpec returns the etcd static Pod actualized to the context of the current configuration
// NB. GetEtcdPodSpec methods holds the information about how kubeadm creates etcd static pod manifests.
func GetEtcdPodSpec(config *etcdconfig.EtcdConfig, endpoint *kubeadmapi.APIEndpoint, nodeName string, initialCluster []Member) v1.Pod {
	pathType := v1.HostPathDirectoryOrCreate
	etcdMounts := map[string]v1.Volume{
		etcdVolumeName:  staticpodutil.NewVolume(etcdVolumeName, config.LocalEtcd.DataDir, &pathType),
		certsVolumeName: staticpodutil.NewVolume(certsVolumeName, config.CertificatesDir+"/etcd", &pathType),
	}
	componentHealthCheckTimeout := GetActiveTimeouts().ControlPlaneComponentHealthCheck

	// probeHostname returns the correct localhost IP address family based on the endpoint AdvertiseAddress
	probeHostname, probePort, probeScheme := staticpodutil.GetEtcdProbeEndpoint(config, utilsnet.IsIPv6String(endpoint.AdvertiseAddress))
	return staticpodutil.ComponentPod(
		v1.Container{
			Name:            constants.Etcd,
			Command:         getEtcdCommand(config, endpoint, nodeName, initialCluster),
			Image:           images.GetEtcdImage(config),
			ImagePullPolicy: v1.PullIfNotPresent,
			// Mount the etcd datadir path read-write so etcd can store data in a more persistent manner
			VolumeMounts: []v1.VolumeMount{
				staticpodutil.NewVolumeMount(etcdVolumeName, config.LocalEtcd.DataDir, false),
				staticpodutil.NewVolumeMount(certsVolumeName, config.CertificatesDir+"/etcd", false),
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
			// The etcd probe endpoints are explained here:
			// https://github.com/kubernetes/kubeadm/issues/3039
			LivenessProbe:  staticpodutil.LivenessProbe(probeHostname, "/livez", constants.ProbePort, probeScheme),
			ReadinessProbe: staticpodutil.ReadinessProbe(probeHostname, "/readyz", constants.ProbePort, probeScheme),
			StartupProbe:   staticpodutil.StartupProbe(probeHostname, "/readyz", constants.ProbePort, probeScheme, componentHealthCheckTimeout),
			Env:            kubeadmutil.MergeKubeadmEnvVars(config.LocalEtcd.ExtraEnvs),
			Ports: []v1.ContainerPort{
				{
					Name:          constants.ProbePort,
					ContainerPort: probePort,
					Protocol:      v1.ProtocolTCP,
				},
			},
		},
		etcdMounts,
		// etcd will listen on the advertise address of the API server, in a different port (2379)
		map[string]string{constants.EtcdAdvertiseClientUrlsAnnotationKey: GetClientURL(endpoint)},
	)
}

// getEtcdCommand builds the right etcd command from the given config object
func getEtcdCommand(config *etcdconfig.EtcdConfig, endpoint *kubeadmapi.APIEndpoint, nodeName string, initialCluster []Member) []string {
	// localhost IP family should be the same that the AdvertiseAddress
	etcdLocalhostAddress := "127.0.0.1"
	if utilsnet.IsIPv6String(endpoint.AdvertiseAddress) {
		etcdLocalhostAddress = "::1"
	}
	defaultArguments := []kubeadmapi.Arg{
		{Name: "name", Value: nodeName},
		{Name: "listen-client-urls", Value: fmt.Sprintf("%s,%s", GetClientURLByIP(etcdLocalhostAddress), GetClientURL(endpoint))},
		{Name: "advertise-client-urls", Value: GetClientURL(endpoint)},
		{Name: "listen-peer-urls", Value: GetPeerURL(endpoint)},
		{Name: "initial-advertise-peer-urls", Value: GetPeerURL(endpoint)},
		{Name: "data-dir", Value: config.LocalEtcd.DataDir},
		{Name: "cert-file", Value: filepath.Join(config.CertificatesDir, constants.EtcdServerCertName)},
		{Name: "key-file", Value: filepath.Join(config.CertificatesDir, constants.EtcdServerKeyName)},
		{Name: "trusted-ca-file", Value: filepath.Join(config.CertificatesDir, constants.EtcdCACertName)},
		{Name: "client-cert-auth", Value: "true"},
		{Name: "peer-cert-file", Value: filepath.Join(config.CertificatesDir, constants.EtcdPeerCertName)},
		{Name: "peer-key-file", Value: filepath.Join(config.CertificatesDir, constants.EtcdPeerKeyName)},
		{Name: "peer-trusted-ca-file", Value: filepath.Join(config.CertificatesDir, constants.EtcdCACertName)},
		{Name: "peer-client-cert-auth", Value: "true"},
		{Name: "snapshot-count", Value: "10000"},
		{Name: "listen-metrics-urls", Value: fmt.Sprintf("http://%s", net.JoinHostPort(etcdLocalhostAddress, strconv.Itoa(constants.EtcdMetricsPort)))},
	}

	etcdImageTag := images.GetEtcdImageTag(config)
	if etcdVersion, err := version.ParseSemantic(etcdImageTag); err == nil && etcdVersion.AtLeast(version.MustParseSemantic("3.6.0")) {
		// Arguments used by Etcd 3.6.0+.
		// TODO: Start always using these once kubeadm only supports etcd >= 3.6.0 for all its supported k8s versions.
		defaultArguments = append(defaultArguments, []kubeadmapi.Arg{
			{Name: "feature-gates", Value: "InitialCorruptCheck=true"},
			{Name: "watch-progress-notify-interval", Value: "5s"},
		}...)
	} else {
		defaultArguments = append(defaultArguments, []kubeadmapi.Arg{
			{Name: "experimental-initial-corrupt-check", Value: "true"},
			{Name: "experimental-watch-progress-notify-interval", Value: "5s"},
		}...)
	}

	if len(initialCluster) == 0 {
		defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "initial-cluster", fmt.Sprintf("%s=%s", nodeName, GetPeerURL(endpoint)), 1)
	} else {
		// NB. the joining etcd member should be part of the initialCluster list
		endpoints := []string{}
		for _, member := range initialCluster {
			endpoints = append(endpoints, fmt.Sprintf("%s=%s", member.Name, member.PeerURL))
		}

		defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "initial-cluster", strings.Join(endpoints, ","), 1)
		defaultArguments = kubeadmapi.SetArgValues(defaultArguments, "initial-cluster-state", "existing", 1)
	}

	command := []string{"etcd"}
	command = append(command, kubeadmutil.ArgumentsToCommand(defaultArguments, config.LocalEtcd.ExtraArgs)...)
	return command
}

func WriteStaticPodToDisk(componentName, manifestDir string, pod v1.Pod) error {

	// creates target folder if not already exists
	if err := os.MkdirAll(manifestDir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", manifestDir)
	}

	// writes the pod to disk
	serialized, err := kubeadmutil.MarshalToYaml(&pod, v1.SchemeGroupVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal manifest for %q to YAML", componentName)
	}

	filename := constants.GetStaticPodFilepath(componentName, manifestDir)

	if err := os.WriteFile(filename, serialized, 0600); err != nil {
		return errors.Wrapf(err, "failed to write static pod manifest file for %q (%q)", componentName, filename)
	}

	return nil
}

func prepareAndWriteEtcdStaticPod(manifestDir string, patchesDir string, cfg *etcdconfig.EtcdConfig, endpoint *kubeadmapi.APIEndpoint, nodeName string, initialCluster []Member, isDryRun bool) error {
	// gets etcd StaticPodSpec, actualized for the current ClusterConfiguration and the new list of etcd members
	spec := GetEtcdPodSpec(cfg, endpoint, nodeName, initialCluster)

	// writes etcd StaticPod to disk
	if err := WriteStaticPodToDisk(constants.Etcd, manifestDir, spec); err != nil {
		return err
	}

	// If dry-running, print the static etcd pod manifest file.
	if isDryRun {
		realPath := constants.GetStaticPodFilepath(constants.Etcd, manifestDir)
		outputPath := constants.GetStaticPodFilepath(constants.Etcd, constants.GetStaticPodDirectory())
		return dryrunutil.PrintDryRunFiles([]dryrunutil.FileToPrint{dryrunutil.NewFileToPrint(realPath, outputPath)}, os.Stdout)
	}
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

// func (c *EtcdClient) InitCluster(config *EtcdConfig, manifestDir, patchesDir string, nodeName string, cfg *kubeadmapi.ClusterConfiguration, endpoint *kubeadmapi.APIEndpoint) error {

// 	if err := prepareAndWriteEtcdStaticPod(config, patchesDir, cfg, endpoint, nodeName, []etcdutil.Member{}, isDryRun); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (c *EtcdClient) JoinCluster(memberName string, peerURL string) error {
// 	return nil
// }
