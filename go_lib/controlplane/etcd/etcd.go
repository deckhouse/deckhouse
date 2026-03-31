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

package etcd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
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

	if err := prepareAndWriteEtcdStaticPod(podManifest, opt, []*etcdserverpb.Member{}); err != nil {
		return err
	}
	return nil
}

func JoinCluster(podManifest []byte, ip string, nodeName string, options ...option) error {
	opt := prepareOptions(options...)

	kubeClient, err := client.MyNewKubernetesClient()
	if err != nil {
		return err
	}

	etcdPeerAddress := GetPeerURL(ip)

	var etcdClient *clientv3.Client

	etcdClient, err = client.New(kubeClient, opt.CertificatesDir)
	if err != nil {
		return err
	}
	//nolint:sloglint
	logger.Info("Adding etcd member", slog.String("etcdPeerAddress", etcdPeerAddress))
	// cluster, err = etcdClient.AddMemberAsLearner(nodeName, etcdPeerAddress)
	clusterResponse, err := etcdClient.MemberAddAsLearner(context.Background(), []string{etcdPeerAddress})
	if err != nil {
		return err
	}

	logger.Info("Creating static Pod manifest during join cluster", slog.String("component", constants.Etcd))

	if err := prepareAndWriteEtcdStaticPod(podManifest, opt, clusterResponse.Members); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), KubernetesAPICallTimeout)
	defer cancel()
	_, err = etcdClient.MemberPromote(ctx, clusterResponse.Member.ID)
	if err != nil {
		return err
	}

	logger.Info("Waiting for the new etcd member to join the cluster", slog.Duration("timeout", etcdHealthyCheckInterval*etcdHealthyCheckRetries))
	if _, err := client.WaitForClusterAvailable(etcdClient, etcdHealthyCheckRetries, etcdHealthyCheckInterval); err != nil {
		return err
	}

	return nil
}

func prepareAndWriteEtcdStaticPod(podManifest []byte, options *options, initialCluster []*etcdserverpb.Member) error {
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
	var endpoints []string //nolint:prealloc
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
