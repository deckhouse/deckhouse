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

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	kubeclient "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var logger = log.Default().Named("etcd")

const (
	etcdHealthyCheckInterval = 5 * time.Second
	etcdHealthyCheckRetries  = 8
	KubernetesAPICallTimeout = 1 * time.Minute
	// Learner must catch up the leader before MemberPromote; on large stores this can take many poll cycles.
	memberPromoteMaxAttempts  = 30
	memberPromoteRetryBackoff = 5 * time.Second
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

	etcdPeerAddress := GetPeerURL(ip)

	var etcdClient *clientv3.Client

	etcdClient, err = client.New(kubeClient, opt.CertificatesDir)
	if err != nil {
		return err
	}

	logger.Info("Adding etcd member", slog.String("etcdPeerAddress", etcdPeerAddress))
	// cluster, err = etcdClient.AddMemberAsLearner(nodeName, etcdPeerAddress)
	clusterResponse, err := etcdClient.MemberAddAsLearner(context.Background(), []string{etcdPeerAddress})
	if err != nil {
		return err
	}

	logger.Info("Creating static Pod manifest during join cluster", slog.String("component", constants.Etcd))

	if err := prepareAndWriteEtcdStaticPod(podManifest, opt, nodeName, clusterResponse.Members); err != nil {
		return err
	}

	memberID := clusterResponse.Member.ID
	for attempt := 1; attempt <= memberPromoteMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), KubernetesAPICallTimeout)
		_, err = etcdClient.MemberPromote(ctx, memberID)
		cancel()
		if err == nil {
			break
		}
		if attempt == memberPromoteMaxAttempts || !isLearnerNotYetSyncedPromoteError(err) {
			return err
		}
		logger.Info("MemberPromote: learner not in sync with leader yet, retrying",
			slog.Uint64("memberID", memberID),
			slog.Int("attempt", attempt),
			slog.Int("maxAttempts", memberPromoteMaxAttempts),
			slog.Duration("nextRetryIn", memberPromoteRetryBackoff))
		time.Sleep(memberPromoteRetryBackoff)
	}

	logger.Info("Waiting for the new etcd member to join the cluster", slog.Duration("timeout", etcdHealthyCheckInterval*etcdHealthyCheckRetries))
	if _, err := client.WaitForClusterAvailable(etcdClient, etcdHealthyCheckRetries, etcdHealthyCheckInterval); err != nil {
		return err
	}

	return nil
}

func isLearnerNotYetSyncedPromoteError(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.FailedPrecondition && strings.Contains(st.Message(), "can only promote a learner member") {
			return true
		}
	}
	return strings.Contains(err.Error(), "can only promote a learner member which is in sync with leader")
}

func prepareAndWriteEtcdStaticPod(podManifest []byte, options *options, nodeName string, initialCluster []*etcdserverpb.Member) error {
	if len(initialCluster) > 0 {
		podManifest = addMembersToPodManifest(podManifest, nodeName, initialCluster)
	}

	if err := writeStaticPodToDisk(podManifest, constants.Etcd, options.ManifestDir); err != nil {
		return err
	}
	logger.Info("podManifest is written to disk")

	return nil
}

func addMembersToPodManifest(podManifest []byte, nodeName string, initialCluster []*etcdserverpb.Member) []byte {
	podManifestString := string(podManifest)
	var endpoints []string
	for _, member := range initialCluster {
		name := member.Name
		// etcd does not assign a name to a member until it starts
		// newly added learners have an empty name - use nodeName instead.
		if name == "" {
			name = nodeName
		}
		endpoints = append(endpoints, fmt.Sprintf("%s=%s", name, member.PeerURLs[0]))
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
