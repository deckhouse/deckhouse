// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	libcon "github.com/deckhouse/lib-connection/pkg"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
)

// errEtcdMemberCheckTransient marks a failure to observe etcd membership that may clear up on
// retry (pod not scheduled yet, exec/network hiccup), as opposed to a permanent structural
// failure (e.g. etcdctl output that doesn't parse) that will fail identically every attempt.
var errEtcdMemberCheckTransient = fmt.Errorf("etcd member check: transient error, may succeed on retry")

// errEtcdNotExpectedMembership marks the expected "still converging" condition (node not a
// member yet / still a member), as opposed to a genuine check failure.
var errEtcdNotExpectedMembership = fmt.Errorf("etcd membership: not yet in the expected state")

// errEtcdClusterIsNotHealthy marks an endpoint that did not answer the health call. Losing a
// member costs a leader election, so it is retried before it is believed.
var errEtcdClusterIsNotHealthy = fmt.Errorf("etcd cluster is not healthy")

func waitEtcdHasMember(ctx context.Context, kubeGetter kubernetes.KubeClientProviderWithCtx, nodeName string) error {
	loopParams := retry.NewEmptyParams(
		retry.WithName("Waiting for '%s' to join etcd", nodeName),
		retry.WithAttempts(2000),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(errEtcdMemberCheckTransient, errEtcdNotExpectedMembership),
	)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		// Fresh client each attempt: the captured tunnel dies on master replace.
		kc, err := kubeGetter.KubeClientCtx(ctx)
		if err != nil {
			return fmt.Errorf("get kube client: %w", err)
		}
		client := kc.KubeClient.(libcon.KubeClient)

		members, err := getEtcdMembers(ctx, client, "")
		if err != nil {
			return fmt.Errorf("getting etcd members: %w", err)
		}

		names := make([]string, 0, len(members))
		for _, m := range members {
			names = append(names, m.Name)
		}

		voting := hasVotingMember(members, nodeName)

		if voting {
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Current members: [%s]", strings.Join(names, ", ")))
			return nil
		}

		return fmt.Errorf("%w: '%s' is not yet a voting member", errEtcdNotExpectedMembership, nodeName)
	})
}

func waitEtcdHasNoMember(ctx context.Context, kubeGetter kubernetes.KubeClientProviderWithCtx, nodeName string) error {
	loopParams := retry.NewEmptyParams(
		retry.WithName("Waiting for '%s' to leave etcd", nodeName),
		retry.WithAttempts(225),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(errEtcdMemberCheckTransient, errEtcdNotExpectedMembership),
	)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		fieldSelector := fields.OneTermNotEqualSelector("spec.nodeName", nodeName).String()

		kc, err := kubeGetter.KubeClientCtx(ctx)
		if err != nil {
			return fmt.Errorf("get kube client: %w", err)
		}
		client := kc.KubeClient.(libcon.KubeClient)

		ok, err := isEtcdHasMember(ctx, client, nodeName, fieldSelector)
		if err != nil {
			return fmt.Errorf("checking etcd membership for '%s': %w", nodeName, err)
		}

		if ok {
			return fmt.Errorf("%w: node '%s' is still listed as etcd cluster member", errEtcdNotExpectedMembership, nodeName)
		}

		return nil
	})
}

// checkEtcdQuorumBeforeRemoval checks the remaining voting members against endpoint
// health and requires every remaining endpoint to be healthy before removal.
func checkEtcdQuorumBeforeRemoval(ctx context.Context, kubeGetter kubernetes.KubeClientProviderWithCtx, nodeToDestroy string) error {
	loopParams := retry.NewEmptyParams(
		retry.WithName("Check etcd quorum without '%s'", nodeToDestroy),
		retry.WithAttempts(30),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(errEtcdMemberCheckTransient, errEtcdClusterIsNotHealthy),
	)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		kc, err := kubeGetter.KubeClientCtx(ctx)
		if err != nil {
			return fmt.Errorf("get kube client: %w", err)
		}

		client, ok := kc.KubeClient.(libcon.KubeClient)
		if !ok {
			return fmt.Errorf("kube client cannot exec into an etcd pod")
		}

		fieldSelector := fields.OneTermNotEqualSelector("spec.nodeName", nodeToDestroy).String()
		members, err := getEtcdMembers(ctx, client, fieldSelector)
		if err != nil {
			return fmt.Errorf("getting etcd members: %w", err)
		}

		endpoints, err := getEtcdEndpointsHealth(ctx, client, fieldSelector)
		if err != nil {
			return err
		}

		voting, healthy := etcdQuorumBeforeRemoval(members, endpoints, nodeToDestroy)

		quorum := voting/2 + 1
		if healthy < quorum {
			return fmt.Errorf(
				"%w: removing '%s' leaves %d voting etcd members with only %d healthy members, quorum needs %d",
				errEtcdClusterIsNotHealthy, nodeToDestroy, voting, healthy, quorum)
		}

		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
			"Removing '%s' leaves %d voting etcd members, quorum %d, %d healthy members",
			nodeToDestroy, voting, quorum, healthy))

		return nil
	})
}

