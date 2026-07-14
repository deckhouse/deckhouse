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

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func waitEtcdHasMember(ctx context.Context, kubeGetter kubernetes.KubeClientProviderWithCtx, nodeName string) error {
	"github.com/deckhouse/lib-dhctl/pkg/retry"
)

// errEtcdMemberCheckTransient marks a failure to observe etcd membership that may clear up on
// retry (pod not scheduled yet, exec/network hiccup), as opposed to a permanent structural
// failure (e.g. etcdctl output that doesn't parse) that will fail identically every attempt.
var errEtcdMemberCheckTransient = fmt.Errorf("etcd member check: transient error, may succeed on retry")

// errEtcdNotExpectedMembership marks the expected "still converging" condition (node not a
// member yet / still a member), as opposed to a genuine check failure.
var errEtcdNotExpectedMembership = fmt.Errorf("etcd membership: not yet in the expected state")

func waitEtcdHasMember(ctx context.Context, client libcon.KubeClient, nodeName string) error {
	attempt := 0

	loopParams := retry.NewEmptyParams(
		retry.WithName("Waiting for '%s' to join etcd", nodeName),
		retry.WithAttempts(2000),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(errEtcdMemberCheckTransient, errEtcdNotExpectedMembership),
	)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		attempt++

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
		hasMember := false
		for _, m := range members {
			names = append(names, m.Name)
			if m.Name == nodeName {
				hasMember = true
			}
		}

		if attempt == 1 || hasMember {
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Current members: [%s]", strings.Join(names, ", ")))
		}

		if hasMember {
			return nil
		}

		return fmt.Errorf("%w: '%s' is not yet a member", errEtcdNotExpectedMembership, nodeName)
	})
}

func waitEtcdHasNoMember(ctx context.Context, kubeGetter kubernetes.KubeClientProviderWithCtx, nodeName string) error {
	const maxAttempts = 225

	loopParams := retry.NewEmptyParams(
		retry.WithName("Waiting for '%s' to leave etcd", nodeName),
		retry.WithAttempts(maxAttempts),
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

func getEtcdMembers(ctx context.Context, client libcon.KubeClient, fieldSelector string) ([]etcdMember, error) {
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

	var pod *corev1.Pod
	for i := range pods.Items {
		for _, cs := range pods.Items[i].Status.ContainerStatuses {
			if cs.Name == "etcd" && cs.State.Running != nil {
				pod = &pods.Items[i]
				break
			}
		}
		if pod != nil {
			break
		}
	}
	if pod == nil {
		return nil, fmt.Errorf("%w: no etcd pod with running container found", errEtcdMemberCheckTransient)
	}

	command := []string{
		"etcdctl",
		"--cacert", "/etc/kubernetes/pki/etcd/ca.crt",
		"--cert", "/etc/kubernetes/pki/etcd/ca.crt",
		"--key", "/etc/kubernetes/pki/etcd/ca.key",
		"--endpoints", "https://127.0.0.1:2379/",
		"member", "list", "-w", "json",
	}

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

type memberListOutput struct {
	Members []etcdMember `json:"members"`
}

type etcdMember struct {
	Name string `json:"name"`
}
