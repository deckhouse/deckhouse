// Copyright 2023 Flant JSC
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

package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	coordinationclientv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	coordination "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	label = "deckhouse.io/documentation-builder-sync"

	leaseDuration         = 35
	leaseRenewPeriod      = 30
	leaseCollectionPeriod = 90
)

func NewLeasesManager() (*LeasesManager, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("create rest config: %w", err)
	}

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("new client set: %w", err)
	}

	podName := os.Getenv("POD_NAME")
	splitPodName := strings.Split(podName, "-")
	if len(splitPodName) != 3 {
		return nil, fmt.Errorf("unexpected POD_NAME %q", podName)
	}

	return &LeasesManager{
		kclient:       kclient,
		podIP:         os.Getenv("POD_IP"),
		podNamespace:  os.Getenv("POD_NAMESPACE"),
		clusterDomain: os.Getenv("CLUSTER_DOMAIN"),
		name:          strings.Join([]string{"module-docs-builder", splitPodName[2]}, "-"),
	}, nil
}

type LeasesManager struct {
	podIP         string
	podNamespace  string
	clusterDomain string

	name    string
	kclient *kubernetes.Clientset
}

func (m *LeasesManager) create(ctx context.Context) error {
	l := m.newLease()
	l, err := m.leases().Create(ctx, l, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	return nil
}

func (m *LeasesManager) Run(ctx context.Context) error {
	if err := m.gc(ctx); err != nil {
		return fmt.Errorf("first gc: %w", err)
	}

	if err := m.create(ctx); err != nil {
		return fmt.Errorf("create leases: %w", err)
	}

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		err := m.renewLoop(ctx)
		if err != nil {
			return fmt.Errorf("renew loop: %w", err)
		}
		return nil
	})

	group.Go(func() error {
		err := m.garbageCollectionLoop(ctx)
		if err != nil {
			return fmt.Errorf("gc loop: %w", err)
		}
		return nil
	})

	return group.Wait()
}

func (m *LeasesManager) renewLoop(ctx context.Context) error {
	ticker := time.NewTicker(time.Second * leaseRenewPeriod)

	for {
		select {
		case <-ticker.C:
			err := m.renew(ctx)
			if err != nil {
				return fmt.Errorf("renew lease: %w", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (m *LeasesManager) renew(ctx context.Context) error {
	lease, err := m.leases().Get(ctx, m.name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get lease %q: %w", m.name, err)
	}

	now := metav1.NowMicro()
	lease.Spec.RenewTime = &now

	_, err = m.leases().Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update lease %q: %w", m.name, err)
	}

	return nil
}

func (m *LeasesManager) Remove(ctx context.Context) error {
	err := m.leases().Delete(ctx, m.name, metav1.DeleteOptions{})

	return err
}

func (m *LeasesManager) newLease() *coordination.Lease {
	address := fmt.Sprintf(
		"%s.%s.pod.%s:8081",
		strings.ReplaceAll(m.podIP, ".", "-"),
		m.podNamespace,
		m.clusterDomain,
	)

	now := metav1.NowMicro()

	return &coordination.Lease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Lease",
			APIVersion: "coordination.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   m.name,
			Labels: map[string]string{label: ""},
		},
		Spec: coordination.LeaseSpec{
			HolderIdentity:       pointer.String(address),
			RenewTime:            &now,
			LeaseDurationSeconds: pointer.Int32(leaseDuration),
		},
	}
}

func (m *LeasesManager) garbageCollectionLoop(ctx context.Context) error {
	ticker := time.NewTicker(time.Second * leaseCollectionPeriod)

	for {
		select {
		case <-ticker.C:
			err := m.gc(ctx)
			if err != nil {
				klog.Error("cleanup leases:", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (m *LeasesManager) gc(ctx context.Context) error {
	list, err := m.leases().List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	for _, lease := range list.Items {
		expireAt := lease.Spec.RenewTime.Add(time.Duration(pointer.Int32Deref(lease.Spec.LeaseDurationSeconds, 0)) * time.Second)

		if !expireAt.Before(time.Now()) {
			continue
		}

		err := m.leases().Delete(ctx, lease.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("remove: %w", err)
		}
	}

	return nil
}

func (m *LeasesManager) leases() coordinationclientv1.LeaseInterface {
	return m.kclient.CoordinationV1().Leases(m.podNamespace)
}
