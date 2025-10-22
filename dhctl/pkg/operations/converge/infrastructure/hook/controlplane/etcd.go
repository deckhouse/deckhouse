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
	"time"

	flantkubeclient "github.com/flant/kube-client/client"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func waitEtcdHasMember(ctx context.Context, client *flantkubeclient.Client, nodeName string) error {
	return retry.NewLoop(fmt.Sprintf("Check the master node '%s' is listed as etcd cluster member", nodeName), 100, 20*time.Second).RunContext(ctx, func() error {
		ok, err := isEtcdHasMember(ctx, client, nodeName, "")
		if err != nil {
			return fmt.Errorf("failed to check etcd cluster member: %s", err)
		}

		if !ok {
			return fmt.Errorf("node '%s' is not listed as etcd cluster member", nodeName)
		}

		return nil
	})
}

func waitEtcdHasNoMember(ctx context.Context, client *flantkubeclient.Client, nodeName string) error {
	return retry.NewLoop(fmt.Sprintf("Check the master node '%s' is no longer listed as etcd cluster member", nodeName), 45, 5*time.Second).RunContext(ctx, func() error {
		// exclude the node we are checking
		fieldSelector := fields.OneTermNotEqualSelector("spec.nodeName", nodeName).String()

		ok, err := isEtcdHasMember(ctx, client, nodeName, fieldSelector)
		if err != nil {
			return err
		}

		if ok {
			return fmt.Errorf("node '%s' is still listed as etcd cluster member", nodeName)
		}

		return nil
	})
}

func isEtcdHasMember(ctx context.Context, client *flantkubeclient.Client, nodeName string, fieldSelector string) (bool, error) {
	pods, err := client.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{
		LabelSelector: "component=etcd,tier=control-plane",
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return false, fmt.Errorf("failed to get etcd pods: %s", err)
	}

	if len(pods.Items) == 0 {
		return false, fmt.Errorf("etcd pods not found")
	}

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace("kube-system").
		Name(pods.Items[0].Name).
		SubResource("exec")

	command := []string{
		"etcdctl",
		"--cacert", "/etc/kubernetes/pki/etcd/ca.crt",
		"--cert", "/etc/kubernetes/pki/etcd/ca.crt",
		"--key", "/etc/kubernetes/pki/etcd/ca.key",
		"--endpoints", "https://127.0.0.1:2379/",
		"member", "list", "-w", "json",
	}

	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: "etcd",
		Stdin:     false,
		Stdout:    true,
		Stderr:    false,
		TTY:       false,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(client.RestConfig(), "POST", req.URL())
	if err != nil {
		return false, fmt.Errorf("failed to create `Executor`: %v", err)
	}

	var stdout bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: nil,
		Tty:    false,
	})
	if err != nil {
		return false, fmt.Errorf("failed to execute in `Stream`: %v", err)
	}

	var members memberListOutput

	err = json.Unmarshal(stdout.Bytes(), &members)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal etcd member list: %s", err)
	}

	for _, member := range members.Members {
		if member.Name == nodeName {
			return true, nil
		}
	}

	return false, nil
}

type memberListOutput struct {
	Members []struct {
		Name string `json:"name"`
	} `json:"members"`
}