// checkEtcdClusterHealthy answers whether the cluster serves, which a member list does not:
// etcd answers that one from the local member alone, while the health call each endpoint
// runs needs a quorum behind it. The endpoints of skippedNode are not required to answer —
// before its removal that member may be exactly what is broken, and after it there is
// nothing left to ask.
func checkEtcdClusterHealthy(ctx context.Context, kubeGetter kubernetes.KubeClientProviderWithCtx, skippedNode string) error {
	loopParams := retry.NewEmptyParams(
		retry.WithName("Check etcd cluster health without '%s'", skippedNode),
		retry.WithAttempts(30),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(errEtcdMemberCheckTransient, errEtcdClusterIsNotHealthy),
	)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		kc, err := kubeGetter.KubeClientCtx(ctx)
		if err != nil {
			return fmt.Errorf("get kube client: %w", err)
		}

		client, ok := kc.KubeClient.(libcon.KubeClient)
		if !ok {
			return fmt.Errorf("kube client cannot exec into an etcd pod")
		}

		fieldSelector := fields.OneTermNotEqualSelector("spec.nodeName", skippedNode).String()

		members, err := getEtcdMembers(ctx, client, fieldSelector)
		if err != nil {
			return fmt.Errorf("getting etcd members: %w", err)
		}

		endpoints, err := getEtcdEndpointsHealth(ctx, client, fieldSelector)
		if err != nil {
			return err
		}

		return checkEtcdEndpointsHealthy(ctx, members, endpoints, skippedNode)
	})
}

func checkEtcdEndpointsHealthy(ctx context.Context, members []etcdMember, endpoints []endpointHealth, skippedNode string) error {
	unhealthy, checked := unhealthyEndpoints(endpoints, memberEndpoints(members, skippedNode))
	if len(unhealthy) > 0 {
		return fmt.Errorf("%w: %s", errEtcdClusterIsNotHealthy, strings.Join(unhealthy, "; "))
	}

	// An empty report proves nothing, and silence must not pass for health here.
	if checked == 0 {
		return fmt.Errorf("%w: no endpoint answered the health call", errEtcdClusterIsNotHealthy)
	}

	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Etcd cluster is healthy on all %d endpoints", checked))

	return nil
}

func getEtcdMembers(ctx context.Context, client libcon.KubeClient, fieldSelector string) ([]etcdMember, error) {
	pod, err := runningEtcdPod(ctx, client, fieldSelector)
	if err != nil {
		return nil, err
	}

	command := append(etcdctlCommand(), "member", "list", "-w", "json")

	var stdout bytes.Buffer

	params := libcon.PodExecParams{
		Namespace: "kube-system",
		Name:      pod.Name,
		Command:   command,
		Container: "etcd",
		Stdout:    &stdout,
	}

	err = client.Exec(ctx, &params)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errEtcdMemberCheckTransient, err)
	}

	var members memberListOutput
	if err = json.Unmarshal(stdout.Bytes(), &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal etcd member list: %w", err)
	}

	return members.Members, nil
}

func getEtcdEndpointsHealth(ctx context.Context, client libcon.KubeClient, fieldSelector string) ([]endpointHealth, error) {
	pod, err := runningEtcdPod(ctx, client, fieldSelector)
	if err != nil {
		return nil, err
	}

	var stdout bytes.Buffer

	params := libcon.PodExecParams{
		Namespace: "kube-system",
		Name:      pod.Name,
		Command:   append(etcdctlCommand(), "endpoint", "health", "--cluster", "-w", "json"),
		Container: "etcd",
		Stdout:    &stdout,
	}

	// A failing endpoint makes etcdctl exit non-zero while still reporting every endpoint
	// on stdout, so the report is read first and the exit code only speaks when it is empty.
	execErr := client.Exec(ctx, &params)

	var endpoints []endpointHealth
	if unmarshalErr := json.Unmarshal(stdout.Bytes(), &endpoints); unmarshalErr != nil {
		if execErr != nil {
			return nil, fmt.Errorf("%w: %w", errEtcdMemberCheckTransient, execErr)
		}

		return nil, fmt.Errorf("failed to unmarshal etcd endpoint health: %w", unmarshalErr)
	}

	return endpoints, nil
}

func isEtcdHasMember(ctx context.Context, client libcon.KubeClient, nodeName, fieldSelector string) (bool, error) {
	members, err := getEtcdMembers(ctx, client, fieldSelector)
	if err != nil {
		return false, err
	}

	for _, m := range members {
		if m.Name == nodeName {
			return true, nil
		}
	}

	return false, nil
}

func runningEtcdPod(ctx context.Context, client libcon.KubeClient, fieldSelector string) (*corev1.Pod, error) {
	pods, err := client.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{
		LabelSelector: "component=etcd,tier=control-plane",
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get etcd pods: %w", errEtcdMemberCheckTransient, err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("%w: etcd pods not found", errEtcdMemberCheckTransient)
	}

	for i := range pods.Items {
		for _, cs := range pods.Items[i].Status.ContainerStatuses {
			if cs.Name == "etcd" && cs.State.Running != nil {
				return &pods.Items[i], nil
			}
		}
	}

	return nil, fmt.Errorf("%w: no etcd pod with running container found", errEtcdMemberCheckTransient)
}

func etcdctlCommand() []string {
	return []string{
		"etcdctl",
		"--cacert", "/etc/kubernetes/pki/etcd/ca.crt",
		"--cert", "/etc/kubernetes/pki/etcd/ca.crt",
		"--key", "/etc/kubernetes/pki/etcd/ca.key",
		"--endpoints", "https://127.0.0.1:2379/",
	}
}

// etcdQuorumBeforeRemoval counts remaining voters and those with a healthy client
// endpoint. Multiple client URLs belonging to one member contribute only one vote.
//
//nolint:nonamedreturns
func etcdQuorumBeforeRemoval(members []etcdMember, endpoints []endpointHealth, nodeToDestroy string) (voting, healthy int) {
	healthyEndpoints := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Health {
			healthyEndpoints[strings.TrimSuffix(endpoint.Endpoint, "/")] = struct{}{}
		}
	}

	for _, member := range members {
		if member.Name == nodeToDestroy || member.IsLearner {
			continue
		}
		voting++
		for _, url := range member.ClientURLs {
			if _, ok := healthyEndpoints[strings.TrimSuffix(url, "/")]; ok {
				healthy++
				break
			}
		}
	}
	return voting, healthy
}

// memberEndpoints collects the client URLs etcd serves nodeName on, so the health call can
// stop requiring an answer from the member that is leaving.
func memberEndpoints(members []etcdMember, nodeName string) map[string]struct{} {
	endpoints := make(map[string]struct{})

	for _, m := range members {
		if m.Name != nodeName {
			continue
		}

		for _, url := range m.ClientURLs {
			endpoints[strings.TrimSuffix(url, "/")] = struct{}{}
		}
	}

	return endpoints
}

// unhealthyEndpoints names the endpoints `etcdctl endpoint health --cluster` refused to
// call healthy, with the reason it gave for each, and reports how many endpoints were
// required to answer at all.
//
//nolint:nonamedreturns
func unhealthyEndpoints(endpoints []endpointHealth, ignored map[string]struct{}) (unhealthy []string, checked int) {
	unhealthy = make([]string, 0, len(endpoints))

	for _, e := range endpoints {
		if _, ok := ignored[strings.TrimSuffix(e.Endpoint, "/")]; ok {
			continue
		}

		checked++

		if e.Health {
			continue
		}

		reason := e.Error
		if reason == "" {
			reason = "unhealthy"
		}

		unhealthy = append(unhealthy, fmt.Sprintf("%s: %s", e.Endpoint, reason))
	}

	return unhealthy, checked
}

// hasVotingMember answers for the name, not for the first entry carrying it: a
// recreated master can be listed twice, as the stale member and as the learner
// rejoining, and either of them being a learner means the node is not back yet.
func hasVotingMember(members []etcdMember, nodeName string) bool {
	found := false

	for _, m := range members {
		if m.Name != nodeName {
			continue
		}

		if m.IsLearner {
			return false
		}

		found = true
	}

	return found
}

type memberListOutput struct {
	Members []etcdMember `json:"members"`
}

type etcdMember struct {
	Name       string   `json:"name"`
	ClientURLs []string `json:"clientURLs"`
	// A learner replicates the log but does not vote, so a master that came back
	// as one does not restore the quorum the next master replace will spend.
	IsLearner bool `json:"isLearner"`
}

type endpointHealth struct {
	Endpoint string `json:"endpoint"`
	Health   bool   `json:"health"`
	Error    string `json:"error"`
}
